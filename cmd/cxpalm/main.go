package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cxpalm/internal/agent"
	"cxpalm/internal/config"
	"cxpalm/internal/control"
	"cxpalm/internal/desktop"
	"cxpalm/internal/store"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "CXPalm:", e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		fmt.Println("CXPalm " + config.Version + "\nCommands: init, serve, devices, revoke, doctor, desktop-host, version")
		return nil
	}
	action := os.Args[1]
	if action == "validate-config" {
		// Validate local desktop input without opening a database, listener or tunnel.
		input, err := io.ReadAll(io.LimitReader(os.Stdin, (64<<10)+1))
		if err != nil {
			return err
		}
		if len(input) > 64<<10 {
			return errors.New("configuration exceeds size limit")
		}
		var c config.Config
		if err = json.Unmarshal(input, &c); err != nil {
			return errors.New("invalid configuration JSON")
		}
		return c.Validate()
	}
	if action == "version" {
		fmt.Println(config.Version)
		return nil
	}
	fs := flag.NewFlagSet(action, flag.ContinueOnError)
	path := fs.String("config", "cxpalm.json", "private configuration path")
	origin := fs.String("origin", "", "fixed HTTPS origin")
	id := fs.String("id", "", "device ID")
	home := fs.String("codex-home", "", "official Codex home directory")
	bin := fs.String("codex-bin", "", "legacy compatibility option (no executor is started)")
	follow := fs.Bool("follow", false, "doctor: passively subscribe to the most recent allowed desktop chat")
	if e := fs.Parse(os.Args[2:]); e != nil {
		return e
	}
	if action == "init" {
		absolute, e := filepath.Abs(*path)
		if e != nil {
			return e
		}
		user, e := os.UserHomeDir()
		if e != nil {
			return e
		}
		codexHome := *home
		if codexHome == "" {
			codexHome = os.Getenv("CODEX_HOME")
		}
		if codexHome == "" {
			codexHome = filepath.Join(user, ".codex")
		}
		c := config.Config{Listen: "127.0.0.1:18789", Origin: *origin, DataDir: filepath.Join(filepath.Dir(absolute), "data"), CodexHome: codexHome, CodexBin: *bin}
		if e = c.Validate(); e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(absolute), 0700); e != nil {
			return e
		}
		b, e := json.MarshalIndent(c, "", "  ")
		if e != nil {
			return e
		}
		f, e := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, e = f.Write(b)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		return closeErr
	}
	c, e := config.Load(*path)
	if e != nil {
		return e
	}
	db, e := store.Open(c.DataDir)
	if e != nil {
		return e
	}
	defer db.Close()
	switch action {
	case "desktop-host":
		return control.Serve(context.Background(), os.Stdin, os.Stdout, control.New(c, db))
	case "devices":
		v, e := db.Devices()
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(v)
	case "revoke":
		return db.Revoke(*id)
	case "doctor":
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		history := desktop.History{Config: c}
		sessions, historyErr := history.List(ctx, "", false)
		ipc, ipcErr := desktop.Connect(ctx, c.IPCPath)
		if ipc != nil {
			ipc.Close()
		}
		v := map[string]any{"version": config.Version, "sessions": len(sessions), "desktopConnected": ipcErr == nil}
		if historyErr != nil {
			v["historyError"] = historyErr.Error()
		}
		if ipcErr != nil {
			v["ipcError"] = ipcErr.Error()
		}
		if *follow && len(sessions) > 0 {
			bridge := desktop.NewBridge(c)
			defer bridge.Close()
			states, remove, e := bridge.Subscribe(ctx, sessions[0].ID)
			if e != nil {
				v["subscriptionError"] = e.Error()
			} else {
				defer remove()
				waiting := true
				for waiting {
					select {
					case state := <-states:
						if state.Connected {
							v["snapshotConnected"] = true
							v["snapshotBytes"] = len(state.Snapshot)
							waiting = false
						} else if state.Error != "" {
							v["subscriptionError"] = state.Error
							waiting = false
						}
					case <-ctx.Done():
						v["subscriptionError"] = "No owner snapshot arrived during passive subscription"
						waiting = false
					}
				}
			}
		}
		return json.NewEncoder(os.Stdout).Encode(v)
	case "serve":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		running, err := agent.Start(ctx, c, db)
		if err != nil {
			return err
		}
		defer running.Stop()
		fmt.Printf("CXPalm %s listening locally on %s; public origin %s\n", config.Version, c.Listen, c.Origin)
		select {
		case <-ctx.Done():
			return nil
		case <-running.Done:
			return running.Err()
		}

	default:
		return errors.New("unknown command")
	}
}
