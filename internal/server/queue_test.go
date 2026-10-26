package server

import (
	"context"
	"cxpalm/internal/desktop"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestQueueSharedProjectionAndCancelReceiptIsNotReplayed(t *testing.T) {
	s, _, token := fixture(t)
	first, err := s.Store.Authorize(token)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := loginFixture(t, s.Store, "second client")
	if err != nil {
		t.Fatal(err)
	}
	home := s.Config.CodexHome
	db, err := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,archived INTEGER);INSERT INTO threads VALUES('thread','fixture','.',1,'vscode',0)`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Store.Enqueue(first.ID, "enqueue-op", "thread", "shared queue", nil)
	if err != nil {
		t.Fatal(err)
	}
	state := desktop.State{ID: "thread", Snapshot: json.RawMessage(`{"id":"thread","turns":[]}`)}
	for range []string{first.ID, second.ID} {
		var raw map[string]any
		out := s.Bridge.PhoneState(state)
		if err = json.Unmarshal(out.Snapshot, &raw); err != nil {
			t.Fatal(err)
		}
		if rows, ok := raw["queuedMessages"].([]any); !ok || len(rows) != 1 {
			t.Fatal("queue not shared", raw)
		}
	}
	command := Command{ID: "cancel-op", Method: "chat.queue.cancel", Thread: "thread", RequestID: id}
	result, err := s.execute(context.Background(), second.ID, command)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	if !json.Valid(encoded) {
		t.Fatal("invalid receipt")
	}
	if _, err = s.execute(context.Background(), second.ID, command); err != nil {
		t.Fatal("cancel replay should return original receipt", err)
	}
	rows, err := s.Store.Queue("thread")
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}
