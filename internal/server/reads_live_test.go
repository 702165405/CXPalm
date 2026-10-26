package server

import (
	"context"
	"cxpalm/internal/config"
	"cxpalm/internal/desktop"
	"cxpalm/internal/store"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in: follow one real unread reply and confirm its official read transition.
// Device authorization and gateway receipt writes remain isolated in a temporary store.
func TestLiveSharedReadAcrossAuthorizedSockets(t *testing.T) {
	path := os.Getenv("CXPALM_READ_SYNC_CONFIG")
	if path == "" {
		t.Skip("opt-in real desktop, isolated shared-read store")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = t.TempDir()
	db, err := store.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(cfg, db)
	defer s.Bridge.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	rows, err := s.Bridge.History.List(ctx, "", false)
	if err != nil {
		t.Fatal(err)
	}
	var selected desktop.State
	for i, row := range rows {
		if i >= 40 {
			break
		}
		candidate, stop := context.WithTimeout(ctx, 1500*time.Millisecond)
		ch, remove, e := s.Bridge.Subscribe(candidate, row.ID)
		if e != nil {
			t.Logf("candidate %d follow error: %v", i, e)
			stop()
			continue
		}
	wait:
		for {
			select {
			case state := <-ch:
				if !state.Connected {
					continue
				}
				var raw map[string]any
				json.Unmarshal(state.Snapshot, &raw)

				var presented map[string]any
				json.Unmarshal(s.Bridge.PhoneState(state).Snapshot, &presented)

				if presented["hasUnreadTurn"] == true && desktop.PhoneState(state).ReadToken != "" {
					selected = state
				}
				break wait
			case <-candidate.Done():
				break wait
			}
		}
		remove()
		stop()
		if selected.ID != "" {
			break
		}
	}
	if selected.ID == "" {
		t.Fatal("a real desktop unread reply is required")
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	origin, _ := url.Parse(cfg.Origin)
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	dial := func(name string) *websocket.Conn {
		t.Helper()
		_, token, e := loginFixture(t, db, name)
		if e != nil {
			t.Fatal(e)
		}
		client := &http.Client{Transport: hostTransport{transport, origin.Host}}
		ws, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {cfg.Origin}, "Authorization": {"Bearer " + token}}})
		if e != nil {
			t.Fatal(e)
		}
		ws.SetReadLimit(16 << 20)
		return ws
	}
	wait := func(ws *websocket.Conn, predicate func(Event) bool) Event {
		t.Helper()
		for {
			_, raw, e := ws.Read(ctx)
			if e != nil {
				t.Fatal(e)
			}
			var event Event
			if e = json.Unmarshal(raw, &event); e != nil {
				t.Fatal(e)
			}
			if predicate(event) {
				return event
			}
		}
	}
	send := func(ws *websocket.Conn, q Command) {
		t.Helper()
		raw, _ := json.Marshal(q)
		if e := ws.Write(ctx, websocket.MessageText, raw); e != nil {
			t.Fatal(e)
		}
	}
	one, two := dial("web-isolated"), dial("android-isolated")
	defer one.CloseNow()
	defer two.CloseNow()
	for _, ws := range []*websocket.Conn{one, two} {
		wait(ws, func(e Event) bool { return e.Type == "hello" })
		send(ws, Command{ID: "follow", Method: "chat.subscribe", Thread: selected.ID})
		wait(ws, func(e Event) bool {
			data, _ := e.Data.(map[string]any)
			return e.Type == "chat.state" && data["connected"] == true
		})
	}
	token := desktop.PhoneState(selected).ReadToken
	started := time.Now()
	send(one, Command{ID: "read", Method: "chat.read", Thread: selected.ID, ReadToken: token})
	result := wait(one, func(e Event) bool { return e.ID == "read" })
	data, _ := result.Data.(map[string]any)
	if result.Type != "result" || data["accepted"] != true {
		t.Fatal("receipt not accepted", result.Type, result.Error)
	}
	wait(two, func(e Event) bool {
		data, _ := e.Data.(map[string]any)
		snapshot, _ := data["snapshot"].(map[string]any)
		sequence, _ := data["sequence"].(float64)
		return e.Type == "chat.state" && data["readToken"] == token && snapshot["hasUnreadTurn"] == false && sequence > float64(selected.Sequence)
	})
	send(two, Command{ID: "list", Method: "sessions.browse", Limit: 100})
	result = wait(two, func(e Event) bool { return e.ID == "list" })
	raw, _ := json.Marshal(result.Data)
	var page desktop.SessionPage
	json.Unmarshal(raw, &page)
	found := false
	for _, row := range page.Items {
		if row.ID == selected.ID {
			found = row.Unread != nil && !*row.Unread && row.ReadToken == token
		}
	}
	if !found {
		t.Fatal("second device list did not match shared read")
	}
	persisted, err := db.ReadToken(selected.ID)
	if err != nil || persisted != token {
		t.Fatal("receipt not persisted", err)
	}
	evidence := map[string]any{"distinctAuthorizedDevices": 2, "sharedChatState": true, "sharedListState": true, "receiptPersisted": true, "realDesktopFollow": true, "officialReadWrite": true, "initiallyUnread": true, "officialConfirmationMs": time.Since(started).Milliseconds(), "liveConfigurationChanged": false}
	if path := os.Getenv("CXPALM_READ_SYNC_RESULT"); path != "" {
		raw, _ := json.MarshalIndent(evidence, "", "  ")
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("shared read evidence: %v", evidence)
}
