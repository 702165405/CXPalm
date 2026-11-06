package desktop

import (
	"context"
	"cxpalm/internal/config"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRolloutReadRemainsRootedWhileDirectoryChanges(t *testing.T) {
	home, outside := t.TempDir(), t.TempDir()
	current := filepath.Join(home, "current")
	parked := filepath.Join(home, "parked")
	if err := os.Mkdir(current, 0700); err != nil {
		t.Fatal(err)
	}
	for directory, text := range map[string]string{current: "inside", outside: "outside-private"} {
		if err := os.WriteFile(filepath.Join(directory, "rollout.jsonl"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	probe := filepath.Join(home, "link-probe")
	if err := os.Symlink(outside, probe); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	bridge := NewBridge(config.Config{CodexHome: home})
	defer bridge.Close()
	readable, err := bridge.openRollout(filepath.Join(current, "rollout.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	readable.Close()
	stop, done := make(chan struct{}), make(chan struct{})
	defer func() { close(stop); <-done }()
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if os.Rename(current, parked) != nil {
				continue
			}
			if os.Symlink(outside, current) == nil {
				_ = os.Remove(current)
			}
			_ = os.Rename(parked, current)
		}
	}()
	successfulReads := 0
	for attempt := 0; attempt < 1000; attempt++ {
		file, err := bridge.openRollout(filepath.Join(current, "rollout.jsonl"))
		if err != nil {
			continue
		}
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "inside" {
			t.Fatal("history read escaped its root during directory replacement")
		}
		successfulReads++
	}
	if successfulReads == 0 {
		t.Fatal("no successful reads during directory replacement")
	}
}

func TestRolloutLongPathWithExtendedWindowsRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows extended path compatibility")
	}
	home := t.TempDir()
	directory := filepath.Join(home, strings.Repeat("segment", 15), strings.Repeat("segment", 15))
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "rollout.jsonl")
	if err := os.WriteFile(path, []byte("inside"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{home, `\\?\` + home} {
		bridge := NewBridge(config.Config{CodexHome: root})
		for _, source := range []string{path, `\\?\` + path} {
			file, err := bridge.openRollout(source)
			if err != nil {
				bridge.Close()
				t.Fatal(err)
			}
			file.Close()
		}
		bridge.Close()
	}
}

func TestArchivedBodyCanBeReadWithoutDesktopOrExecution(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "archive.jsonl")
	content := `{"type":"response_item","payload":{"type":"message","role":"system","content":[{"text":"must not display system instructions"}]}}` + "\n" + `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"text":"Old question"}]}}` + "\n" + `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"text":"Old answer <script>literal</script>"}]}}` + "\n"
	if e := os.WriteFile(path, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER,rollout_path TEXT);INSERT INTO threads VALUES('archive','Archive','/old',1,'vscode','Codex Desktop',1,?)`, path)
	if e != nil {
		t.Fatal(e)
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	states, remove, e := b.Subscribe(context.Background(), "archive")
	if e != nil {
		t.Fatal(e)
	}
	defer remove()
	state := PhoneState(<-states)
	if state.Connected || !state.Archived || state.Owner != "" || b.ipc != nil {
		t.Fatal("archive preview pretends to execute")
	}
	var raw struct {
		Source string `json:"historySource"`
		Turns  []struct {
			Items []struct {
				Text string `json:"text"`
			} `json:"items"`
		} `json:"turns"`
	}
	if e = json.Unmarshal(state.Snapshot, &raw); e != nil {
		t.Fatal(e)
	}
	if raw.Source != "rollout" || len(raw.Turns) != 1 || len(raw.Turns[0].Items) != 2 || raw.Turns[0].Items[0].Text != "Old question" || raw.Turns[0].Items[1].Text != "Old answer <script>literal</script>" {
		t.Fatal("archive body missing or system instructions exposed")
	}
}
