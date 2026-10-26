package desktop

import (
	"context"
	"errors"
	"time"
)

type SessionSync struct {
	Total    int    `json:"total"`
	Checked  int    `json:"checked"`
	Live     int    `json:"live"`
	Scanning bool   `json:"scanning"`
	Error    string `json:"error,omitempty"`
}

// One gateway-wide passive sweep, independent of list filters/pagination.
// Background follows never invoke OpenSelectedChat or create execution owners.
func (b *Bridge) StartSessionSync() {
	b.listSyncOnce.Do(func() {
		go func() {
			watches := map[string]func(){}
			defer func() {
				for _, remove := range watches {
					remove()
				}
			}()
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				ctx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
				b.syncSessionWatches(ctx, watches)
				cancel()
				select {
				case <-b.done:
					return
				case <-ticker.C:
				}
			}
		}()
	})
}

func (b *Bridge) syncSessionWatches(ctx context.Context, watches map[string]func()) {
	rows := []Session{}
	for offset := 0; ; offset += 500 {
		page, err := b.History.ListPage(ctx, "", false, 500, offset)
		if err != nil {
			b.mu.Lock()
			b.listProgress.Scanning = false
			b.listProgress.Error = "会话列表同步暂不可用，正在重试"
			b.mu.Unlock()
			return
		}
		rows = append(rows, page...)
		if len(page) < 500 {
			break
		}
	}
	b.mu.Lock()
	b.listProgress = SessionSync{Total: len(rows), Scanning: true}
	b.mu.Unlock()
	present := map[string]bool{}
	for _, row := range rows {
		if ctx.Err() != nil {
			break
		}
		present[row.ID] = true
		if watches[row.ID] == nil {
			states, remove, err := b.Subscribe(ctx, row.ID)
			if err != nil {
				if !errors.Is(err, errSessionNotFound) {
					b.mu.Lock()
					b.listProgress.Error = "部分会话订阅失败，正在重试"
					b.mu.Unlock()
				}
				continue
			}
			stop, done := context.WithCancel(b.ctx)
			watches[row.ID] = func() { done(); remove() }
			go func() {
				for {
					select {
					case <-stop.Done():
						return
					case <-states:
					}
				}
			}()
		}
		b.mu.Lock()
		b.listProgress.Checked++
		b.mu.Unlock()
	}
	if ctx.Err() == nil {
		for id, remove := range watches {
			if !present[id] {
				remove()
				delete(watches, id)
			}
		}
		b.mu.Lock()
		b.listMembers = present
		stale := []string{}
		for id, state := range b.sessions {
			if !present[id] && !state.Archived && !state.removed {
				stale = append(stale, id)
			}
		}
		b.mu.Unlock()
		for _, id := range stale {
			row, err := b.History.Get(ctx, id)
			if err != nil && !errors.Is(err, errSessionNotFound) {
				b.mu.Lock()
				b.listProgress.Error = "部分会话状态核对失败，正在重试"
				b.mu.Unlock()
				continue
			}
			if err == nil && !row.Archived {
				continue
			}
			b.mu.Lock()
			if state := b.sessions[id]; state != nil {
				state.Archived = err == nil && row.Archived
				state.removed = errors.Is(err, errSessionNotFound)
				state.Connected = false
				state.Owner = ""
				state.Revision = nil
				if state.Archived {
					state.Error = "会话已在官方桌面归档"
				} else {
					state.Error = "会话已从官方列表移除"
				}
				b.emitLocked(state)
			}
			b.mu.Unlock()
		}
	}
	b.mu.Lock()
	b.listProgress.Scanning = false
	if ctx.Err() != nil {
		b.listProgress.Error = "会话列表同步尚未完成，正在重试"
	}
	b.mu.Unlock()
}
