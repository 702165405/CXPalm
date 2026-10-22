package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadConfirmationLaneIsNonblockingBoundedAndScoped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slots := make(chan struct{}, 2)
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	events := make(chan Event, 8)
	var calls atomic.Int32
	execute := func(ctx context.Context, q Command) (any, error) {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
		}
		return map[string]bool{"accepted": true}, nil
	}
	send := func(e Event) bool { events <- e; return true }
	for _, id := range []string{"one", "two"} {
		done := make(chan struct{})
		go func(id string) {
			dispatchRead(ctx, slots, Command{ID: id, Method: "chat.read", Thread: id}, execute, send)
			close(done)
		}(id)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("read confirmation blocked command dispatcher")
		}
		<-started
	}
	dispatchRead(ctx, slots, Command{ID: "overflow", Method: "chat.read", Thread: "other"}, execute, send)
	e := <-events
	if e.ID != "overflow" || e.Type != "result" || e.Data.(map[string]any)["accepted"] != false || calls.Load() != 2 {
		t.Fatal("read overflow was unbounded or falsely accepted", e)
	}
	dispatchRead(ctx, slots, Command{ID: "send", Method: "chat.send"}, execute, send)
	if e := <-events; e.Type != "error" || calls.Load() != 2 {
		t.Fatal("ordered write entered background lane")
	}
	close(release)
	for i := 0; i < 2; i++ {
		select {
		case e := <-events:
			if e.Type != "result" {
				t.Fatal(e)
			}
		case <-time.After(time.Second):
			t.Fatal("read result lost")
		}
	}
}
