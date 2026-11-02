package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cxpalm/internal/config"
)

func TestErrorPresentationSurvivesLiveLeanAndHistoryWithoutMutatingNativeState(t *testing.T) {
	for _, lazy := range []bool{false, true} {
		raw := map[string]any{"turns": []any{
			map[string]any{"turnId": "failed", "status": "failed", "error": map[string]any{"message": "Request failed: 429 <img src=x> **plain**", "additionalDetails": "PRIVATE_BODY", "stack": "PRIVATE_STACK"}, "items": []any{}},
			map[string]any{"turnId": "tool", "status": "failed", "items": []any{map[string]any{"id": "tool", "type": "mcpToolCall", "error": map[string]any{"message": "Tool unavailable"}}}},
			map[string]any{"turnId": "fallback", "status": "failed", "items": []any{}},
			map[string]any{"turnId": "ok", "status": "completed", "error": nil, "items": []any{}},
		}}
		before := mustJSON(raw)
		view := projectState(State{ID: "fixture", Snapshot: before}, lazy)
		if strings.Contains(string(view.Snapshot), "PRIVATE") || string(mustJSON(raw)) != string(before) {
			t.Fatal("error leaked private fields or changed native base")
		}
		var value map[string]any
		json.Unmarshal(view.Snapshot, &value)
		turns := value["turns"].([]any)
		for i, want := range []string{"Request failed: 429 <img src=x> **plain**", "Tool unavailable", "本轮执行失败，官方桌面未提供详细原因。"} {
			items := turns[i].(map[string]any)["items"].([]any)
			if len(items) != 1 {
				t.Fatalf("error missing/duplicated: %s", view.Snapshot)
			}
			item := items[0].(map[string]any)
			if item["type"] != "sessionError" || item["text"] != want || item["deferred"] == true {
				t.Fatal(item)
			}
		}
		if len(turns[3].(map[string]any)["items"].([]any)) != 0 {
			t.Fatal("healthy chat showed error")
		}
		page, err := historyPageProjection(raw, "latest", 0, 20, "desktop", false, lazy)
		if err != nil || len(page.Turns) != 4 {
			t.Fatal(err)
		}
		again := projectState(State{Snapshot: view.Snapshot}, lazy)
		var second map[string]any
		json.Unmarshal(again.Snapshot, &second)
		for i, entry := range second["turns"].([]any) {
			items := entry.(map[string]any)["items"].([]any)
			original := turns[i].(map[string]any)["items"].([]any)
			if len(items) != len(original) {
				t.Fatal("projection duplicates errors")
			}
			for j, item := range items {
				if item.(map[string]any)["text"] != original[j].(map[string]any)["text"] {
					t.Fatal("projected error changed")
				}
			}
		}
	}
}

func TestErrorMessagesRedactSecretsAndRemainBounded(t *testing.T) {
	secret := "fixture-sensitive-value"
	message := "Bad request; Authorization: Bearer " + secret + "; api_key='" + secret + "' https://example.test/private?token=" + secret + "\x00"
	out := displayError(map[string]any{"message": message, "additionalDetails": secret})
	if strings.Contains(out, secret) || strings.Contains(out, "https://") || strings.ContainsRune(out, 0) {
		t.Fatal("secret not redacted")
	}
	if !strings.Contains(out, "Bad request") {
		t.Fatal("diagnostic message lost")
	}
	if len([]rune(displayError(strings.Repeat("错", 8000)))) > 4050 {
		t.Fatal("error not bounded")
	}
	for _, v := range []any{nil, 42, map[string]any{"stack": "private"}, map[string]any{"message": map[string]any{"token": secret}}} {
		if displayError(v) != "" {
			t.Fatal("non-display payload forwarded")
		}
	}
}

func TestRolloutErrorEventsSurviveHistory(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "rollout.jsonl")
	lines := []string{
		`{"type":"event_msg","payload":{"type":"task_started","turn_id":"a"}}`,
		`{"type":"event_msg","payload":{"type":"error","turn_id":"foreign","message":"WRONG_TURN"}}`,
		`{"type":"event_msg","payload":{"type":"error","turn_id":"a","message":"Stream disconnected; retry exhausted"}}`,
		`{"type":"event_msg","payload":{"type":"turn_aborted","turn_id":"a"}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	for _, complete := range []bool{false, true} {
		raw, err := b.readHistory(context.Background(), Session{ID: "fixture", RolloutPath: path}, complete)
		if err != nil {
			t.Fatal(err)
		}
		view := PhoneState(State{Snapshot: raw})
		if strings.Contains(string(view.Snapshot), "WRONG_TURN") || !strings.Contains(string(view.Snapshot), "Stream disconnected") || strings.Count(string(view.Snapshot), `"type":"sessionError"`) != 1 {
			t.Fatalf("rollout error lost or duplicated: %s", view.Snapshot)
		}
	}
}
