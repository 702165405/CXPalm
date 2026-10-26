package server

import (
	"bytes"
	"context"
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestDeployedBrowserCookieReadOnly(t *testing.T) {
	path := os.Getenv("CXPALM_BROWSER_LIVE_CONFIG")
	if path == "" {
		t.Skip("opt-in deployed browser authentication and read-only desktop verification")
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
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	post := func(path string, value any) *http.Response {
		t.Helper()
		body, _ := json.Marshal(value)
		req, _ := http.NewRequest("POST", cfg.Origin+path, bytes.NewReader(body))
		req.Header.Set("Origin", cfg.Origin)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal("public browser request unavailable")
		}
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	response := post("/api/v1/browser/login", map[string]any{"username": username, "password": password, "remember": false})
	var login struct {
		Device  store.Device `json:"device"`
		Token   string       `json:"token"`
		Version string       `json:"version"`
	}
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&login) != nil || login.Device.ID == "" || login.Token != "" || login.Version != config.Version {
		t.Fatal("browser login did not issue a private cookie session")
	}
	defer func() {
		if err := db.Revoke(login.Device.ID); err != nil {
			t.Error("temporary device revocation failed")
		}
	}()
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Domain != "" || cookies[0].Path != "/api/v1/" {
		t.Fatal("public cookie flags unsafe")
	}
	if post("/api/v1/browser/session", map[string]string{}).StatusCode != 200 {
		t.Fatal("cookie session unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(cfg.Origin, "https")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {cfg.Origin}}})
	if err != nil {
		t.Fatal("public cookie WSS unavailable")
	}
	defer c.CloseNow()
	c.SetReadLimit(2 << 20)
	if _, _, err = c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	call := func(id, method, thread string) Event {
		t.Helper()
		raw, _ := json.Marshal(Command{ID: id, Method: method, Thread: thread})
		if err := c.Write(ctx, websocket.MessageText, raw); err != nil {
			t.Fatal(err)
		}
		for {
			_, raw, err := c.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var result Event
			if json.Unmarshal(raw, &result) != nil {
				t.Fatal("bad event")
			}
			if result.ID == id {
				return result
			}
		}
	}
	for _, method := range []string{"files.list", "files.read", "terminal.open", "terminal.read", "terminal.write", "terminal.resize", "terminal.close", "projects.list", "git.status", "git.diff", "git.log", "git.stage", "git.unstage", "git.commit", "side.list", "side.create", "side.read", "side.send", "side.stop", "side.approve", "side.catalog"} {
		if result := call(method, method, ""); result.Type != "error" || result.Error != "method is not implemented or not allowlisted" {
			t.Fatal("browser restored a removed method")
		}
	}
	result := call("browser-session-list", "sessions.list", "")
	if result.Type != "result" {
		t.Fatal("browser cannot list desktop sessions")
	}
	raw, _ := json.Marshal(result.Data)
	var sessions []struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &sessions) != nil || len(sessions) == 0 {
		t.Fatal("no allowed desktop session")
	}
	catalog := call("browser-catalog", "chat.catalog", sessions[0].ID)
	if catalog.Type != "result" {
		t.Fatal("real conversation catalog unavailable")
	}
	encoded, _ := json.Marshal(catalog.Data)
	var metadata struct {
		Models []any `json:"models"`
		Skills []any `json:"skills"`
	}
	if json.Unmarshal(encoded, &metadata) != nil || len(metadata.Models) == 0 || len(metadata.Skills) == 0 {
		t.Fatal("real model/skill catalog is empty")
	}
	t.Logf("read-only desktop metadata: %d models, %d skills", len(metadata.Models), len(metadata.Skills))
	if call("browser-follow", "chat.subscribe", sessions[0].ID).Type != "result" {
		t.Fatal("browser cannot follow original desktop owner")
	}
	for {
		_, raw, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event struct {
			Type string `json:"type"`
			Data struct {
				Connected bool            `json:"connected"`
				Snapshot  json.RawMessage `json:"snapshot"`
				Error     string          `json:"error"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &event) == nil && event.Type == "chat.state" && event.Data.Connected && len(event.Data.Snapshot) > 0 {
			break
		}
		if event.Type == "chat.state" {
			t.Logf("browser follower state: connected=%v snapshotBytes=%d error=%s", event.Data.Connected, len(event.Data.Snapshot), event.Data.Error)
		}
	}
	if post("/api/v1/browser/logout", map[string]string{}).StatusCode != 200 || db.Valid(login.Device.ID) {
		t.Fatal("public logout failed to revoke browser")
	}
	if post("/api/v1/browser/session", map[string]string{}).StatusCode != 401 {
		t.Fatal("revoked browser still authorized")
	}
	t.Log("real HTTPS HttpOnly-cookie login, WSS, desktop session listing and follower snapshot passed; removed routes rejected; logout revoked temporary device; no model mutation")
}
