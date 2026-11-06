package desktop

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSuggestionExchangeStartsOnlyASeparateEphemeralPrediction(t *testing.T) {
	source := "00000000-0000-0000-0000-000000000001"
	fork := "00000000-0000-0000-0000-000000000002"
	turn := "00000000-0000-0000-0000-000000000003"
	for _, mode := range []string{"ready", "same-identity", "unsafe-policy", "unsupported", "none", "oversize", "tool-call"} {
		t.Run(mode, func(t *testing.T) {
			var methods []string
			value, err := suggestionExchange(func(id int, method string, params any) (json.RawMessage, error) {
				methods = append(methods, method)
				fields := params.(map[string]any)
				switch method {
				case "initialize":
					return json.RawMessage(`{}`), nil
				case "config/read":
					return json.RawMessage(`{"layers":[{"config":{"mcp_servers":{"fixture":{"command":"must-not-run"}}}}]}`), nil
				case "thread/start":
					if fields["ephemeral"] != true || fields["sandbox"] != "read-only" || fields["approvalPolicy"] != "never" ||
						len(fields["dynamicTools"].([]any)) != 0 || len(fields["environments"].([]any)) != 0 {
						t.Fatal("prediction must be separate, nonpersistent and tool restricted", fields)
					}
					if fields["config"].(map[string]any)["mcp_servers.fixture.enabled"] != false {
						t.Fatal("configured MCP server not explicitly disabled")
					}
					identity := fork
					if mode == "same-identity" {
						identity = source
					}
					policy := "readOnly"
					if mode == "unsafe-policy" {
						policy = "dangerFullAccess"
					}
					return mustJSON(map[string]any{"thread": map[string]any{"id": identity}, "sandbox": map[string]any{"type": policy}, "approvalPolicy": "never"}), nil
				case "turn/start":
					if fields["threadId"] != fork || fields["outputSchema"] == nil {
						t.Fatal("prediction turn must use only the isolated identity and constrained output", fields)
					}
					if mode == "unsupported" {
						return nil, errors.New("not implemented")
					}
					return mustJSON(map[string]any{"turn": map[string]any{"id": turn}}), nil
				default:
					t.Fatal("suggestions must never resume or start a main turn", method)
					return nil, nil
				}
			}, func() error { return nil }, func(thread, predictionTurn string) (string, error) {
				if thread != fork || predictionTurn != turn {
					t.Fatal("wrong prediction result scope")
				}
				text := "Check the remaining tests."
				switch mode {
				case "none":
					text = ""
				case "oversize":
					text = strings.Repeat("x", 281)
				case "tool-call":
					return "", errors.New("interactive tool request aborted")
				}
				return string(mustJSON(map[string]string{"text": text})), nil
			}, source, "fixture", t.TempDir(), "[]")
			if mode == "ready" && (err != nil || value != "Check the remaining tests.") {
				t.Fatal(value, err)
			}
			if mode != "ready" && value != "" {
				t.Fatal("unsafe or unavailable prediction escaped", value)
			}
			if (mode == "same-identity" || mode == "unsafe-policy") && (err == nil || len(methods) != 3) {
				t.Fatal("a main identity or unsafe policy reached prediction", methods)
			}
		})
	}
}

func TestSuggestionsRequireAnExactFreshIdleCompletedReply(t *testing.T) {
	turn := "00000000-0000-0000-0000-000000000003"
	raw := map[string]any{"id": "fixture", "threadRuntimeStatus": map[string]any{"type": "idle"}, "requests": []any{},
		"turns": []any{map[string]any{"turnId": turn, "status": "completed", "items": []any{map[string]any{"type": "agentMessage", "text": "fixture"}}}}}
	state := State{Connected: true, Owner: "original", lastNative: time.Now(), Snapshot: mustJSON(raw)}
	token := readToken(raw)
	if !suggestionSource(state, turn, token) {
		t.Fatal("valid reply rejected")
	}
	for _, mode := range []string{"active", "approval", "failed", "stale", "disconnected", "archived", "owner", "token", "turn"} {
		next := state
		nextRaw := next.snapshotValue()
		nextToken, nextTurn := token, turn
		switch mode {
		case "active":
			nextRaw["threadRuntimeStatus"] = map[string]any{"type": "active"}
		case "approval":
			nextRaw["requests"] = []any{map[string]any{"id": "approval"}}
		case "failed":
			nextRaw["turns"].([]any)[0].(map[string]any)["status"] = "failed"
			nextToken = readToken(nextRaw)
		case "stale":
			next.lastNative = time.Now().Add(-time.Minute)
		case "disconnected":
			next.Connected = false
		case "archived":
			next.Archived = true
		case "owner":
			next.Owner = ""
		case "token":
			nextToken = strings.Repeat("0", 64)
		case "turn":
			nextTurn = "00000000-0000-0000-0000-000000000004"
		}
		next.Snapshot = mustJSON(nextRaw)
		if suggestionSource(next, nextTurn, nextToken) {
			t.Fatal("ineligible source accepted", mode)
		}
	}
}
