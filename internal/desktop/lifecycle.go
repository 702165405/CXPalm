package desktop

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"cxpalm/internal/config"
)

func officialCLI(configured string) (string, error) {
	legacyNPM := strings.Contains(strings.ToLower(filepath.ToSlash(configured)), "/@openai/codex/") || strings.Contains(strings.ToLower(filepath.ToSlash(configured)), "/@openai/codex-win32-")
	if configured != "" && configured != "codex" && !legacyNPM {
		return configured, nil
	}
	candidates := []string{}
	if runtime.GOOS == "windows" {
		candidates, _ = filepath.Glob(filepath.Join(os.Getenv("LOCALAPPDATA"), "OpenAI", "Codex", "bin", "*", "codex.exe"))
	} else if runtime.GOOS == "darwin" {
		candidates = []string{"/Applications/Codex.app/Contents/Resources/codex", "/Applications/ChatGPT.app/Contents/Resources/codex"}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, e := os.Stat(candidates[i])
		b, f := os.Stat(candidates[j])
		return e == nil && (f != nil || a.ModTime().After(b.ModTime()))
	})
	for _, candidate := range candidates {
		if info, e := os.Stat(candidate); e == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	if configured != "" && configured != "codex" {
		return configured, nil
	}
	command := "codex"
	if runtime.GOOS == "windows" {
		command = "codex.exe"
	}
	if path, e := exec.LookPath(command); e == nil {
		return path, nil
	}
	return "", errors.New("official Codex CLI not found; install or update official Codex Desktop")
}

var threadID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// This short-lived official client changes only empty-thread/archive/goal metadata.
// It cannot resume threads, start turns, run tools, or own an execution.
func metadataCall(ctx context.Context, c config.Config, method string, params any) (json.RawMessage, error) {
	switch method {
	case "thread/start", "thread/read", "thread/name/set", "thread/archive", "thread/unarchive", "thread/goal/get", "thread/goal/set", "thread/goal/clear":
	default:
		return nil, errors.New("metadata method not allowlisted")
	}
	callCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	ctx = callCtx
	if method == "thread/start" {
		supplied, ok := params.(map[string]any)
		if !ok {
			return nil, errors.New("invalid thread start parameters")
		}
		copy := map[string]any{}
		for k, v := range supplied {
			copy[k] = v
		}
		copy["historyMode"] = "paginated"
		params = copy
	}
	bin, err := officialCLI(c.CodexBin)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, "app-server", "--listen", "stdio://")
	quietProcess(cmd)
	cmd.Env = append(os.Environ(), "CODEX_HOME="+c.CodexHome)
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		return nil, e
	}
	cmd.Stderr = io.Discard
	if e = cmd.Start(); e != nil {
		return nil, e
	}
	defer func() {
		in.Close()
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(1500 * time.Millisecond):
			_ = cmd.Process.Kill()
			<-done
		}
	}()
	enc := json.NewEncoder(in)
	scan := bufio.NewScanner(out)
	scan.Buffer(make([]byte, 4096), 16<<20)
	call := func(id int, method string, params any) (json.RawMessage, error) {
		if e := enc.Encode(map[string]any{"id": id, "method": method, "params": params}); e != nil {
			return nil, e
		}
		for scan.Scan() {
			var f struct {
				ID     *int            `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(scan.Bytes(), &f) != nil {
				continue
			}
			if f.ID == nil || *f.ID != id {
				continue
			}
			if f.Error != nil {
				return nil, fmt.Errorf("official metadata API: %s", f.Error.Message)
			}
			if len(f.Result) == 0 {
				return nil, errors.New("missing official API result")
			}
			return f.Result, nil
		}
		if e := scan.Err(); e != nil {
			return nil, e
		}
		return nil, errors.New("metadata API disconnected; outcome must be inspected")
	}
	if _, e = call(1, "initialize", map[string]any{"clientInfo": map[string]any{"name": "cxpalm", "version": config.Version}, "capabilities": map[string]any{"experimentalApi": strings.HasPrefix(method, "thread/goal/") || method == "thread/start"}}); e != nil {
		return nil, e
	}
	if e = enc.Encode(map[string]any{"method": "initialized"}); e != nil {
		return nil, e
	}
	result, err := call(2, method, params)
	if err != nil {
		return nil, err
	}
	if method == "thread/start" {
		var data struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if json.Unmarshal(result, &data) != nil || !threadID.MatchString(data.Thread.ID) {
			return nil, errors.New("invalid new thread result")
		}
		// An empty thread is lazy-persisted. Naming it commits metadata without a turn.
		if _, err = call(3, "thread/name/set", map[string]any{"threadId": data.Thread.ID, "name": "New chat"}); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (b *Bridge) Lifecycle(ctx context.Context, method, id, project string) (any, error) {
	if method == "sessions.create" {
		cwd, e := b.newChatDirectory(ctx, project)
		if e != nil {
			return nil, e
		}
		result, e := metadataCall(ctx, b.config, "thread/start", map[string]any{"cwd": cwd, "ephemeral": false})
		if e != nil {
			return nil, e
		}
		var data struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if json.Unmarshal(result, &data) != nil || !threadID.MatchString(data.Thread.ID) {
			return nil, errors.New("cannot verify newly created chat; inspect desktop before retrying")
		}
		// Confirm persistence from a different metadata process, not only a lazy in-memory thread.
		if _, err := metadataCall(ctx, b.config, "thread/read", map[string]any{"threadId": data.Thread.ID, "includeTurns": true}); err != nil {
			return map[string]any{"id": data.Thread.ID, "desktopReady": false, "opening": false, "message": "会话已创建，但历史持久化未确认，请核对桌面，不要重复创建。"}, nil
		}
		b.openCreatedChat(data.Thread.ID)
		return map[string]any{"id": data.Thread.ID, "desktopReady": false, "opening": true}, nil
	}
	if method != "sessions.archive" && method != "sessions.unarchive" {
		return nil, errors.New("session operation not allowlisted")
	}
	if !threadID.MatchString(id) {
		return nil, errors.New("invalid thread ID")
	}
	b.mu.Lock()
	lock := b.actions[id]
	if lock == nil {
		lock = &sync.Mutex{}
		b.actions[id] = lock
	}
	b.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()
	row, e := b.History.Get(ctx, id)
	if e != nil {
		return nil, e
	}
	archive := method == "sessions.archive"
	if row.Archived == archive {
		return map[string]any{"id": id, "archived": archive}, nil
	}
	// Unknown/disconnected is not idle. Never archive an active desktop task.
	if archive {
		snapshot, e := b.nativeSnapshot(id)
		if e != nil {
			return nil, e
		}
		state := Session{Status: "unknown"}
		enrichSession(&state, snapshot)
		if state.Status == "unknown" || state.Status == "running" || state.Status == "waiting" {
			return nil, errors.New("chat must be connected and idle before archiving")
		}
	}
	api := "thread/archive"
	if !archive {
		api = "thread/unarchive"
	}
	if _, e = metadataCall(ctx, b.config, api, map[string]any{"threadId": id}); e != nil {
		return nil, e
	}
	// Confirmation is the official persisted result, not a synthetic UI toggle.
	fresh, e := b.History.Get(ctx, id)
	if e != nil || fresh.Archived != archive {
		return nil, errors.New("archive result not confirmed; inspect official desktop")
	}
	if archive {
		b.mu.Lock()
		if s := b.sessions[id]; s != nil {
			s.Archived = true
			s.Connected = false
			s.Owner = ""
			s.Error = "chat archived"
			b.emitLocked(s)
		}
		b.mu.Unlock()
	}
	if c, e := b.connect(ctx); e == nil {
		params := map[string]any{"hostId": "local", "conversationId": id}
		raw, _ := json.Marshal(params)
		name, version := "thread-unarchived", 1
		if archive {
			name, version = "thread-archived", 2
			params["cwd"] = row.CWD
			raw, _ = json.Marshal(params)
		}
		_ = c.send(ctx, Frame{Type: "broadcast", Method: name, Version: version, Params: raw})
	}
	return map[string]any{"id": id, "archived": archive}, nil
}
