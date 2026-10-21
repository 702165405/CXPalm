package desktop

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type readContext struct {
	Identity map[string]string `json:"identity"`
	HostKey  string            `json:"executionHostKey"`
}
type nativeReads struct {
	Context readContext
	Unread  map[string]bool
}
type nativeReadCache struct {
	sync.Mutex
	auth, state os.FileInfo
	value       *nativeReads
}

// Decode only identity claims; bearer and refresh tokens never leave this function.
// The desktop validates this context again before accepting the IPC broadcast.
func (b *Bridge) nativeReads() (*nativeReads, error) {
	value, err := b.loadNativeReads()
	if err != nil {
		return nil, errors.Join(ErrReadSyncUnavailable, err)
	}
	return value, nil
}

func (b *Bridge) loadNativeReads() (*nativeReads, error) {
	c := &b.nativeReadCache
	c.Lock()
	defer c.Unlock()
	authPath := filepath.Join(b.config.CodexHome, "auth.json")
	statePath := filepath.Join(b.config.CodexHome, ".codex-global-state.json")
	a, err := os.Stat(authPath)
	if err != nil {
		return nil, errors.New("desktop read identity unavailable")
	}
	s, err := os.Stat(statePath)
	if err != nil {
		return nil, errors.New("desktop read state unavailable")
	}
	same := func(x, y os.FileInfo) bool {
		return x != nil && y != nil && x.ModTime() == y.ModTime() && x.Size() == y.Size()
	}
	if c.value != nil && same(c.auth, a) && same(c.state, s) {
		return c.value, nil
	}
	raw, err := readBounded(authPath, 1<<20)
	if err != nil {
		return nil, errors.New("desktop read identity unavailable")
	}
	var auth struct {
		Mode   string `json:"auth_mode"`
		Tokens struct {
			Access string `json:"access_token"`
		} `json:"tokens"`
	}
	if json.Unmarshal(raw, &auth) != nil {
		return nil, errors.New("invalid desktop read identity")
	}
	identity := map[string]string{}
	var parts []string
	if auth.Mode == "chatgpt" || auth.Mode == "chatgptAuthTokens" {
		token := strings.Split(auth.Tokens.Access, ".")
		if len(token) != 3 {
			return nil, errors.New("desktop read identity unavailable")
		}
		payload, err := base64.RawURLEncoding.DecodeString(token[1])
		if err != nil {
			return nil, errors.New("invalid desktop read identity")
		}
		var claims struct {
			Auth struct {
				Account       string `json:"chatgpt_account_id"`
				LegacyAccount string `json:"account_id"`
				User          string `json:"user_id"`
				LegacyUser    string `json:"chatgpt_user_id"`
			} `json:"https://api.openai.com/auth"`
		}
		if json.Unmarshal(payload, &claims) != nil {
			return nil, errors.New("invalid desktop read identity")
		}
		account, user := claims.Auth.Account, claims.Auth.User
		if account == "" {
			account = claims.Auth.LegacyAccount
		}
		if user == "" {
			user = claims.Auth.LegacyUser
		}
		if account == "" || user == "" {
			return nil, errors.New("desktop read identity unavailable")
		}
		identity = map[string]string{"kind": "chatgpt", "accountId": account, "userId": user}
		parts = []string{"chatgpt", account, user}
	} else {
		return nil, errors.New("desktop read synchronization requires a supported signed-in identity")
	}
	data, _ := json.Marshal(parts)
	sum := sha256.Sum256(data)
	key := hex.EncodeToString(sum[:])
	raw, err = readBounded(statePath, 16<<20)
	if err != nil {
		return nil, errors.New("desktop read state unavailable")
	}
	var state struct {
		Reads struct {
			Version    int                            `json:"version"`
			ByIdentity map[string]map[string][]string `json:"unreadByIdentity"`
		} `json:"electron-thread-read-state-v1"`
	}
	if json.Unmarshal(raw, &state) != nil || state.Reads.Version != 1 {
		return nil, errors.New("unsupported desktop read state")
	}
	hosts := state.Reads.ByIdentity[key]
	// Only a host key actually published by the current desktop is used. No guessed
	// host configuration, credential writes, or direct edits to official state files.
	host := ""
	for candidate := range hosts {
		if strings.HasPrefix(candidate, "local:") {
			if host != "" {
				return nil, errors.New("ambiguous desktop read host")
			}
			host = candidate
		}
	}
	if host == "" {
		return nil, errors.New("desktop read host unavailable")
	}
	out := &nativeReads{Context: readContext{Identity: identity, HostKey: host}, Unread: map[string]bool{}}
	for _, id := range hosts[host] {
		out.Unread[id] = true
	}
	c.auth, c.state, c.value = a, s, out
	return out, nil
}
func sameReadContext(a, b readContext) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// Notification journals are isolated by the official signed-in account and execution host.
func (b *Bridge) NotificationScope() (string, error) {
	native, err := b.nativeReads()
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal([]any{b.config.CodexHome, native.Context})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
