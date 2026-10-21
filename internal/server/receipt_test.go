package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"cxpalm/internal/store"
)

func TestLegacyArchiveRejectionRequiresOriginalFingerprintAndDoesNotReplay(t *testing.T) {
	s, _, _ := fixture(t)
	thread := "11111111-1111-4111-8111-111111111111"
	for _, method := range []string{"sessions.archive", "sessions.unarchive", "sessions.create", "chat.send"} {
		command := Command{ID: method, Method: method, Thread: thread}
		encoded, _ := json.Marshal(command)
		_, _, err := s.Store.Begin("phone", method, store.Hash(string(encoded)))
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Store.Finish("phone", method, "unknown", map[string]string{"error": "official metadata API: thread " + thread + " already has an active writer"})
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.resolveOperation("phone", method)
		if err != nil {
			t.Fatal(err)
		}
		want := "unknown"
		if method == "sessions.archive" || method == "sessions.unarchive" {
			want = "rejected"
		}
		if r.Status != want {
			t.Fatal(method, r)
		}
		// The historical journal is untouched and original IDs remain fenced.
		raw, _ := s.Store.Receipt("phone", method)
		if raw.Status != "unknown" {
			t.Fatal("audit record overwritten")
		}
		_, fresh, _ := s.Store.Begin("phone", method, store.Hash(string(encoded)))
		if fresh {
			t.Fatal("legacy rejection can replay")
		}
		other, err := s.resolveOperation("another-device", method)
		if err != nil || other.Status != "not_sent" {
			t.Fatal("cross-device receipt leak", other, err)
		}
	}
}

func TestKnownArchivePreconditionSettlesWithoutPretendingSuccess(t *testing.T) {
	s, _, _ := fixture(t)
	db, err := sql.Open("sqlite", filepath.Join(s.Config.CodexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id := "11111111-1111-4111-8111-111111111111"
	_, err = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER);INSERT INTO threads VALUES(?,'fixture','.',1,'vscode','Codex Desktop',0)`, id)
	if err != nil {
		t.Fatal(err)
	}
	command := Command{ID: "blocked", Method: "sessions.archive", Thread: id}
	for i := 0; i < 2; i++ {
		value, err := s.execute(context.Background(), "phone", command)
		if err != nil {
			t.Fatal(err)
		}
		receipt := value.(store.Receipt)
		if receipt.Status != "rejected" {
			t.Fatal("known precondition stayed unknown", receipt)
		}
		var result map[string]string
		_ = json.Unmarshal(receipt.Result, &result)
		if result["code"] != "archive_state_unavailable" {
			t.Fatal(result)
		}
	}
}
