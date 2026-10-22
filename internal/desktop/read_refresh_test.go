package desktop

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"cxpalm/internal/config"
)

func TestReadBroadcastCoalescesPassiveOwnerRefresh(t *testing.T) {
	b, c, owner, _ := readFixture(t, false)
	started := time.Now()
	for i := 0; i < 30; i++ {
		b.learnReadContext(c, readHint(readTestContext(), "desktop-coordinator"))
	}
	f := readFrame(t, owner)
	var p struct {
		ID        string `json:"conversationId"`
		Following bool   `json:"following"`
	}
	json.Unmarshal(f.Params, &p)
	if f.Method != "thread-stream-following-changed" || f.Version != 1 ||
		len(f.Targets) != 1 || f.Targets[0] != "original-owner" || p.ID != "chat" || !p.Following {
		t.Fatal("refresh did not use the passive original-owner path")
	}
	if time.Since(started) > 600*time.Millisecond {
		t.Fatal("read broadcast waited for maintenance sweep")
	}
	if readView(t, b)["hasUnreadTurn"] != true {
		t.Fatal("broadcast optimistically cleared unread")
	}
	owner.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buffer := make([]byte, 1)
	if n, _ := owner.Read(buffer); n != 0 {
		t.Fatal("broadcast burst created repeated snapshot requests")
	}
}

func TestMarkReadRefreshesSnapshotWithoutReplayingWrite(t *testing.T) {
	b, c, owner, token := readFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan bool, 1)
	started := time.Now()
	go func() { accepted, _ := b.MarkRead(ctx, "chat", token); result <- accepted }()
	if f := readFrame(t, owner); f.Method != "thread-read-state-changed" {
		t.Fatal("missing original read mutation")
	}
	if f := readFrame(t, owner); f.Method != "thread-stream-following-changed" {
		t.Fatal("confirmation retry replayed the write instead of requesting a snapshot")
	}
	publishRead(t, b, c, "original-owner", false, "reply")
	if !<-result || time.Since(started) > 600*time.Millisecond {
		t.Fatal("fresh owner confirmation did not finish promptly")
	}
	t.Logf("isolated read confirmation after passive refresh: %s", time.Since(started))
}

func TestSessionSignalsWakeForStatusNotBodyTokens(t *testing.T) {
	b := NewBridge(config.Config{})
	defer b.Close()
	s := &State{ID: "chat", Connected: true, Snapshot: mustJSON(map[string]any{
		"hasUnreadTurn": true, "threadRuntimeStatus": map[string]any{"type": "active"},
		"turns": []any{map[string]any{"items": []any{map[string]any{"text": "first"}}}},
	})}
	b.mu.Lock()
	b.emitLocked(s)
	b.mu.Unlock()
	quiet := b.ReadChanges()
	for i := 0; i < 100; i++ {
		s.Snapshot = mustJSON(map[string]any{"hasUnreadTurn": true,
			"threadRuntimeStatus": map[string]any{"type": "active"}, "turns": []any{i}})
		b.mu.Lock()
		b.emitLocked(s)
		b.mu.Unlock()
	}
	select {
	case <-quiet:
		t.Fatal("body-only updates woke catalogue scans")
	default:
	}
	s.Snapshot = mustJSON(map[string]any{"hasUnreadTurn": true,
		"threadRuntimeStatus": map[string]any{"type": "idle"},
		"turns":               []any{map[string]any{"status": "completed"}}})
	b.mu.Lock()
	b.emitLocked(s)
	b.mu.Unlock()
	select {
	case <-quiet:
	default:
		t.Fatal("completed status waited for list polling")
	}
}
