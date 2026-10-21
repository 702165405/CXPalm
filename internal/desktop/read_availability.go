package desktop

import (
	"cxpalm/internal/store"
	"errors"
)

const ReadSyncUnavailableReason = "当前桌面未提供可验证的账号和主机身份，跨端已读和会话提醒暂不可用；正常聊天不受影响。身份恢复后会自动重新检查。"

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
		return NotificationPreferenceState{Reason: ReadSyncUnavailableReason}, nil
	}
	scope, err := b.NotificationScope()
	if err != nil {
		// Do not select a historical account or overwrite its saved preferences.
		return NotificationPreferenceState{Reason: ReadSyncUnavailableReason}, nil
	}
	pref, err := b.ReadStore.NotificationPreference(scope, id)
	return NotificationPreferenceState{NotificationPreference: pref, Available: err == nil}, err
}
