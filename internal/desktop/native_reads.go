package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

type readContext struct {
	Identity map[string]string `json:"identity"`
	HostKey  string            `json:"executionHostKey"`
}

func validReadContext(c readContext) bool {
	if !strings.HasPrefix(c.HostKey, "local:") || len(c.HostKey) <= 6 || len(c.HostKey) > 4096 {
		return false
	}
	for _, value := range c.Identity {
		if value == "" || len(value) > 512 {
			return false
		}
	}
	switch c.Identity["kind"] {
	case "chatgpt":
		return len(c.Identity) == 3 && c.Identity["accountId"] != "" && c.Identity["userId"] != ""
	case "execution-storage":
		return len(c.Identity) == 2 && c.Identity["authMode"] != ""
	}
	return false
}

func sameReadContext(a, b readContext) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// Broadcasts supply only an outgoing context hint, never a read acknowledgement.
func (b *Bridge) learnReadContext(c *IPC, f Frame) {
	var p struct {
		Host    string      `json:"hostId"`
		ID      string      `json:"conversationId"`
		Unread  *bool       `json:"hasUnreadTurn"`
		Context readContext `json:"context"`
	}
	if f.Type != "broadcast" || f.Method != "thread-read-state-changed" || f.Version != 3 ||
		f.Source == "" || f.Source == c.clientID || json.Unmarshal(f.Params, &p) != nil ||
		p.Host != "local" || p.ID == "" || p.Unread == nil || !validReadContext(p.Context) {
		return
	}
	b.mu.Lock()
	if b.ipc != c {
		b.mu.Unlock()
		return
	}
	changed := b.readContextIPC != c || !sameReadContext(b.readContextHint, p.Context)
	b.readContextHint, b.readContextIPC, b.readContextSource = p.Context, c, f.Source
	if changed {
		b.readChangedLocked()
		for _, s := range b.sessions {
			b.emitLocked(s)
		}
	}
	b.mu.Unlock()
	b.refreshAfterReadBroadcast(c, p.ID)
}
func (b *Bridge) clearReadContextLocked() {
	b.readContextHint, b.readContextIPC, b.readContextSource = readContext{}, nil, ""
}

// Notifications belong to this installation, independent of model accounts.
func (b *Bridge) NotificationScope() (string, error) {
	home := filepath.Clean(b.config.CodexHome)
	if !filepath.IsAbs(home) {
		return "", errors.New("notification desktop directory must be absolute")
	}
	if runtime.GOOS == "windows" {
		home = strings.ToLower(home)
	}
	raw, _ := json.Marshal([]string{"cxpalm-notifications-desktop-v1", home})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
