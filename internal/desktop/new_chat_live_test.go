package desktop

import (
	"context"
	"cxpalm/internal/config"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Explicit opt-in creates exactly one empty test chat and attempts to archive only that chat.
// It never starts a turn, sends a prompt, or changes an existing chat.
func TestLiveCreateProjectlessEmptyChat(t *testing.T) {
	path := os.Getenv("CXPALM_LIVE_CREATE_CONFIG")
	if path == "" {
		t.Skip("explicit empty-chat acceptance not requested")
	}
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBridge(c)
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	start := time.Now()
	value, err := b.Lifecycle(ctx, "sessions.create", "", "none")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(value)
	var created struct {
		ID      string `json:"id"`
		Opening bool   `json:"opening"`
	}
	if json.Unmarshal(raw, &created) != nil || !threadID.MatchString(created.ID) {
		t.Fatal("creation not confirmed")
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		if _, err := metadataCall(cleanup, c, "thread/archive", map[string]any{"threadId": created.ID}); err != nil {
			t.Logf("Desktop owner requires archiving the empty test chat through the official app: %s (%v)", created.ID, err)
		}
	}()
	confirmed := time.Since(start)
	states, remove, err := b.Subscribe(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer remove()
	for {
		select {
		case state := <-states:
			if state.Connected {
				var native map[string]any
				if json.Unmarshal(state.Snapshot, &native) != nil {
					t.Fatal("invalid desktop state")
				}
				if len(orderedTurns(native)) != 0 {
					t.Fatal("unexpected turn in empty test chat")
				}
				row, err := b.History.Get(ctx, created.ID)
				if err != nil || !isPalmProjectless(row.CWD) {
					t.Fatal("not a dedicated projectless workspace")
				}
				t.Logf("Real empty chat persisted, opened by official desktop; creation confirmation=%s owner ready=%s; no model turn; cleanup attempted separately", confirmed, time.Since(start))
				return
			}
		case <-ctx.Done():
			t.Fatal("created empty chat was not connected by official desktop within deadline")
		}
	}
}
