package desktop

import (
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMissingContextDisablesOnlyOfficialReadWrites(t *testing.T) {
	root := t.TempDir()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	b := NewBridge(config.Config{CodexHome: root})
	defer b.Close()
	b.ReadStore = db
	scope, err := b.NotificationScope()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := db.SetNotificationPreference(scope, "chat", true)
	if err != nil {
		t.Fatal(err)
	}
	if b.ReadSyncAvailable() || b.OfficialReadSyncAvailable() {
		t.Fatal("missing context reported write capability")
	}
	pref, err := b.NotificationPreference("chat")
	if err != nil || !pref.Available || pref.NotificationPreference != saved {
		t.Fatal("notifications depend on read context", err)
	}
	raw := json.RawMessage(`{"id":"chat","hasUnreadTurn":true,"turns":[{"turnId":"turn","items":[{"type":"agentMessage","text":"reply"}]}]}`)
	var view map[string]any
	json.Unmarshal(b.PhoneState(State{ID: "chat", Connected: true, Snapshot: raw}).Snapshot, &view)
	if view["readSyncAvailable"] != false || view["hasUnreadTurn"] != true || view["notificationPreference"].(map[string]any)["available"] != true {
		t.Fatal("desktop read state or notifications changed", view)
	}
	for _, name := range []string{"auth.json", ".codex-global-state.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatal("official file created", name)
		}
	}
}

func TestNotificationScopeNeedsNoOfficialIdentityOrReadState(t *testing.T) {
	root := t.TempDir()
	b := NewBridge(config.Config{CodexHome: root})
	defer b.Close()
	scope, err := b.NotificationScope()
	if err != nil || scope == "" {
		t.Fatal(err)
	}
	for _, name := range []string{"auth.json", ".codex-global-state.json"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(`invalid`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if next, err := b.NotificationScope(); err != nil || next != scope {
		t.Fatal("files changed notification scope", err)
	}
	other := NewBridge(config.Config{CodexHome: t.TempDir()})
	defer other.Close()
	if next, err := other.NotificationScope(); err != nil || next == scope {
		t.Fatal("different installations share notification scope", err)
	}
}
