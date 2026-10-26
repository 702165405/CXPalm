package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFullListSweepIsPassiveUnfilteredIdempotentAndRemovesArchived(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	b.config.IPCPath = filepath.Join(b.config.CodexHome, "absent.sock")
	db, err := sql.Open("sqlite", filepath.Join(b.config.CodexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 501; i++ {
		if _, err = tx.Exec("INSERT INTO threads VALUES(?,?,?,0,'vscode','Codex Desktop',0)", fmt.Sprint("old-", i), "Historical", "."); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	watches := map[string]func(){}
	defer func() {
		for _, remove := range watches {
			remove()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b.syncSessionWatches(ctx, watches)
	if len(watches) != 502 {
		t.Fatal("sweep stopped at visible page or 500-row database boundary", len(watches))
	}
	b.syncSessionWatches(ctx, watches)
	b.mu.Lock()
	for id, state := range b.sessions {
		if len(b.watchers[id]) != 1 || !state.lastOpen.IsZero() {
			t.Fatal("duplicate watcher or GUI activation", id)
		}
	}
	progress := b.listProgress
	b.mu.Unlock()
	if progress.Checked != 502 || progress.Total != 502 || progress.Scanning || progress.Error != "" {
		t.Fatal(progress)
	}
	if _, err = db.Exec("UPDATE threads SET archived=1 WHERE id='old-500'"); err != nil {
		t.Fatal(err)
	}
	b.syncSessionWatches(ctx, watches)
	if len(watches) != 501 || watches["old-500"] != nil {
		t.Fatal("archived passive watcher retained")
	}
	b.mu.Lock()
	n := len(b.watchers["old-500"])
	b.mu.Unlock()
	if n != 0 {
		t.Fatal("archive leaked watcher")
	}
}
func TestHistoricalTerminalSummaryIsExplicitAndDoesNotGrantLiveAuthority(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	path := filepath.Join(b.config.CodexHome, "history.jsonl")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	complete := `{"type":"event_msg","payload":{"type":"task_complete"}}` + "\n"
	write(complete)
	row := Session{RolloutPath: path, Status: "unknown"}
	b.replyDate(&row)
	if row.Status != "completed" || row.StatusSource != "history" || row.Live {
		t.Fatal(row)
	}
	// A stale native cache must not hide newer rooted historical completion.
	enrichSession(&row, State{Connected: false, Snapshot: mustJSON(map[string]any{"threadRuntimeStatus": map[string]any{"type": "active"}})})
	if row.Status != "completed" || row.Live {
		t.Fatal("stale owner overrode provenance", row)
	}
	write(complete + `{"type":"event_msg","payload":{"type":"task_started"}}` + "\n")
	b.replyDate(&row)
	if row.Status != "unknown" || row.StatusSource != "" || row.Live {
		t.Fatal("old completion survived newer active boundary", row)
	}
	write(`{"type":"event_msg","payload":{"type":"turn_aborted"}}` + "\n")
	b.replyDate(&row)
	if row.Status != "interrupted" || row.StatusSource != "history" {
		t.Fatal(row)
	}
	enrichSession(&row, State{Connected: true, Snapshot: mustJSON(map[string]any{"threadRuntimeStatus": map[string]any{"type": "active"}})})
	if row.Status != "running" || row.StatusSource != "desktop" || !row.Live {
		t.Fatal(row)
	}
	if _, err := b.Snapshot("history"); err == nil {
		t.Fatal("history manufactured native owner")
	}
}

func TestOfficialArchiveAndDeletionRemoveCachedAuthorityAndLiveCounts(t *testing.T) {
	for _, archive := range []bool{false, true} {
		t.Run(fmt.Sprint(archive), func(t *testing.T) {
			b, _, _ := controlFixture(t)
			defer b.Close()
			b.config.IPCPath = filepath.Join(b.config.CodexHome, "absent.sock")
			db, e := sql.Open("sqlite", filepath.Join(b.config.CodexHome, "state_5.sqlite"))
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			if _, e = db.Exec(`INSERT INTO threads VALUES('gone','Gone','.',1,'vscode','Codex Desktop',0)`); e != nil {
				t.Fatal(e)
			}
			for _, id := range []string{"thread", "gone"} {
				b.sessions[id] = &State{ID: id, Owner: "official-owner", Connected: true, Snapshot: mustJSON(map[string]any{"id": id, "threadRuntimeStatus": map[string]any{"type": "idle"}})}
			}
			watches := map[string]func(){}
			defer func() {
				for _, remove := range watches {
					remove()
				}
			}()
			b.syncSessionWatches(context.Background(), watches)
			if archive {
				_, e = db.Exec(`UPDATE threads SET archived=1 WHERE id='gone'`)
			} else {
				_, e = db.Exec(`DELETE FROM threads WHERE id='gone'`)
			}
			if e != nil {
				t.Fatal(e)
			}
			b.syncSessionWatches(context.Background(), watches)
			page, e := b.Browse(context.Background(), "", false, "", 100, 0)
			if e != nil {
				t.Fatal(e)
			}
			if len(page.Items) != 1 || page.Sync.Total != 1 || page.Sync.Live != 1 {
				t.Fatal("removed cache inflated current membership", page)
			}
			if _, e = b.nativeSnapshot("gone"); e == nil {
				t.Fatal("removed official row kept write authority")
			}
			if b.sessions["gone"].Archived != archive {
				t.Fatal("archive was inferred incorrectly")
			}
			c := &IPC{events: make(chan Frame, 1)}
			done := make(chan struct{})
			go func() { b.events(c); close(done) }()
			c.events <- Frame{Method: "thread-stream-state-changed", Version: 11, Source: "official-owner", Params: mustJSON(map[string]any{"hostId": "local", "conversationId": "gone", "change": map[string]any{"type": "snapshot", "revision": 2, "conversationState": map[string]any{"id": "gone"}}})}
			close(c.events)
			<-done
			if b.sessions["gone"].Connected {
				t.Fatal("late native frame revived removed owner")
			}

			if archive {
				page, e = b.Browse(context.Background(), "", true, "", 100, 0)
				if e != nil || len(page.Items) != 1 || page.Items[0].ID != "gone" {
					t.Fatal("official archive not mirrored", page, e)
				}
			}
		})
	}
}
