package server

import (
	"context"
	"crypto/sha256"
	"cxpalm/internal/config"
	"cxpalm/internal/desktop"
	"cxpalm/internal/store"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func notificationState(sequence uint64, runtime string, requests string) desktop.State {
	return desktop.State{ID: "unselected-chat", Connected: true, Sequence: sequence, Snapshot: json.RawMessage(`{"threadRuntimeStatus":{"type":"` + runtime + `"},"turns":[{"turnId":"turn-a"}],"requests":` + requests + `}`)}
}
func TestNotificationTransitionsDoNotDependOnSelectedChat(t *testing.T) {
	tracker := notificationTracker{}
	if got := tracker.observe(notificationState(1, "idle", "[]"), "Other chat"); len(got) != 0 {
		t.Fatal("initial snapshot must not replay history")
	}
	tracker.observe(notificationState(2, "active", "[]"), "Other chat")
	stale := tracker.observe(notificationState(1, "idle", "[]"), "Other chat")
	if len(stale) != 0 {
		t.Fatal("stale state generated alert")
	}
	offline := notificationState(3, "idle", "[]")
	offline.Connected = false
	if len(tracker.observe(offline, "Other chat")) != 0 {
		t.Fatal("disconnect reported completion")
	}
	events := tracker.observe(notificationState(4, "idle", `[{"id":"request-1"}]`), "Other chat")
	if len(events) != 2 || events[0].Kind != "complete" || events[1].Kind != "approval" {
		t.Fatalf("missing independent events: %+v", events)
	}
	if len(events[0].ID) != 64 || events[0].Thread != "unselected-chat" || events[0].Title != "Other chat" {
		t.Fatal(events)
	}
	if len(tracker.observe(notificationState(5, "idle", `[{"id":"request-1"}]`), "Other chat")) != 0 {
		t.Fatal("duplicate notification")
	}
}
func TestNotificationsFanOutToAllReceiversAndReplayOnce(t *testing.T) {
	feed := notificationFeed{listeners: map[uint64]func(Event) bool{}}
	// Don't start discovery against a live desktop in this pure fanout test.
	feed.once.Do(func() {})
	first, second := []Event{}, []Event{}
	stopFirst := feed.subscribe(nil, "", "a", func(e Event) bool { first = append(first, e); return true })
	stopSecond := feed.subscribe(nil, "", "b", func(e Event) bool { second = append(second, e); return true })
	defer stopSecond()
	one := Notification{ID: "one", Kind: "complete", Thread: "chat-a"}
	two := Notification{ID: "two", Kind: "approval", Thread: "chat-b"}
	feed.publish(one)
	feed.publish(one)
	feed.publish(two)
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("fanout or dedup failed %d %d", len(first), len(second))
	}
	stopFirst()
	feed.publish(Notification{ID: "three", Kind: "complete"})
	if len(first) != 3 || len(second) != 4 {
		t.Fatal("disconnect affected another receiver")
	}
	var replay Event
	stop := feed.subscribe(nil, "one", "r", func(e Event) bool { replay = e; return true })
	defer stop()
	data := replay.Data.(map[string]any)
	if data["gap"] != false || len(data["events"].([]Notification)) != 2 {
		t.Fatal(data)
	}
	stopGap := feed.subscribe(nil, "unknown", "g", func(e Event) bool {
		if e.Data.(map[string]any)["resynced"] != true || e.Data.(map[string]any)["gap"] != false || e.Data.(map[string]any)["cursor"] != "three" {
			t.Error("unknown cursor did not establish a current checkpoint")
		}
		return true
	})
	defer stopGap()
}

func TestAuthenticatedWebAndAppBothReceiveNotifications(t *testing.T) {
	s, ts, firstToken := fixture(t)
	s.notifications.once.Do(func() {}) // Isolate transport from the official desktop.
	scope, err := s.Bridge.NotificationScope()
	if err != nil {
		t.Fatal(err)
	}
	pref, err := s.Store.SetNotificationPreference(scope, "not-selected", true)
	if err != nil {
		t.Fatal(err)
	}
	_, secondToken, err := loginFixture(t, s.Store, "second-receiver")
	if err != nil {
		t.Fatal(err)
	}
	ctx, done := context.WithTimeout(context.Background(), 6*time.Second)
	defer done()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: roundTripper{transport}}
	dial := func(token string) *websocket.Conn {
		c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {"https://cx.example"}, "Authorization": {"Bearer " + token}}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.CloseNow() })
		if _, _, err = c.Read(ctx); err != nil {
			t.Fatal(err)
		}
		if err = c.Write(ctx, websocket.MessageText, []byte(`{"id":"watch","method":"notifications.subscribe"}`)); err != nil {
			t.Fatal(err)
		}
		_, raw, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var reply Event
		if json.Unmarshal(raw, &reply) != nil || reply.Type != "result" {
			t.Fatalf("subscribe failed: %s", raw)
		}
		return c
	}
	web, app := dial(firstToken), dial(secondToken)
	event := Notification{ID: strings.Repeat("a", 64), Kind: "approval", Thread: "not-selected", Title: "Other chat", Subscription: pref.Generation}
	s.notifications.publish(event)
	for _, receiver := range []*websocket.Conn{web, app} {
		_, raw, err := receiver.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Type string       `json:"type"`
			Data Notification `json:"data"`
		}
		if json.Unmarshal(raw, &got) != nil || got.Type != "notification" || got.Data.ID != event.ID || got.Data.Thread != "not-selected" {
			t.Fatalf("wrong notification: %s", raw)
		}
	}
	devices, err := s.Store.Devices()
	if err != nil {
		t.Fatal(err)
	}
	for _, device := range devices {
		if device.Name == "phone" {
			if err = s.Store.Revoke(device.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	event.ID = strings.Repeat("b", 64)
	s.notifications.publish(event)
	_, raw, err := app.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), event.ID) {
		t.Fatalf("revocation stopped the other receiver: %s", raw)
	}
	if _, _, err = web.Read(ctx); err == nil {
		t.Fatal("revoked receiver received another event")
	}
}

func notificationAccountFixture(t *testing.T, root, account string) {
	t.Helper()
	claims := `{"https://api.openai.com/auth":{"chatgpt_account_id":"` + account + `","user_id":"test-user"}}`
	auth, _ := json.Marshal(map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{"access_token": "header." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".signature"}})
	parts, _ := json.Marshal([]string{"chatgpt", account, "test-user"})
	key := sha256.Sum256(parts)
	global, _ := json.Marshal(map[string]any{"electron-thread-read-state-v1": map[string]any{"version": 1, "unreadByIdentity": map[string]any{hex.EncodeToString(key[:]): map[string]any{"local:test-host": []string{}}}}})
	for name, raw := range map[string][]byte{"auth.json": auth, ".codex-global-state.json": global} {
		if err := os.WriteFile(filepath.Join(root, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestNotificationReplayAndCheckpointRecoverAfterGatewayRestart(t *testing.T) {
	root := t.TempDir()
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{CodexHome: root, DataDir: dir}
	s := New(cfg, db)
	s.notifications.once.Do(func() {})
	if err := s.notifications.syncJournal(s); err != nil {
		t.Fatal(err)
	}
	tracker := notificationTracker{}
	tracker.observe(notificationState(900, "active", "[]"), "Chat")
	pref, err := db.SetNotificationPreference(s.notifications.scope, "chat", true)
	if err != nil {
		t.Fatal(err)
	}
	trackedPref, err := db.SetNotificationPreference(s.notifications.scope, "unselected-chat", true)
	if err != nil {
		t.Fatal(err)
	}
	s.notifications.saveCheckpoint("unselected-chat", s.notifications.scope, tracker)
	s.notifications.publish(Notification{ID: "before-restart", Kind: "complete", Thread: "chat", Subscription: pref.Generation})
	s.Bridge.Close()
	db.Close()
	db, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	next := New(cfg, db)
	defer next.Bridge.Close()
	next.notifications.once.Do(func() {})
	if err := next.notifications.syncJournal(next); err != nil {
		t.Fatal(err)
	}
	recovered, scope := next.notifications.checkpoint("unselected-chat")
	events := recovered.observe(notificationState(1, "idle", "[]"), "Chat")
	if len(events) != 1 || events[0].Kind != "complete" {
		t.Fatal("missed completion after restart", events)
	}
	events[0].Subscription = trackedPref.Generation
	if !next.notifications.publishScoped(scope, events[0]) {
		t.Fatal("journal publication failed")
	}
	next.notifications.saveCheckpoint("unselected-chat", scope, recovered)
	var replay map[string]any
	stop := next.notifications.subscribe(nil, "before-restart", "replay", func(e Event) bool { replay = e.Data.(map[string]any); return true })
	stop()
	if replay["gap"] != false || replay["resynced"] != false || len(replay["events"].([]Notification)) != 1 {
		t.Fatal("restart lost cursor", replay)
	}
	stop = next.notifications.subscribe(nil, "old-evicted-position", "recover", func(e Event) bool { replay = e.Data.(map[string]any); return true })
	stop()
	if replay["resynced"] != true || replay["gap"] != false || replay["cursor"] != events[0].ID {
		t.Fatal("current checkpoint missing", replay)
	}
	stop = next.notifications.subscribe(nil, replay["cursor"].(string), "again", func(e Event) bool { replay = e.Data.(map[string]any); return true })
	stop()
	if replay["resynced"] != false || len(replay["events"].([]Notification)) != 0 {
		t.Fatal("repeated stale-cursor recovery", replay)
	}
	notificationAccountFixture(t, root, "another-longer-account")
	if err := next.notifications.syncJournal(next); err != nil {
		t.Fatal(err)
	}
	if len(next.notifications.recent) != 2 || next.notifications.scope != scope {
		t.Fatal("account switch reset the desktop journal")
	}
	if !next.notifications.publishScoped(scope, Notification{ID: "after-account-switch", Thread: "chat", Subscription: pref.Generation}) {
		t.Fatal("account switch interrupted the desktop watcher")
	}
}
func TestNotificationJournalFailureIsNotReportedAsHealthy(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	feed := notificationFeed{journal: db, scope: "account", checkpoints: map[string]json.RawMessage{}}
	if feed.publishScoped("account", Notification{ID: "event"}) {
		t.Fatal("closed journal claimed durable delivery")
	}
	feed.status("")
	if feed.lastError == "" {
		t.Fatal("refresh hid a real persistence failure")
	}
}
