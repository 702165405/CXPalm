package desktop

import (
	"context"
	"cxpalm/internal/store"
	"encoding/binary"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func queueFixture(t *testing.T) (*Bridge, string, net.Conn) {
	b, raw, _ := controlFixture(t)
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b.ReadStore = db
	login, err := db.SetLogin(store.LoginInput{Username: "fixture", Password: "fixture-password-012345"})
	_ = login
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := db.Login("fixture", "fixture-password-012345", "queue fixture", "", true)
	if err != nil {
		t.Fatal(err)
	}
	a, peer := net.Pipe()
	c := &IPC{conn: a, pending: map[string]chan Frame{}, closed: make(chan struct{}), events: make(chan Frame, 16), clientID: "queue-fixture"}
	go c.read()
	b.ipc = c
	b.sessions["thread"] = &State{ID: "thread", Owner: "original-owner", Connected: true, Snapshot: mustJSON(raw)}
	t.Cleanup(func() { b.Close(); peer.Close(); db.Close() })
	return b, d.ID, peer
}
func TestQueueWaitsForIdleAndNativeConfirmationWithoutReplay(t *testing.T) {
	b, device, peer := queueFixture(t)
	first, err := b.ReadStore.Enqueue(device, "first", "thread", "hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.ReadStore.Enqueue(device, "second", "thread", "next", nil)
	if err != nil {
		t.Fatal(err)
	}
	set := func(kind string, seen bool) {
		raw := map[string]any{"id": "thread", "threadRuntimeStatus": map[string]any{"type": kind}}
		if seen {
			raw["turns"] = []any{map[string]any{"params": map[string]any{"clientUserMessageId": first}}}
		}
		b.mu.Lock()
		b.sessions["thread"].Snapshot = mustJSON(raw)
		b.mu.Unlock()
	}
	set("active", false)
	b.queueStep("thread")
	rows, _ := b.ReadStore.Queue("thread")
	if rows[0].Status != "queued" {
		t.Fatal("dispatched while active")
	}
	observed := make(chan Frame, 1)
	go func() {
		f, e := readTestFrame(peer)
		if e != nil {
			return
		}
		observed <- f
		r, _ := json.Marshal(Frame{Type: "response", RequestID: f.RequestID, ResultType: "success", Result: json.RawMessage(`{"method":"thread-follower-start-turn","result":{"result":{"turn":{"id":"native-turn"}}}}`)})
		var h [4]byte
		binary.LittleEndian.PutUint32(h[:], uint32(len(r)))
		writeAll(peer, h[:])
		writeAll(peer, r)
	}()
	set("idle", false)
	b.queueStep("thread")
	select {
	case f := <-observed:
		if f.Method != "thread-follower-start-turn" || f.Target != "original-owner" {
			t.Fatal("escaped original owner")
		}
	case <-time.After(time.Second):
		t.Fatal("no dispatch")
	}
	rows, _ = b.ReadStore.Queue("thread")
	if rows[0].Status != "sent" {
		t.Fatal(rows)
	}
	b.queueStep("thread")
	rows, _ = b.ReadStore.Queue("thread")
	if len(rows) != 2 || rows[1].ID != second || rows[1].Status != "queued" {
		t.Fatal("stale idle snapshot advanced queue")
	}
	set("active", true)
	b.queueStep("thread")
	b.queueStep("thread")
	rows, _ = b.ReadStore.Queue("thread")
	if len(rows) != 1 || rows[0].Status != "queued" {
		t.Fatal(rows)
	}
	b.ReadStore.Revoke(device)
	set("idle", false)
	b.queueStep("thread")
	rows, _ = b.ReadStore.Queue("thread")
	if rows[0].Status != "unknown" {
		t.Fatal("revoked queue executed")
	}
	if _, err := b.CancelQueue(context.Background(), "thread", second); err != nil {
		t.Fatal(err)
	}
}
func TestQueueAmbiguousOwnerResponseStopsHead(t *testing.T) {
	b, device, peer := queueFixture(t)
	b.ReadStore.Enqueue(device, "lost", "thread", "one", nil)
	b.ReadStore.Enqueue(device, "next", "thread", "two", nil)
	go func() { readTestFrame(peer); peer.Close() }()
	b.queueStep("thread")
	b.queueStep("thread")
	rows, _ := b.ReadStore.Queue("thread")
	if len(rows) != 2 || rows[0].Status != "unknown" || rows[1].Status != "queued" {
		t.Fatal(rows)
	}
	for _, raw := range []string{`{}`, `{"result":{"status":"outcome-unknown"}}`, `{"result":{"status":"rejected"}}`} {
		if queueAccepted([]byte(raw)) {
			t.Fatal("ambiguous result accepted", raw)
		}
	}
}
