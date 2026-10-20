package desktop

import (
	"context"
	"cxpalm/internal/config"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInitialHistoryDoesNotWaitForDesktopHandshake(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "rollout.jsonl")
	raw := `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"text":"Visible immediately"}]}}` + "\n"
	if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`CREATE TABLE threads(id TEXT,cwd TEXT,source TEXT,archived INTEGER,updated_at INTEGER,rollout_path TEXT);INSERT INTO threads VALUES('chat','.','vscode',0,1,?)`, path)
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	b := NewBridge(config.Config{CodexHome: home, IPCPath: filepath.Join(home, "absent.sock")})
	defer b.Close()
	// Hold the connection gate to model an arbitrarily slow desktop, not a fast fake.
	b.connectMu.Lock()
	var release sync.Once
	defer release.Do(b.connectMu.Unlock)
	type result struct {
		states <-chan State
		remove func()
		err    error
	}
	ready := make(chan result, 1)
	go func() { s, r, e := b.Subscribe(context.Background(), "chat"); ready <- result{s, r, e} }()
	select {
	case r := <-ready:
		if r.err != nil {
			t.Fatal(r.err)
		}
		defer r.remove()
		s := <-r.states
		if s.Connected || s.Owner != "" || !strings.Contains(string(PhoneState(s).Snapshot), "Visible immediately") {
			t.Fatal("first frame is empty or pretends desktop is connected")
		}
	case <-time.After(time.Second):
		t.Fatal("history is blocked behind the desktop connection gate")
	}
	release.Do(b.connectMu.Unlock)
}

func TestHistoryPathNormalizationDoesNotEscapeRoot(t *testing.T) {
	home := t.TempDir()
	inside := filepath.Join(home, "in.jsonl")
	outside := filepath.Join(t.TempDir(), "out.jsonl")
	for _, p := range []string{inside, outside} {
		if e := os.WriteFile(p, []byte("fixture"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	paths := []string{inside}
	bad := []string{outside, filepath.Join(home, "..", filepath.Base(outside))}
	if runtime.GOOS == "windows" {
		paths = append(paths, `\\?\`+inside)
		bad = append(bad, `\\?\`+outside)
		if historyComparisonPath(`\\?\UNC\server\share\folder`) != `\\server\share\folder` {
			t.Fatal("UNC equivalence not normalized")
		}
	}
	for _, p := range paths {
		f, e := b.openRollout(p)
		if e != nil {
			t.Fatal("equivalent inside path rejected", e)
		}
		f.Close()
	}
	for _, p := range bad {
		if f, e := b.openRollout(p); e == nil {
			f.Close()
			t.Fatal("outside path accepted")
		}
	}
	link := filepath.Join(home, "escape")
	if os.Symlink(filepath.Dir(outside), link) == nil {
		if f, e := b.openRollout(filepath.Join(link, filepath.Base(outside))); e == nil {
			f.Close()
			t.Fatal("symlink escaped history root")
		}
	}
}

func TestPreviewFastFilterStillValidatesMessageRoles(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "rollout.jsonl")
	lines := []string{
		`{"type":"event_msg","payload":{"type":"tool","text":"response_item message"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"text":"never show"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"text":"safe"}]}}`,
	}
	if e := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	raw := b.historyPreview(context.Background(), Session{ID: "test", RolloutPath: path})
	var decoded map[string]any
	if json.Unmarshal(raw, &decoded) != nil || strings.Contains(string(raw), "never show") || !strings.Contains(string(raw), `"text":"safe"`) {
		t.Fatal("filter broke strict type/role validation")
	}
}
