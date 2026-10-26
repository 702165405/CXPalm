package server

import (
	"context"
	"errors"

	"cxpalm/internal/desktop"
)

// Waiting for official read confirmation must not block ordered chat commands.
// Only this metadata operation may use the bounded background lane.
func dispatchRead(ctx context.Context, slots chan struct{}, q Command,
	execute func(context.Context, Command) (any, error), send func(Event) bool) {
	if q.Method != "chat.read" {
		send(Event{ID: q.ID, Type: "error", Error: "background method not allowed"})
		return
	}
	select {
	case slots <- struct{}{}:
	default:
		send(Event{ID: q.ID, Type: "result", Data: map[string]any{"accepted": false, "available": false,
			"thread": q.Thread, "readToken": q.ReadToken, "reason": desktop.ReadSyncUnavailableReason}})
		return
	}
	go func() {
		defer func() { <-slots }()
		if ctx.Err() != nil {
			return
		}
		value, err := execute(ctx, q)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			if errors.Is(err, desktop.ErrReadSyncUnavailable) {
				send(Event{ID: q.ID, Type: "result", Data: map[string]any{"accepted": false, "available": false,
					"thread": q.Thread, "readToken": q.ReadToken, "reason": desktop.ReadSyncUnavailableReason}})
			} else {
				send(Event{ID: q.ID, Type: "error", Error: err.Error()})
			}
			return
		}
		send(Event{ID: q.ID, Type: "result", Data: value})
	}()
}
