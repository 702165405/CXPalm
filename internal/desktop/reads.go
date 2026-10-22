package desktop

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// A receipt confirms reading in CXPalm, not an official desktop mutation. Keep
// it durable before optional owner synchronization so missing auth or a timeout
// cannot leave the same visible reply unread on every receiver.
func (b *Bridge) MarkRead(ctx context.Context, id, token string) (bool, error) {
	if len(token) != 64 {
		return false, errors.New("invalid read token")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return false, errors.New("invalid read token")
	}
	if _, err := b.History.Get(ctx, id); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	b.mu.Lock()
	s := b.sessions[id]
	if s == nil || !s.Connected || s.Owner == "" || s.Archived || s.removed || readToken(s.snapshotValue()) != token {
		b.mu.Unlock()
		return false, nil
	}
	if b.ReadStore == nil {
		b.mu.Unlock()
		return false, ErrReadSyncUnavailable
	}
	err := b.ReadStore.SetReadToken(id, token)
	if err == nil {
		s.Unread, s.UnreadToken = nil, ""
		b.readChangedLocked()
		b.emitLocked(s)
	}
	b.mu.Unlock()
	if err != nil {
		return false, err
	}
	// Best effort only; never manufacture an official identity or replay this
	// write later. The helper rechecks reply, owner connection and native context.
	_, _ = b.syncOfficialRead(ctx, id, token)
	return true, nil
}

// Official read synchronization retains its own identity and confirmation checks.
func (b *Bridge) syncOfficialRead(ctx context.Context, id, token string) (bool, error) {
	if len(token) != 64 {
		return false, errors.New("invalid read token")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return false, errors.New("invalid read token")
	}
	if _, err := b.History.Get(ctx, id); err != nil {
		return false, err
	}
	native, err := b.nativeReads()
	if err != nil {
		return false, err
	}
	b.mu.Lock()
	s := b.sessions[id]
	c := b.ipc
	valid := func() bool {
		if s == nil || !s.Connected || s.Owner == "" || b.ReadStore == nil {
			return false
		}
		raw := s.snapshotValue()
		if raw == nil || readToken(raw) != token {
			return false
		}
		return true
	}
	if !valid() {
		b.mu.Unlock()
		return false, nil
	}
	b.mu.Unlock()
	if native.Unread[id] {
		if c == nil {
			return false, errors.New("desktop disconnected")
		}
		params := mustJSON(map[string]any{"hostId": "local", "conversationId": id, "hasUnreadTurn": false, "context": native.Context})
		if err = c.send(ctx, Frame{Type: "broadcast", Method: "thread-read-state-changed", Version: 3, Params: params}); err != nil {
			return false, err
		}
		timeout := time.NewTimer(4 * time.Second)
		defer timeout.Stop()
		tick := time.NewTicker(25 * time.Millisecond)
		defer tick.Stop()
	confirm:
		for {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-timeout.C:
				return false, errors.New("desktop did not confirm read state; not marked as read")
			case <-tick.C:
				current, e := b.nativeReads()
				if e != nil {
					return false, e
				}
				if !sameReadContext(native.Context, current.Context) {
					return false, errors.New("desktop account or host changed")
				}
				if !current.Unread[id] {
					break confirm
				}
			}
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// A newer reply must remain unread even if the previous receipt finished late.
	if b.sessions[id] != s || !valid() {
		return false, nil
	}
	return true, nil
}
func (b *Bridge) ReadChanges() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.readChanges
}
func (b *Bridge) readChangedLocked()        { close(b.readChanges); b.readChanges = make(chan struct{}) }
func (b *Bridge) PhoneState(s State) State  { return b.phoneState(s, false) }
func (b *Bridge) StreamState(s State) State { return b.phoneState(s, true) }
func (b *Bridge) phoneState(s State, lazy bool) State {
	view := s
	if raw := s.snapshotValue(); raw != nil {
		view.document = &nativeDocument{value: b.withSteers(s.ID, raw)}
	}
	p := b.projectNotifications(b.projectQueue(b.projectGoal(projectState(view, lazy))))
	if b.ReadStore == nil || !s.Connected || p.ReadToken == "" {
		return p
	}
	var raw map[string]any
	if json.Unmarshal(p.Snapshot, &raw) != nil {
		return p
	}
	stored, err := b.ReadStore.ReadToken(s.ID)
	if err != nil {
		raw["readSyncAvailable"] = false
	} else {
		raw["hasUnreadTurn"] = stored != p.ReadToken
	}
	p.Snapshot = mustJSON(raw)
	return p
}
