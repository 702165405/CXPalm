package desktop

import (
	"context"
	"cxpalm/internal/attachments"
	"encoding/json"
	"errors"
	"time"
)

// Queues live in the gateway, not in a browser. Never replace the desktop's own
// queue wholesale: its follower set-state method has no atomic append operation.
func (b *Bridge) StartQueues() {
	if b.ReadStore == nil {
		return
	}
	go func() {
		timer := time.NewTicker(time.Second)
		defer timer.Stop()
		for {
			ids, err := b.ReadStore.QueueThreads()
			if err == nil {
				for _, id := range ids {
					b.queueWorker(id)
				}
			}
			select {
			case <-b.done:
				return
			case <-timer.C:
			}
		}
	}()
}
func (b *Bridge) Queue(ctx context.Context, device, id, operation, text string, skills []string) (any, error) {
	return b.QueueAttachments(ctx, device, id, operation, text, skills, nil)
}
func (b *Bridge) QueueAttachments(ctx context.Context, device, id, operation, text string, skills, attachmentIDs []string) (any, error) {
	if len(attachmentIDs) > 0 {
		if b.Uploads == nil {
			return nil, errors.New("uploads unavailable")
		}
		if _, err := b.Uploads.Resolve(device, id, attachmentIDs); err != nil {
			return nil, err
		}
	}
	row, err := b.History.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.Archived {
		return nil, errors.New("请先取消归档")
	}
	if _, err = b.nativeSnapshot(id); err != nil {
		return nil, err
	}
	if len(skills) > 0 {
		catalog, err := b.Catalog(row.CWD)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, id := range skills {
			found := false
			for _, skill := range catalog.Skills {
				if skill.ID == id {
					found = true
					break
				}
			}
			if !found || seen[id] {
				return nil, errors.New("Skill 已不可用或重复，请刷新目录")
			}
			seen[id] = true
		}
	}
	key, err := b.ReadStore.EnqueueFiles(device, operation, id, text, skills, attachmentIDs)
	if err != nil {
		return nil, err
	}
	b.queueChanged(id)
	b.queueWorker(id)
	return map[string]any{"queued": true, "id": key}, nil
}
func (b *Bridge) CancelQueue(ctx context.Context, id, message string) (any, error) {
	if _, err := b.History.Get(ctx, id); err != nil {
		return nil, err
	}
	if err := b.ReadStore.CancelQueued(id, message); err != nil {
		return nil, err
	}
	b.queueChanged(id)
	return map[string]any{"removed": true}, nil
}
func (b *Bridge) queueChanged(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s := b.sessions[id]; s != nil {
		b.emitLocked(s)
	}
}
func (b *Bridge) projectQueue(s State) State {
	if b.ReadStore == nil || len(s.Snapshot) == 0 {
		return s
	}
	rows, err := b.ReadStore.Queue(s.ID)
	var raw map[string]any
	if json.Unmarshal(s.Snapshot, &raw) != nil {
		return s
	}
	raw["queuedMessages"] = rows
	raw["queueAvailable"] = err == nil
	s.Snapshot = mustJSON(raw)
	return s
}
func (b *Bridge) queueWorker(id string) {
	b.queueMu.Lock()
	defer b.queueMu.Unlock()
	if b.queueWorkers == nil {
		b.queueWorkers = map[string]bool{}
	}
	if b.queueWorkers[id] {
		return
	}
	b.queueWorkers[id] = true
	go func() {
		defer func() { b.queueMu.Lock(); delete(b.queueWorkers, id); b.queueMu.Unlock() }()
		ctx, cancel := context.WithTimeout(b.ctx, 8*time.Second)
		states, remove, err := b.Subscribe(ctx, id)
		cancel()
		if err != nil {
			return
		}
		defer remove()
		timer := time.NewTicker(time.Second)
		defer timer.Stop()
		for {
			if b.queueStep(id) {
				return
			}
			select {
			case <-b.done:
				return
			case <-states:
			case <-timer.C:
			}
		}
	}()
}

// The head stays until the desktop snapshot contains its client message ID.
// An old idle snapshot therefore cannot dispatch the next message prematurely.
func (b *Bridge) queueStep(id string) bool {
	if b.ctx.Err() != nil {
		return true
	}
	rows, err := b.ReadStore.Queue(id)
	if err != nil {
		return false
	}
	if len(rows) == 0 {
		return true
	}
	q := rows[0]
	s, err := b.nativeSnapshot(id)
	if err != nil {
		return false
	}
	raw := s.snapshotValue()
	if raw == nil {
		return false
	}
	if q.Status == "sent" {
		if queuedObserved(raw, q.ID) {
			if b.ReadStore.ObserveQueued(q.ID) == nil {
				b.queueChanged(id)
			}
		}
		return false
	}
	if q.Status != "queued" {
		return false
	}
	status, _ := raw["threadRuntimeStatus"].(map[string]any)
	if status["type"] != "idle" {
		return false
	}
	claimed, err := b.ReadStore.ClaimQueued(q.ID)
	if err != nil || !claimed {
		return false
	}
	b.queueChanged(id)
	finish := func(status, message string) { _ = b.ReadStore.FinishQueued(q.ID, status, message); b.queueChanged(id) }
	if !b.ReadStore.QueueAuthorized(q.Device) {
		finish("unknown", "排队设备授权已失效或 IP 已不在白名单内，消息未发送")
		return false
	}
	ctx, cancel := context.WithTimeout(b.ctx, 50*time.Second)
	defer cancel()
	files := []attachments.File{}
	if len(q.Attachments) > 0 {
		if b.Uploads == nil {
			finish("unknown", "附件不可用，未发送")
			return false
		}
		files, err = b.Uploads.Resolve(q.Device, id, q.Attachments)
		if err != nil {
			finish("unknown", "附件不可用，未发送")
			return false
		}
	}
	result, err := b.ControlActionFiles(ctx, id, q.ID, "chat.send", q.Text, "", nil, Options{}, q.Skills, files)
	if err != nil {
		finish("unknown", err.Error())
		return false
	}
	encoded, err := json.Marshal(result)
	if err != nil || !queueAccepted(encoded) {
		finish("unknown", "桌面未明确确认发送，请核对；不会自动重发")
		return false
	}
	finish("sent", "")
	return false
}
func queuedObserved(raw map[string]any, id string) bool {
	for _, entry := range orderedTurns(raw) {
		turn, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if params, ok := turn["params"].(map[string]any); ok && params["clientUserMessageId"] == id {
			return true
		}
		for _, item := range orderedItems(turn["items"]) {
			if item, ok := item.(map[string]any); ok && (item["clientId"] == id || item["serverClientUserMessageId"] == id) {
				return true
			}
		}
	}
	return false
}
func queueAccepted(encoded []byte) bool {
	var value any
	if json.Unmarshal(encoded, &value) != nil {
		return false
	}
	// IPC success alone is not a turn acceptance; inspect the owner's nested result.
	var accepted func(any) bool
	accepted = func(v any) bool {
		m, ok := v.(map[string]any)
		if !ok {
			return false
		}
		if turn, ok := m["turn"].(map[string]any); ok && fmtID(turn["id"]) != "" {
			return true
		}
		if m["status"] == "accepted" && fmtID(m["turnId"]) != "" {
			return true
		}
		return accepted(m["result"])
	}
	return accepted(value)
}
