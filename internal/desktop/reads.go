package desktop

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// Local receipts never override official state. A write is sent once and must
// be confirmed by a fresh original-owner snapshot, not a reflected broadcast.
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
	s, c := b.sessions[id], b.ipc
	valid := func() bool {
		return b.sessions[id] == s && s != nil && s.Connected && s.Owner != "" && !s.Archived && !s.removed && readToken(s.snapshotValue()) == token
	}
	if !valid() {
		b.mu.Unlock()
		return false, nil
	}
	unread, known := s.snapshotValue()["hasUnreadTurn"].(bool)
	if !known {
		b.mu.Unlock()
		return false, ErrReadSyncUnavailable
	}
	if !unread {
		b.mu.Unlock()
		return true, nil
	}
	if !b.readContextAvailableLocked() {
		b.mu.Unlock()
		return false, ErrReadSyncUnavailable
	}
	if b.readAttempts[id] == token {
		b.mu.Unlock()
		return false, ErrReadSyncUnavailable
	}
	if b.readAttempts == nil {
		b.readAttempts = map[string]string{}
	}
	b.readAttempts[id] = token
	native, owner := b.readContextHint, s.Owner
	b.mu.Unlock()
	params := mustJSON(map[string]any{"hostId": "local", "conversationId": id, "hasUnreadTurn": false, "context": native})
	writeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	if err := c.send(writeCtx, Frame{Type: "broadcast", Method: "thread-read-state-changed", Version: 3, Params: params}); err != nil {
		return false, errors.Join(ErrReadSyncUnavailable, err)
	}
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		b.mu.Lock()
		current := valid() && s.Owner == owner && b.ipc == c && b.readContextAvailableLocked() && sameReadContext(native, b.readContextHint)
		confirmed := false
		if current {
			value, ok := s.snapshotValue()["hasUnreadTurn"].(bool)
			confirmed = ok && !value
		}
		b.mu.Unlock()
		if !current {
			return false, errors.Join(ErrReadSyncUnavailable, errors.New("desktop reply, owner or read context changed; read not confirmed"))
		}
		if confirmed {
			return true, nil
		}
		select {
		case <-writeCtx.Done():
			return false, errors.Join(ErrReadSyncUnavailable, errors.New("desktop did not confirm read state; inspect desktop before retrying"))
		case <-c.Done():
			return false, errors.Join(ErrReadSyncUnavailable, errors.New("desktop disconnected; read not confirmed"))
		case <-tick.C:
		}
	}
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
	var raw map[string]any
	if json.Unmarshal(p.Snapshot, &raw) == nil {
		_, known := raw["hasUnreadTurn"].(bool)
		b.mu.Lock()
		blocked := b.readAttempts[s.ID] == p.ReadToken && raw["hasUnreadTurn"] == true
		b.mu.Unlock()
		available := known && s.Connected && !blocked && b.ReadSyncAvailable()
		raw["readSyncAvailable"], raw["officialReadSyncAvailable"] = available, available
		p.Snapshot = mustJSON(raw)
	}
	return p
}
