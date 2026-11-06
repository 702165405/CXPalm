package server

import (
	"context"
	"time"

	"cxpalm/internal/desktop"
)

func dispatchSuggestion(scope context.Context, slots chan struct{}, command Command, authorized func() bool,
	execute func(context.Context, Command) (any, error), send func(Event) bool) {
	if command.Method != "chat.suggest" {
		send(Event{ID: command.ID, Type: "error", Error: "suggestion method not allowed"})
		return
	}
	empty := desktop.Suggestion{Thread: command.Thread, Turn: command.Turn, ReadToken: command.ReadToken}
	select {
	case slots <- struct{}{}:
	default:
		send(Event{ID: command.ID, Type: "result", Data: empty})
		return
	}
	go func() {
		defer func() { <-slots }()
		ctx, cancel := context.WithTimeout(scope, 48*time.Second)
		defer cancel()
		if !authorized() || ctx.Err() != nil {
			return
		}
		done := make(chan struct{})
		defer close(done)
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !authorized() {
						cancel()
						return
					}
				}
			}
		}()
		value, err := execute(ctx, command)
		if ctx.Err() != nil || !authorized() {
			return
		}
		if err != nil {
			value = empty
		}
		send(Event{ID: command.ID, Type: "result", Data: value})
	}()
}
