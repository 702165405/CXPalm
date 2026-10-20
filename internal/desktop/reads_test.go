package desktop

import (
	"context"
	"crypto/sha256"
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func nativeReadFixture(t *testing.T, root string, ids ...string) (readContext, func(...string)) {
	t.Helper()
	identity := map[string]string{"kind": "chatgpt", "accountId": "account-test", "userId": "user-test"}
	sum := sha256.Sum256([]byte(`["chatgpt","account-test","user-test"]`))
	key := fmt.Sprintf("%x", sum)
	host := "local:fixture-host"
	claims := map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "account-test", "user_id": "user-test", "unrelated_bool": true}}
	payload, _ := json.Marshal(claims)
	auth := map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{"access_token": "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"}}
	raw, _ := json.Marshal(auth)
	if err := os.WriteFile(filepath.Join(root, "auth.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	write := func(ids ...string) {
		if ids == nil {
			ids = []string{}
		}
		raw, _ := json.Marshal(map[string]any{"electron-thread-read-state-v1": map[string]any{"version": 1, "unreadByIdentity": map[string]any{key: map[string]any{host: ids}}}})
		if err := os.WriteFile(filepath.Join(root, ".codex-global-state.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(ids...)
	return readContext{Identity: identity, HostKey: host}, write
}

func TestSharedReadReceiptExactReplyAndPersistence(t *testing.T) {
	root := t.TempDir()
	native, err := sql.Open("sqlite", filepath.Join(root, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	_, err = native.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,archived INTEGER);INSERT INTO threads VALUES('chat','Example','.',1,'vscode',0)`)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	b := NewBridge(config.Config{CodexHome: root})
	defer b.Close()
	b.ReadStore = db
	nativeContext, writeNative := nativeReadFixture(t, root, "chat")
	client, owner := net.Pipe()
	b.ipc = &IPC{conn: client, closed: make(chan struct{}), clientID: "cxpalm-test"}
	received := make(chan error, 1)
	go func() {
		defer owner.Close()
		var header [4]byte
		if _, e := io.ReadFull(owner, header[:]); e != nil {
			received <- e
			return
		}
		raw := make([]byte, binary.LittleEndian.Uint32(header[:]))
		if _, e := io.ReadFull(owner, raw); e != nil {
			received <- e
			return
		}
		var frame Frame
		json.Unmarshal(raw, &frame)
		var params struct {
			Host    string      `json:"hostId"`
			ID      string      `json:"conversationId"`
			Unread  bool        `json:"hasUnreadTurn"`
			Context readContext `json:"context"`
		}
		json.Unmarshal(frame.Params, &params)
		if frame.Method != "thread-read-state-changed" || frame.Version != 3 || params.Host != "local" || params.ID != "chat" || params.Unread || !sameReadContext(params.Context, nativeContext) {
			received <- fmt.Errorf("invalid native write")
			return
		}
		writeNative()
		received <- nil
	}()
	raw := json.RawMessage(`{"id":"chat","hasUnreadTurn":true,"threadRuntimeStatus":{"type":"idle"},"turns":[{"turnId":"t","status":"completed","items":[{"id":"a","type":"agentMessage","text":"first reply"}]}]}`)
	state := &State{ID: "chat", Owner: "official", Connected: true, Snapshot: raw}
	b.sessions["chat"] = state
	one, two := make(chan State, 4), make(chan State, 4)
	b.watchers["chat"] = map[chan State]bool{one: true, two: true}
	var canonical map[string]any
	json.Unmarshal(raw, &canonical)
	token := readToken(canonical)
	change := b.ReadChanges()
	accepted, err := b.MarkRead(context.Background(), "chat", token)
	if err != nil || !accepted {
		t.Fatal(accepted, err)
	}
	if err := <-received; err != nil {
		t.Fatal(err)
	}
	select {
	case <-change:
	default:
		t.Fatal("list subscribers were not woken")
	}
	for _, ch := range []chan State{one, two} {
		select {
		case s := <-ch:
			var p map[string]any
			json.Unmarshal(b.PhoneState(s).Snapshot, &p)
			if p["hasUnreadTurn"] != false {
				t.Fatal("other device did not observe read")
			}
		default:
			t.Fatal("chat subscriber not notified")
		}
	}
	if string(state.Snapshot) != string(raw) {
		t.Fatal("native patch base mutated")
	}
	page, err := b.Browse(context.Background(), "", false, "", 10, 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].Unread == nil || *page.Items[0].Unread {
		t.Fatal("list diverged", err)
	}
	if ok, err := b.MarkRead(context.Background(), "chat", token); err != nil || !ok || state.Sequence != 2 {
		t.Fatal("receipt is not idempotent", err)
	}
	db.Close()
	db, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	b.ReadStore = db
	stored, err := db.ReadToken("chat")
	if err != nil || stored != token {
		t.Fatal("read receipt lost on gateway restart", err)
	}
	writeNative("chat")
	state.Snapshot = json.RawMessage(strings.Replace(string(raw), "first reply", "new reply", 1))
	if ok, err := b.MarkRead(context.Background(), "chat", token); err != nil || ok {
		t.Fatal("stale receipt accepted", err)
	}
	var p map[string]any
	json.Unmarshal(b.PhoneState(*state).Snapshot, &p)
	if p["hasUnreadTurn"] != true {
		t.Fatal("new reply was incorrectly read")
	}
	state.Snapshot = raw
	c := &IPC{events: make(chan Frame, 1)}
	c.events <- Frame{Method: "thread-read-state-changed", Version: 3, Source: "official", Params: mustJSON(map[string]any{"hostId": "local", "conversationId": "chat", "hasUnreadTurn": true, "context": nativeContext})}
	close(c.events)
	b.events(c)
	stored, _ = db.ReadToken("chat")
	if stored != "" {
		t.Fatal("explicit official unread did not invalidate shared receipt")
	}
	state.Connected = false
	if accepted, err = b.MarkRead(context.Background(), "chat", token); err == nil && accepted {
		t.Fatal("offline read accepted")
	}
	state.Connected = true
	writeNative()
	state.Snapshot = json.RawMessage(strings.Replace(string(raw), `"idle"`, `"active"`, 1))
	if ok, err := b.MarkRead(context.Background(), "chat", token); err != nil || !ok {
		t.Fatal("visible reply cannot become read while turn is still active", err)
	}
	for _, bad := range []string{"", strings.Repeat("z", 64)} {
		if _, err = b.MarkRead(context.Background(), "chat", bad); err == nil {
			t.Fatal("invalid token accepted")
		}
	}
}

func TestOfficialReadOverlaySurvivesUnrelatedPatchesButNotNewReply(t *testing.T) {
	b := NewBridge(config.Config{})
	defer b.Close()
	raw := json.RawMessage(`{"id":"chat","hasUnreadTurn":false,"turns":[{"turnId":"turn","status":"completed","items":[{"type":"agentMessage","text":"reply"}]}]}`)
	var canonical map[string]any
	json.Unmarshal(raw, &canonical)
	unread := true
	state := State{ID: "chat", Connected: true, Snapshot: raw, Unread: &unread, UnreadToken: readToken(canonical)}
	// Settings/tool-only state changes must not erase an explicit owner unread event.
	state.Snapshot = json.RawMessage(strings.Replace(string(raw), `"id":"chat"`, `"id":"chat","latestModel":"new-model"`, 1))
	var projected map[string]any
	json.Unmarshal(PhoneState(state).Snapshot, &projected)
	if projected["hasUnreadTurn"] != true {
		t.Fatal("unrelated state update erased official unread")
	}
	state.Snapshot = json.RawMessage(strings.Replace(string(raw), `"reply"`, `"new reply"`, 1))
	json.Unmarshal(PhoneState(state).Snapshot, &projected)
	if projected["hasUnreadTurn"] != false {
		t.Fatal("old read overlay leaked to another reply")
	}
}

func TestExpiredBroadcastCannotOverridePersistedManualUnread(t *testing.T) {
	root := t.TempDir()
	_, write := nativeReadFixture(t, root, "chat")
	b := NewBridge(config.Config{CodexHome: root})
	defer b.Close()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	b.ReadStore = db
	raw := json.RawMessage(`{"id":"chat","hasUnreadTurn":false,"turns":[{"turnId":"t","items":[{"type":"agentMessage","text":"reply"}]}]}`)
	var m map[string]any
	json.Unmarshal(raw, &m)
	unread := false
	state := State{ID: "chat", Connected: true, Snapshot: raw, Unread: &unread, UnreadToken: readToken(m), UnreadUntil: time.Now().Add(-time.Second)}
	write("chat")
	json.Unmarshal(b.PhoneState(state).Snapshot, &m)
	if m["hasUnreadTurn"] != true {
		t.Fatal("old receipt hid persisted official manual unread")
	}
}

func TestUnconfirmedNativeReadDoesNotBecomeSharedRead(t *testing.T) {
	root := t.TempDir()
	nativeReadFixture(t, root, "chat")
	history, err := sql.Open("sqlite", filepath.Join(root, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = history.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,archived INTEGER);INSERT INTO threads VALUES('chat','Example','.',1,'vscode',0)`)
	history.Close()
	if err != nil {
		t.Fatal(err)
	}
	b := NewBridge(config.Config{CodexHome: root})
	defer b.Close()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	b.ReadStore = db
	raw := json.RawMessage(`{"id":"chat","hasUnreadTurn":true,"turns":[{"turnId":"t","items":[{"type":"agentMessage","text":"reply"}]}]}`)
	state := &State{ID: "chat", Owner: "owner", Connected: true, Snapshot: raw}
	b.sessions["chat"] = state
	var m map[string]any
	json.Unmarshal(raw, &m)
	client, owner := net.Pipe()
	defer owner.Close()
	b.ipc = &IPC{conn: client, closed: make(chan struct{}), clientID: "test"}
	// The desktop receives the frame but never accepts or persists it.
	go io.Copy(io.Discard, owner)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if accepted, err := b.MarkRead(ctx, "chat", readToken(m)); err == nil || accepted {
		t.Fatal("unconfirmed write acknowledged", err)
	}
	if receipt, _ := db.ReadToken("chat"); receipt != "" {
		t.Fatal("unconfirmed native result persisted as read")
	}
	json.Unmarshal(b.PhoneState(*state).Snapshot, &m)
	if m["hasUnreadTurn"] != true {
		t.Fatal("failed native confirmation hidden by local overlay")
	}
}
