package desktop

import (
	"context"
	"cxpalm/internal/config"
	"encoding/json"
	"fmt"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNativeTreeMatchesExistingPatchEngineAndPreservesPastViews(t *testing.T) {
	initial := map[string]any{"id": "thread", "a/b": map[string]any{"~x": []any{"A", nil, "C"}}, "nested": map[string]any{"items": []any{1.0, 2.0}}, "flag": true}
	raw := mustJSON(initial)
	patches := []string{
		`[{"op":"add","path":"/a~1b/~0x/1","value":"世界🙂"}]`,
		`[{"op":"replace","path":"/flag","value":null}]`,
		`[{"op":"remove","path":"/nested/items/0"},{"op":"add","path":"/nested/items/-","value":{"deep":true}}]`,
		`[{"op":"replace","path":"","value":{"id":"thread","x":[]}}]`,
	}
	random := rand.New(rand.NewSource(25))
	for i := 0; i < 300; i++ {
		index := random.Intn(3)
		patches = append(patches, fmt.Sprintf(`[{"op":"replace","path":["a/b","~x",%d],"value":%q}]`, index, fmt.Sprintf("value-%d-🙂", i)))
	}
	for _, patch := range patches {
		encoded, err := normalizePatches(json.RawMessage(patch))
		if err != nil {
			t.Fatal(err)
		}
		engine, err := jsonpatch.DecodePatch(encoded)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := engine.Apply(raw)
		if err != nil {
			t.Fatal(err)
		}
		var before map[string]any
		_ = json.Unmarshal(raw, &before)
		next, err := patchDocument(before, encoded)
		if err != nil {
			t.Fatal(err)
		}
		var want any
		_ = json.Unmarshal(expected, &want)
		if !reflect.DeepEqual(next, want) {
			t.Fatalf("patch mismatch: %s", patch)
		}
		if !reflect.DeepEqual(mustJSON(before), raw) {
			t.Fatal("past snapshot was mutated")
		}
	}
	for _, patch := range []string{`[{"op":"remove","path":"/missing"}]`, `[{"op":"replace","path":"/missing","value":1}]`, `[{"op":"add","path":"/a~2b","value":1}]`, `[{"op":"replace","path":"/a~1b/~0x/-1","value":1}]`, `[{"op":"remove","path":"/flag"},{"op":"remove","path":"/missing"}]`} {
		if _, err := patchDocument(initial, json.RawMessage(patch)); err == nil {
			t.Fatal("invalid patch accepted", patch)
		}
		if !reflect.DeepEqual(mustJSON(initial), raw) {
			t.Fatal("failed transaction mutated original")
		}
	}
}
func TestLargeNativePatchAndLeanProjectionProfile(t *testing.T) {
	raw := mustJSON(map[string]any{"id": "thread", "threadRuntimeStatus": map[string]any{"type": "active"}, "turns": []any{map[string]any{"turnId": "t", "items": []any{map[string]any{"id": "tool", "type": "commandExecution", "command": "fixture command", "aggregatedOutput": strings.Repeat("output\n", 900000)}, map[string]any{"id": "answer", "type": "agentMessage", "text": "Starting"}}}}})
	var tree map[string]any
	_ = json.Unmarshal(raw, &tree)
	patch := json.RawMessage(`[{"op":"replace","path":"/turns/0/items/1/text","value":"Starting with a new token"}]`)
	engine, _ := jsonpatch.DecodePatch(patch)
	start := time.Now()
	for i := 0; i < 10; i++ {
		if _, err := engine.Apply(raw); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Since(start)
	start = time.Now()
	for i := 0; i < 10; i++ {
		if _, err := patchDocument(tree, patch); err != nil {
			t.Fatal(err)
		}
	}
	next := time.Since(start)
	full := PhoneState(State{ID: "thread", Snapshot: raw})
	lean := projectState(State{ID: "thread", document: &nativeDocument{value: tree}}, true)
	if len(lean.Snapshot) > 10000 || len(full.Snapshot) < 5000000 {
		t.Fatalf("deferred output still transferred: %d -> %d", len(full.Snapshot), len(lean.Snapshot))
	}
	var p map[string]any
	_ = json.Unmarshal(lean.Snapshot, &p)
	item := p["turns"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)
	if item["deferred"] != true || item["text"] != "" {
		t.Fatal(item)
	}
	if readToken(tree) != lean.ReadToken {
		t.Fatal("lazy presentation changed read authority")
	}
	t.Logf("synthetic %d-byte native state, 10 small patches: previous=%.3fms COW=%.3fms; full phone=%d bytes lean=%d bytes", len(raw), float64(old)/float64(time.Millisecond), float64(next)/float64(time.Millisecond), len(full.Snapshot), len(lean.Snapshot))
}

func TestOfficialSnapshotAndPatchKeepOwnerReadAndNotificationState(t *testing.T) {
	b := NewBridge(config.Config{})
	defer b.Close()
	client, owner := net.Pipe()
	defer owner.Close()
	c := &IPC{conn: client, closed: make(chan struct{}), events: make(chan Frame, 8)}
	b.ipc = c
	b.sessions["thread"] = &State{ID: "thread"}
	watch := make(chan State, 4)
	b.watchers["thread"] = map[chan State]bool{watch: true}
	done := make(chan struct{})
	go func() { b.events(c); close(done) }()
	send := func(change map[string]any) State {
		c.events <- Frame{Method: "thread-stream-state-changed", Version: 11, Source: "official-owner", Params: mustJSON(map[string]any{"hostId": "local", "conversationId": "thread", "change": change})}
		select {
		case state := <-watch:
			return state
		case <-time.After(time.Second):
			t.Fatal("native state stalled")
			return State{}
		}
	}
	raw := map[string]any{"id": "thread", "threadRuntimeStatus": map[string]any{"type": "active"}, "turns": []any{map[string]any{"turnId": "t", "items": []any{map[string]any{"id": "tool", "type": "commandExecution", "text": strings.Repeat("large output", 10000)}, map[string]any{"id": "a", "type": "agentMessage", "text": "Before"}}}}}
	first := send(map[string]any{"type": "snapshot", "revision": 1, "conversationState": raw})
	next := send(map[string]any{"type": "patches", "baseRevision": 1, "revision": 2, "patches": []any{map[string]any{"op": "replace", "path": []any{"threadRuntimeStatus", "type"}, "value": "idle"}, map[string]any{"op": "replace", "path": []any{"turns", 0, "items", 1, "text"}, "value": "After"}}})
	if !next.Connected || next.Owner != "official-owner" || next.document == nil || len(next.Snapshot) != 0 || next.lastNative.IsZero() {
		t.Fatal("patch did not retain lazy native owner state")
	}
	if first.snapshotValue()["threadRuntimeStatus"].(map[string]any)["type"] != "active" {
		t.Fatal("past snapshot modified")
	}
	projected := b.StreamState(next)
	if !strings.Contains(string(projected.Snapshot), "After") || strings.Contains(string(projected.Snapshot), "large output") {
		t.Fatal("lean projection lost reply or leaked collapsed output")
	}
	notification := NotificationState(next)
	if notification.ReadToken != projected.ReadToken || !strings.Contains(string(notification.Snapshot), "idle") {
		t.Fatal("notification/read identity diverged")
	}
	legacy, err := b.Snapshot("thread")
	if err != nil || !strings.Contains(string(legacy.Snapshot), "After") {
		t.Fatal("legacy readback stale", err)
	}
	bad := send(map[string]any{"type": "patches", "baseRevision": 999, "revision": 3, "patches": []any{map[string]any{"op": "replace", "path": []any{"id"}, "value": "other"}}})
	if bad.Connected || bad.Owner != "" || bad.snapshotValue()["id"] != "thread" {
		t.Fatal("invalid lineage changed native authority")
	}
	close(c.events)
	<-done
}

func TestHistoryFileRevisionCacheAndFullDetailRead(t *testing.T) {
	home := t.TempDir()
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	path := filepath.Join(home, "rollout.jsonl")
	line := `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"text":"Original history"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	row := Session{ID: "history", RolloutPath: path}
	first, err := b.readHistoryValue(context.Background(), row, true)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := b.readHistoryValue(context.Background(), row, true)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.ValueOf(first["turns"]).Pointer() != reflect.ValueOf(cached["turns"]).Pointer() {
		t.Fatal("unchanged history reparsed")
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(line, "Original history", "Updated full history")), 0600); err != nil {
		t.Fatal(err)
	}
	fresh, err := b.readHistoryValue(context.Background(), row, true)
	if err != nil || !strings.Contains(string(mustJSON(fresh)), "Updated full history") {
		t.Fatal("changed file returned stale cache", err)
	}
	owner, raw, _ := controlFixture(t)
	defer owner.Close()
	text := strings.Repeat("complete output\n", 10000)
	raw["turns"] = []any{map[string]any{"turnId": "t", "items": []any{map[string]any{"id": "tool", "type": "commandExecution", "command": "fixture command", "text": text}}}}
	owner.sessions["thread"] = &State{ID: "thread", document: &nativeDocument{value: raw}}
	detail, err := owner.Item(context.Background(), "thread", "t", "tool")
	if err != nil {
		t.Fatal(err)
	}
	if detail.(map[string]any)["text"] != text || detail.(map[string]any)["bodyRevision"] != bodyRevision(text, "fixture command") {
		t.Fatal("deferred output truncated or version mismatch")
	}
	if _, err := owner.Item(context.Background(), "not-in-catalog", "t", "tool"); err == nil {
		t.Fatal("detail read bypassed catalog")
	}
}
