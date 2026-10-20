package server

import (
	"context"
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	db, e := store.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	c := config.Config{Origin: "https://cx.example", Listen: "127.0.0.1:18789", DataDir: root, CodexHome: root}
	s := New(c, db)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Bridge.Close(); db.Close() })
	_, token, e := loginFixture(t, db, "phone")
	if e != nil {
		t.Fatal(e)
	}
	return s, ts, token
}
func loginFixture(t *testing.T, db *store.Store, name string) (store.Device, string, error) {
	t.Helper()
	status, err := db.LoginSettings()
	if err != nil {
		return store.Device{}, "", err
	}
	if !status.Configured {
		if _, err = db.SetLogin(store.LoginInput{Username: "fixture", Password: "fixture-test-password-012345"}); err != nil {
			return store.Device{}, "", err
		}
	}
	return db.Login("fixture", "fixture-test-password-012345", name, "", true)
}
func TestLegacyAuthenticationRoutesDoNotExist(t *testing.T) {
	_, ts, _ := fixture(t)
	for _, path := range []string{"/api/v1/pair", "/api/v1/browser/pair", "/pair", "/revoke", "/status"} {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(`{"secret":"`+strings.Repeat("a", 64)+`"}`))
		req.Host = "cx.example"
		req.Header.Set("Origin", "https://cx.example")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 404 {
			t.Fatalf("legacy endpoint %s returned %d", path, response.StatusCode)
		}
	}
}
func TestWebSocketAuthenticationAndAllowlist(t *testing.T) {
	s, ts, token := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: roundTripper{transport}}
	headers := http.Header{"Origin": []string{"https://cx.example"}, "Authorization": []string{"Bearer " + token}}
	c, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: headers})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	_, _, e = c.Read(ctx)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(Command{ID: "one", Method: "arbitrary.rpc"})
	if e = c.Write(ctx, websocket.MessageText, b); e != nil {
		t.Fatal(e)
	}
	_, b, e = c.Read(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var v Event
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	if v.Type != "error" {
		t.Fatal(string(b))
	}
	devices, e := s.Store.Devices()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Store.Revoke(devices[0].ID); e != nil {
		t.Fatal(e)
	}
	if s.Store.Valid(devices[0].ID) {
		t.Fatal("revoked device still valid")
	}
}

type roundTripper struct{ base http.RoundTripper }

func TestRemovedWorkspaceMethodsFailClosed(t *testing.T) {
	s, ts, token := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: roundTripper{transport}}
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{
		HTTPClient: client, HTTPHeader: http.Header{"Origin": {"https://cx.example"}, "Authorization": {"Bearer " + token}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	_, raw, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var hello struct {
		Type string
		Data struct {
			Device       string
			Capabilities []string
		}
	}
	if err = json.Unmarshal(raw, &hello); err != nil || hello.Type != "hello" {
		t.Fatal("missing hello")
	}
	retained := map[string]bool{"chat.settings": false, "chat.catalog": false, "chat.send": false}
	for _, capability := range hello.Data.Capabilities {
		if strings.HasPrefix(capability, "files.") || strings.HasPrefix(capability, "terminal.") || strings.HasPrefix(capability, "git.") || strings.HasPrefix(capability, "side.") {
			t.Fatal("removed capability advertised")
		}
		if _, ok := retained[capability]; ok {
			retained[capability] = true
		}
	}
	for method, found := range retained {
		if !found {
			t.Fatal("retained method missing", method)
		}
	}
	for i, method := range []string{"files.list", "files.read", "terminal.open", "terminal.read", "terminal.write", "terminal.resize", "terminal.close", "terminal.future", "files.future", "projects.list", "git.status", "git.diff", "git.log", "git.stage", "git.unstage", "git.commit", "side.list", "side.create", "side.read", "side.send", "side.stop", "side.approve", "side.catalog", "git.future", "side.future"} {
		id := fmt.Sprintf("removed-%d", i)
		// Include legacy fields: an old or custom client must not restore the removed routes.
		command, _ := json.Marshal(map[string]any{"id": id, "method": method, "project": "p", "path": ".", "terminal": "old", "cols": 80, "rows": 24, "text": "echo not-executed"})
		if err = c.Write(ctx, websocket.MessageText, command); err != nil {
			t.Fatal(err)
		}
		_, raw, err = c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event Event
		if err = json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		if event.ID != id || event.Type != "error" || event.Error != "method is not implemented or not allowlisted" {
			t.Fatalf("removed method was not rejected: %s", method)
		}
		if _, err = s.Store.Receipt(hello.Data.Device, id); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("removed method entered the mutation ledger", method)
		}
	}
	s.Store.Begin(hello.Data.Device, "fixture-read", "fixture")
	s.Store.Finish(hello.Data.Device, "fixture-read", "confirmed", nil)
	for i, method := range []string{"operation.get"} {
		id := fmt.Sprintf("retained-%d", i)
		command, _ := json.Marshal(Command{ID: id, Method: method, RequestID: "fixture-read"})
		if err = c.Write(ctx, websocket.MessageText, command); err != nil {
			t.Fatal(err)
		}
		_, raw, err = c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event Event
		if err = json.Unmarshal(raw, &event); err != nil || event.ID != id || event.Type != "result" {
			t.Fatal("retained read failed", method)
		}
	}
}

func (r roundTripper) RoundTrip(q *http.Request) (*http.Response, error) {
	q = q.Clone(q.Context())
	q.Host = "cx.example"
	return r.base.RoundTrip(q)
}
