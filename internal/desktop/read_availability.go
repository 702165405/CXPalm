package desktop

import (
	"cxpalm/internal/store"
	"errors"
)

const ReadSyncUnavailableReason = "当前桌面未提供可验证的已读同步身份，官方跨端已读暂不可用；聊天和会话提醒不受影响。身份恢复后会自动重新检查。"

var ErrReadSyncUnavailable = errors.New(ReadSyncUnavailableReason)

type NotificationPreferenceState struct {
	store.NotificationPreference
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (b *Bridge) ReadSyncAvailable() bool {
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
