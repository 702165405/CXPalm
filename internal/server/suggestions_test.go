package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"cxpalm/internal/desktop"
)

func TestSuggestionLaneDoesNotBlockAndCancelsWithSelection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slots := make(chan struct{}, 1)
	started := make(chan struct{})
	stopped := make(chan struct{})
	events := make(chan Event, 4)
	execute := func(ctx context.Context, command Command) (any, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	}
	send := func(event Event) bool { events <- event; return true }
	dispatchSuggestion(ctx, slots, Command{ID: "first", Method: "chat.suggest"}, func() bool { return true }, execute, send)
	<-started
	dispatchSuggestion(ctx, slots, Command{ID: "overflow", Method: "chat.suggest"}, func() bool { return true }, execute, send)
	if event := <-events; event.ID != "overflow" || event.Data.(desktop.Suggestion).Text != "" {
		t.Fatal("suggestions exceeded their own bounded lane", event)
	}
	dispatchSuggestion(ctx, slots, Command{ID: "write", Method: "chat.send"}, func() bool { return true }, execute, send)
	if event := <-events; event.Type != "error" {
		t.Fatal("a write entered the prediction lane", event)
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("selection cancellation did not stop the prediction")
	}
	select {
	case event := <-events:
		t.Fatal("cancelled scope emitted a late result", event)
	case <-time.After(10 * time.Millisecond):
	}
}

func TestSuggestionRevocationCancelsGenerationAndSuppressesResult(t *testing.T) {
	var authorized atomic.Bool
	authorized.Store(true)
	started, stopped := make(chan struct{}), make(chan struct{})
	events := make(chan Event, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatchSuggestion(ctx, make(chan struct{}, 1), Command{ID: "revoked", Method: "chat.suggest"}, authorized.Load,
		func(ctx context.Context, command Command) (any, error) {
			close(started)
			<-ctx.Done()
			close(stopped)
			return desktop.Suggestion{Text: "must not escape"}, nil
		}, func(event Event) bool { events <- event; return true })
	<-started
	authorized.Store(false)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("revocation did not cancel prediction")
	}
	select {
	case event := <-events:
		t.Fatal("revoked authorization received a suggestion", event)
	case <-time.After(10 * time.Millisecond):
	}
}
