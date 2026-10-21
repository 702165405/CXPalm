package server

import (
	"context"
	"cxpalm/internal/desktop"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingIdentityReadRequestsReturnUnavailableNotSuccess(t *testing.T) {
	s, _, _ := fixture(t)
	db, err := sql.Open("sqlite", filepath.Join(s.Config.CodexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,archived INTEGER);INSERT INTO threads VALUES('chat','Example','.',1,'vscode',0)`); err != nil {
		t.Fatal(err)
	}
	value, err := s.execute(context.Background(), "phone", Command{Method: "chat.read", Thread: "chat", ReadToken: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal("automatic read surfaced an optional-feature error", err)
	}
	result := value.(map[string]any)
	if result["accepted"] != false || result["available"] != false || result["reason"] != desktop.ReadSyncUnavailableReason {
		t.Fatal(result)
	}
	if token, err := s.Store.ReadToken("chat"); err != nil || token != "" {
		t.Fatal("unverified read persisted", token, err)
	}
	value, err = s.execute(context.Background(), "phone", Command{Method: "chat.notifications.get", Thread: "chat"})
	pref := value.(desktop.NotificationPreferenceState)
	if err != nil || pref.Available || pref.Enabled || pref.Reason == "" {
		t.Fatal(value, err)
	}
	for _, q := range []Command{
		{Method: "chat.read", Thread: "chat", ReadToken: "bad"},
		{Method: "chat.read", Thread: "unknown", ReadToken: strings.Repeat("a", 64)},
		{Method: "chat.notifications.get", Thread: "unknown"},
	} {
		if _, err := s.execute(context.Background(), "phone", q); err == nil {
			t.Fatal("availability masked request validation", q.Method)
		}
	}
}

func TestLostIdentityCannotReplayPreviousAccountNotifications(t *testing.T) {
	s, _, _ := fixture(t)
	notificationAccountFixture(t, s.Config.CodexHome, "account")
	s.notifications.once.Do(func() {})
	if err := s.notifications.syncJournal(s); err != nil {
		t.Fatal(err)
	}
	scope := s.notifications.scope
	pref, err := s.Store.SetNotificationPreference(scope, "chat", true)
	if err != nil {
		t.Fatal(err)
	}
	s.notifications.publishScoped(scope, Notification{ID: "private-event", Thread: "chat", Kind: "complete", Subscription: pref.Generation})
	if err := os.Remove(filepath.Join(s.Config.CodexHome, "auth.json")); err != nil {
		t.Fatal(err)
	}
	var replay map[string]any
	stop := s.notifications.subscribe(s, "baseline:"+scope, "watch", func(e Event) bool {
		if e.Type == "result" {
			replay = e.Data.(map[string]any)
		}
		return true
	})
	stop()
	if len(replay["events"].([]Notification)) != 0 || s.notifications.scope != "" {
		t.Fatal("unverified identity replayed private notifications")
	}
	if s.notifications.publishScoped(scope, Notification{ID: "late-event"}) {
		t.Fatal("old account watcher remained active")
	}
	notificationAccountFixture(t, s.Config.CodexHome, "account")
	if err := s.notifications.syncJournal(s); err != nil || s.notifications.scope != scope || len(s.notifications.recent) != 1 {
		t.Fatal("recovery lost the durable account journal", err)
	}
}
