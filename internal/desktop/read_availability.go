package desktop

import (
	"cxpalm/internal/store"
	"errors"
)

const ReadSyncUnavailableReason = "桌面已读同步尚未就绪；以桌面状态为准，聊天和会话提醒不受影响。"

var ErrReadSyncUnavailable = errors.New(ReadSyncUnavailableReason)

type NotificationPreferenceState struct {
	store.NotificationPreference
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (b *Bridge) ReadSyncAvailable() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.readContextAvailableLocked()
}

func (b *Bridge) OfficialReadSyncAvailable() bool {
	return b.ReadSyncAvailable()
}

func (b *Bridge) readContextAvailableLocked() bool {
	c := b.ipc
	if c == nil || b.readContextIPC != c || !validReadContext(b.readContextHint) {
		return false
	}
	select {
	case <-c.Done():
		return false
	default:
		return true
	}
}

func (b *Bridge) NotificationPreference(id string) (NotificationPreferenceState, error) {
	if b.ReadStore == nil {
		return NotificationPreferenceState{Reason: "提醒存储暂不可用"}, nil
	}
	scope, err := b.NotificationScope()
	if err != nil {
		return NotificationPreferenceState{}, err
	}
	pref, err := b.ReadStore.NotificationPreference(scope, id)
	return NotificationPreferenceState{NotificationPreference: pref, Available: err == nil}, err
}
