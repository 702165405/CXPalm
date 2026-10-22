package desktop

import (
	"cxpalm/internal/store"
	"errors"
)

const ReadSyncUnavailableReason = "网关已读存储暂不可用；聊天和会话提醒连接不受影响，请稍后重试。"

var ErrReadSyncUnavailable = errors.New(ReadSyncUnavailableReason)

type NotificationPreferenceState struct {
	store.NotificationPreference
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (b *Bridge) ReadSyncAvailable() bool {
	return b.ReadStore != nil
}

func (b *Bridge) OfficialReadSyncAvailable() bool {
	_, err := b.nativeReads()
	return err == nil
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
