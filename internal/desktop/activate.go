package desktop

import (
	"context"
	"time"
)

// Only explicit client selection may open the original desktop; background
// notification/queue subscriptions remain read-only followers.
func (b *Bridge) OpenSelectedChat(ctx context.Context, id string) {
	b.openSelectedChat(ctx, id, 2*time.Second, openOfficialChat)
}

func (b *Bridge) openSelectedChat(ctx context.Context, id string, delay time.Duration, open func(context.Context, string) error) {
	if ctx.Err() != nil || b.ctx.Err() != nil || !threadID.MatchString(id) {
		return
	}
	b.mu.Lock()
	s := b.sessions[id]
	if s == nil || s.Archived || s.Connected || len(b.watchers[id]) == 0 || b.opening[id] {
		b.mu.Unlock()
		return
	}
	b.opening[id] = true
	b.mu.Unlock()
	defer func() { b.mu.Lock(); delete(b.opening, id); b.mu.Unlock() }()
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-b.done:
		return
	case <-timer.C:
	}
	b.mu.Lock()
	s = b.sessions[id]
	needed := s != nil && !s.Archived && !s.Connected && len(b.watchers[id]) > 0
	b.mu.Unlock()
	if !needed || ctx.Err() != nil || b.ctx.Err() != nil {
		return
	}
	row, err := b.History.Get(ctx, id)
	if err != nil || row.Archived {
		return
	}
	b.mu.Lock()
	s = b.sessions[id]
	if s == nil || s.Connected || time.Since(s.lastOpen) < time.Minute {
		b.mu.Unlock()
		return
	}
	s.lastOpen = time.Now()
	b.mu.Unlock()
	call, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	stop := context.AfterFunc(b.ctx, cancel)
	defer stop()
	if err = open(call, id); err != nil {
		if ctx.Err() != nil || b.ctx.Err() != nil {
			return
		}
		b.mu.Lock()
		if s = b.sessions[id]; s != nil && !s.Connected {
			s.Error = "无法在官方 Codex 桌面打开此会话，请在电脑打开后重试。"
			b.emitLocked(s)
		}
		b.mu.Unlock()
		return
	}
	// Only a subsequent official snapshot grants live state or write authority.
	b.followAsync(id)
}
