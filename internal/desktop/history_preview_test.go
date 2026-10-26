package desktop

import (
	"context"
	"cxpalm/internal/config"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
