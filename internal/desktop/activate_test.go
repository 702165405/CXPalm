package desktop

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

const activationID = "01234567-1234-1234-1234-123456789abc"

func activationFixture(t *testing.T) *Bridge {
	t.Helper()
	b, _, _ := controlFixture(t)
	t.Cleanup(b.Close)
	b.config.IPCPath = filepath.Join(b.config.CodexHome, "absent.sock")
	db, err := sql.Open("sqlite", filepath.Join(b.config.CodexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("UPDATE threads SET id=? WHERE id='thread'", activationID)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	b.sessions[activationID] = &State{ID: activationID}
	b.watchers[activationID] = map[chan State]bool{make(chan State, 4): true}
	return b
}
func TestSelectedChatActivationRequiresAuthorizedDisconnectedWatcher(t *testing.T) {
	for _, kind := range []string{"cold", "connected", "archived", "unwatched", "unknown", "invalid", "canceled", "closed", "open-failed"} {
		t.Run(kind, func(t *testing.T) {
			b := activationFixture(t)
			id := activationID
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "connected":
				b.sessions[id].Connected = true
			case "archived":
				b.sessions[id].Archived = true
			case "unwatched":
				delete(b.watchers, id)
			case "unknown":
				id = "01234567-1234-1234-1234-123456789def"
				b.sessions[id] = &State{ID: id}
				b.watchers[id] = b.watchers[activationID]
			case "invalid":
				id = "../../private"
			case "canceled":
				cancel()
			case "closed":
				b.Close()
			}
			calls := 0
			b.openSelectedChat(ctx, id, 0, func(ctx context.Context, got string) error {
				calls++
				if got != activationID {
					t.Fatal("unauthorized URL")
				}
				if kind == "open-failed" {
					return errors.New("fixture launch failure")
				}
				return nil
			})
			want := 0
			if kind == "cold" || kind == "open-failed" {
				want = 1
			}
			if calls != want {
				t.Fatalf("launches=%d want=%d", calls, want)
			}
			if kind == "cold" || kind == "open-failed" {
				if _, err := b.Snapshot(id); err == nil {
					t.Fatal("opening GUI granted execution authority without native snapshot")
				}
			}
			if kind == "open-failed" && b.sessions[id].Error == "" {
				t.Fatal("missing launch failure state")
			}
		})
	}
}
func TestSelectedChatActivationDeduplicatesAndLetsNativeSnapshotWin(t *testing.T) {
	b := activationFixture(t)
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	var calls atomic.Int32
	open := func(context.Context, string) error { calls.Add(1); close(started); <-release; return nil }
	go func() { b.openSelectedChat(context.Background(), activationID, 0, open); close(finished) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("opening not started")
	}
	b.openSelectedChat(context.Background(), activationID, 0, open)
	close(release)
	<-finished
	if calls.Load() != 1 {
		t.Fatal("duplicate official open")
	}
	ready := make(chan struct{})
	go func() {
		b.openSelectedChat(context.Background(), activationID, 50*time.Millisecond, func(context.Context, string) error { calls.Add(1); return nil })
		close(ready)
	}()
	b.mu.Lock()
	b.sessions[activationID].Connected = true
	b.mu.Unlock()
	<-ready
	if calls.Load() != 1 {
		t.Fatal("opened an already synchronized owner")
	}
}
func TestCanceledSelectionDoesNotOpenAnotherConversation(t *testing.T) {
	b := activationFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var calls atomic.Int32
	go func() {
		b.openSelectedChat(ctx, activationID, time.Hour, func(context.Context, string) error { calls.Add(1); return nil })
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled selection remained pending")
	}
	if calls.Load() != 0 {
		t.Fatal("opened canceled selection")
	}
}

func TestRepeatedSelectedSubscriptionsDoNotReopenDesktopDuringCooldown(t *testing.T) {
	b := activationFixture(t)
	calls := 0
	open := func(context.Context, string) error { calls++; return nil }
	for i := 0; i < 8; i++ {
		b.openSelectedChat(context.Background(), activationID, 0, open)
	}
	if calls != 1 {
		t.Fatal("sequential resubscriptions reopened official desktop", calls)
	}
	b.mu.Lock()
	b.sessions[activationID].lastOpen = time.Now().Add(-2 * time.Minute)
	b.mu.Unlock()
	b.openSelectedChat(context.Background(), activationID, 0, open)
	if calls != 2 {
		t.Fatal("bounded retry never permits recovery")
	}
}
