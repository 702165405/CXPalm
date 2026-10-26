package desktop

import (
	"context"
	"time"
)

// Read broadcasts do not update presentation. They request a fresh snapshot
// from the existing owner instead of waiting for the 15-second maintenance sweep.
func (b *Bridge) refreshAfterReadBroadcast(c *IPC, id string) {
	b.mu.Lock()
	s := b.sessions[id]
	if b.ipc != c || s == nil || !s.Connected || s.Owner == "" || s.Archived || s.removed || b.readRefreshes[id] {
		b.mu.Unlock()
		return
	}
	if b.readRefreshes == nil {
		b.readRefreshes = map[string]bool{}
	}
	b.readRefreshes[id] = true
	b.mu.Unlock()
	go func() {
		defer func() { b.mu.Lock(); delete(b.readRefreshes, id); b.mu.Unlock() }()
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-b.done:
			return
		case <-c.Done():
			return
		case <-timer.C:
		}
		b.mu.Lock()
		s := b.sessions[id]
		owner := ""
		if b.ipc == c && s != nil && s.Connected && !s.Archived && !s.removed {
			owner = s.Owner
		}
		b.mu.Unlock()
		if owner != "" {
			ctx, cancel := context.WithTimeout(b.ctx, time.Second)
			defer cancel()
			_ = c.Follow(ctx, id, owner)
		}
	}()
}
