package desktop

import (
	"context"
	"cxpalm/internal/config"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func controlFixture(t *testing.T) (*Bridge, map[string]any, Catalog) {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER); INSERT INTO threads VALUES('thread','fixture',?,0,'vscode','Codex Desktop',0)`, root)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	os.WriteFile(filepath.Join(home, "models_cache.json"), []byte(`{"models":[{"slug":"fixture","display_name":"Fixture","supported_reasoning_levels":[{"effort":"low"},{"effort":"high"}]},{"slug":"low-only","supported_reasoning_levels":[{"effort":"low"}]}]}`), 0600)
	dir := filepath.Join(home, "skills", "example")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: example\ndescription: fixture only\n---\nPRIVATE SKILL BODY"), 0600)
	b := NewBridge(config.Config{CodexHome: home})
	raw := map[string]any{"id": "thread", "latestModel": "fixture", "latestReasoningEffort": "high", "latestCollaborationMode": map[string]any{"mode": "default"}, "threadRuntimeStatus": map[string]any{"type": "idle"}}
	catalog, err := b.Catalog(root)
	if err != nil {
		t.Fatal(err)
	}
	return b, raw, catalog
}
func TestSettingsValidationAndNoImplicitEscalation(t *testing.T) {
	b, raw, c := controlFixture(t)
	defer b.Close()
	revision := ControlsRevision(raw)
	for _, options := range []Options{{Model: "unknown", Expected: revision}, {Model: "fixture", Effort: "ultra", Expected: revision}, {Mode: "unsafe", Expected: revision}, {Permission: "full", Expected: revision}, {Model: "fixture", Expected: "stale"}} {
		if _, err := buildSettings(raw, options, c); err == nil {
			t.Fatal("unsafe/stale settings accepted")
		}
	}
	settings, err := buildSettings(raw, Options{Model: "low-only", Mode: "plan", Expected: revision}, c)
	if err != nil {
		t.Fatal(err)
	}
	if settings["effort"] != nil || settings["permissions"] != nil {
		t.Fatal("model change retained unsupported effort or escalated permissions")
	}
	if settings["collaborationMode"].(map[string]any)["mode"] != "plan" {
		t.Fatal("plan mode lost")
	}
	settings, err = buildSettings(raw, Options{Permission: "full", ConfirmFullAccess: true, Expected: revision}, c)
	if err != nil || settings["permissions"] != ":danger-full-access" || settings["approvalPolicy"] != "never" {
		t.Fatal("explicit permission selection not represented")
	}
	for _, permission := range []string{"read-only", "workspace", "auto-review"} {
		settings, err := buildSettings(raw, Options{Permission: permission, Expected: revision}, c)
		reviewer := "user"
		if permission == "auto-review" {
			reviewer = "auto_review"
		}
		if err != nil || settings["approvalsReviewer"] != reviewer || settings["approvalPolicy"] != "on-request" {
			t.Fatalf("incorrect approval reviewer for %s: %v %v", permission, settings, err)
		}
		if permission == "auto-review" && settings["permissions"] != ":workspace" {
			t.Fatal("auto review disabled sandbox")
		}
	}
	encoded, _ := json.Marshal(c)
	if strings.Contains(string(encoded), "PRIVATE") || strings.Contains(string(encoded), b.config.CodexHome) {
		t.Fatal("catalog leaked skill body or filesystem path")
	}
}
func TestControlsUseOnlyOriginalOwnerFollowerIPC(t *testing.T) {
	for _, action := range []string{"chat.settings", "chat.compact", "chat.send"} {
		t.Run(action, func(t *testing.T) {
			b, raw, catalog := controlFixture(t)
			a, peer := net.Pipe()
			defer peer.Close()
			ipc := &IPC{conn: a, pending: map[string]chan Frame{}, closed: make(chan struct{}), events: make(chan Frame, 4), clientID: "fixture"}
			go ipc.read()
			b.ipc = ipc
			defer b.Close()
			encoded, _ := json.Marshal(raw)
			b.sessions["thread"] = &State{ID: "thread", Owner: "original-owner", Connected: true, Snapshot: encoded}
			observed := make(chan Frame, 1)
			go func() {
				f, err := readTestFrame(peer)
				if err != nil {
					return
				}
				observed <- f
				encoded, _ := json.Marshal(Frame{Type: "response", RequestID: f.RequestID, ResultType: "success", Result: json.RawMessage(`{"applied":true}`)})
				var header [4]byte
				binary.LittleEndian.PutUint32(header[:], uint32(len(encoded)))
				writeAll(peer, header[:])
				writeAll(peer, encoded)
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			options := Options{}
			var skills []string
			if action == "chat.settings" {
				options = Options{Model: "fixture", Effort: "low", Mode: "plan", Expected: ControlsRevision(raw)}
			}
			if action == "chat.send" {
				skills = []string{catalog.Skills[0].ID}
			}
			if _, err := b.ControlAction(ctx, "thread", "operation", action, "hello", "", nil, options, skills); err != nil {
				t.Fatal(err)
			}
			f := <-observed
			if f.Target != "original-owner" || !strings.HasPrefix(f.Method, "thread-follower-") {
				t.Fatal("main chat escaped original owner")
			}
			if action == "chat.settings" && (f.Method != "thread-follower-update-thread-settings" || f.Version != 2) {
				t.Fatal("wrong settings protocol")
			}
			if action == "chat.compact" && f.Method != "thread-follower-compact-thread" {
				t.Fatal("wrong compact protocol")
			}
			if action == "chat.send" && !strings.Contains(string(f.Params), `"type":"skill"`) {
				t.Fatal("skill was only decorative, not passed as native input")
			}
		})
	}
}
func TestProjectionControlsRedactsInstructions(t *testing.T) {
	_, raw, _ := controlFixture(t)
	raw["latestThreadSettings"] = map[string]any{"model": "fixture", "developer_instructions": "PRIVATE"}
	raw["latestCollaborationMode"] = map[string]any{"mode": "plan", "settings": map[string]any{"model": "fixture", "developer_instructions": "PRIVATE"}}
	encoded, _ := json.Marshal(raw)
	out := PhoneState(State{Snapshot: encoded})
	if strings.Contains(string(out.Snapshot), "PRIVATE") || !strings.Contains(string(out.Snapshot), "controlsRevision") {
		t.Fatal("control projection exposes hidden instructions or lacks revision")
	}
}
