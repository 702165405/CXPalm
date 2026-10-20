package desktop

import (
	"context"
	"errors"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

func validThreadName(name string) bool {
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 120 {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (b *Bridge) Rename(ctx context.Context, id, name string) (any, error) {
	name = strings.TrimSpace(name)
	if !threadID.MatchString(id) || !validThreadName(name) {
		return nil, errors.New("invalid thread ID or name (1-120 characters, no control characters)")
	}
	b.mu.Lock()
	lock := b.actions[id]
	if lock == nil {
		lock = &sync.Mutex{}
		b.actions[id] = lock
	}
	b.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()
	row, err := b.History.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.Title != name {
		if _, err = metadataCall(ctx, b.config, "thread/name/set", map[string]any{"threadId": id, "name": name}); err != nil {
			return nil, err
		}
	}
	// Acknowledgement alone is not confirmation: read the official persisted title.
	row, err = b.History.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.Title != name {
		return nil, errors.New("rename was not confirmed; inspect the desktop before retrying")
	}
	return map[string]any{"id": id, "title": row.Title}, nil
}
