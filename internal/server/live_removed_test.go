package server

import (
	"context"
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestDeployedFileAndTerminalRemoval(t *testing.T) {
	path := os.Getenv("CXPALM_REMOVAL_LIVE_CONFIG")
	if path == "" {
		t.Skip("opt-in deployed permission-removal verification")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("configuration unavailable")
	}
	db, err := store.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	username, password := os.Getenv("CXPALM_LIVE_USERNAME"), os.Getenv("CXPALM_LIVE_PASSWORD")
	if username == "" || password == "" {
		t.Skip("explicit account credentials required")
	}
	device, token, err := db.Login(username, password, "permission-removal-verification", "", false)
	if err != nil {
		t.Fatal("temporary authorization unavailable")
	}
	defer func() {
		if err := db.Revoke(device.ID); err != nil {
			t.Error("temporary device revocation failed")
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	socket, _, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(cfg.Origin, "https")+"/api/v1/ws", &websocket.DialOptions{
		HTTPClient: client, HTTPHeader: http.Header{"Origin": {cfg.Origin}, "Authorization": {"Bearer " + token}},
	})
	if err != nil {
		t.Fatal("public WSS unavailable")
	}
	defer socket.CloseNow()
	socket.SetReadLimit(2 << 20)
	_, raw, err := socket.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var hello struct {
		Type string
		Data struct {
			Version      string
			Capabilities []string
		}
	}
	if err = json.Unmarshal(raw, &hello); err != nil || hello.Type != "hello" || hello.Data.Version != config.Version {
		t.Fatal("running Agent version mismatch")
	}
	for _, method := range hello.Data.Capabilities {
		if strings.HasPrefix(method, "files.") || strings.HasPrefix(method, "terminal.") || strings.HasPrefix(method, "git.") || strings.HasPrefix(method, "side.") {
			t.Fatal("removed capability remains deployed")
		}
	}
	for i, method := range []string{"files.list", "files.read", "terminal.open", "terminal.read", "terminal.write", "terminal.resize", "terminal.close", "projects.list", "git.status", "git.diff", "git.log", "git.stage", "git.unstage", "git.commit", "side.list", "side.create", "side.read", "side.send", "side.stop", "side.approve", "side.catalog"} {
		id := fmt.Sprintf("permission-removal-%d", i)
		request, _ := json.Marshal(Command{ID: id, Method: method})
		if err = socket.Write(ctx, websocket.MessageText, request); err != nil {
			t.Fatal(err)
		}
		_, raw, err = socket.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var result Event
		if err = json.Unmarshal(raw, &result); err != nil || result.ID != id || result.Type != "error" || result.Error != "method is not implemented or not allowlisted" {
			t.Fatal("deployed route not rejected", method)
		}
	}
	t.Log("deployed Agent rejects all removed workspace routes for an authorized temporary device; no file read, shell or model turn; temporary device revoked")
}
