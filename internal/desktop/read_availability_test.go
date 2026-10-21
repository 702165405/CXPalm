package desktop

import (
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMissingIdentityDisablesOptionalFeaturesWithoutChangingState(t *testing.T) {
	root := t.TempDir()
	nativeReadFixture(t, root, "chat")
	statePath := filepath.Join(root, ".codex-global-state.json")
	before, _ := os.ReadFile(statePath)
	authPath := filepath.Join(root, "auth.json")
	auth, _ := os.ReadFile(authPath)
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
	if err := os.Remove(authPath); err != nil {
		t.Fatal(err)
	}
	if b.ReadSyncAvailable() {
		t.Fatal("cached identity survived logout")
	}
	pref, err := b.NotificationPreference("chat")
	if err != nil || pref.Available || pref.Enabled || pref.Reason == "" || pref.Generation != "" {
		t.Fatal("missing identity did not degrade safely", pref, err)
	}
	raw := json.RawMessage(`{"id":"chat","hasUnreadTurn":true,"turns":[{"turnId":"turn","items":[{"type":"agentMessage","text":"reply"}]}]}`)
	projected := b.PhoneState(State{ID: "chat", Connected: true, Snapshot: raw})
	var view map[string]any
	if err := json.Unmarshal(projected.Snapshot, &view); err != nil {
		t.Fatal(err)
	}
	if view["readSyncAvailable"] != false || view["hasUnreadTurn"] != true || len(view["turns"].([]any)) == 0 {
		t.Fatal("optional read synchronization blocked or falsely read the chat")
	}
	after, _ := os.ReadFile(statePath)
	if string(before) != string(after) {
		t.Fatal("official state changed")
	}
	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatal("a synthetic identity was created")
	}
	if err := os.WriteFile(authPath, auth, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := b.NotificationPreference("chat")
	if err != nil || !restored.Available || restored.NotificationPreference != saved || !b.ReadSyncAvailable() {
		t.Fatal("identity recovery lost the original preference", restored, err)
	}
}
