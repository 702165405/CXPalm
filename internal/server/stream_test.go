package server

import (
	"cxpalm/internal/desktop"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLatestStateNeverBacklogsAndRejectsOldSubscription(t *testing.T) {
	var slot latestState
	slot.reset("new")
	for i := uint64(1); i <= 10000; i++ {
		slot.put(streamDelivery{subscription: "new", state: desktop.State{Sequence: i}})
	}
	slot.put(streamDelivery{subscription: "old", state: desktop.State{Sequence: 999999}})
	if value := slot.take(); value == nil || value.state.Sequence != 10000 {
		t.Fatal("latest state lost")
	}
	if slot.take() != nil {
		t.Fatal("obsolete snapshots queued")
	}
	slot.put(streamDelivery{subscription: "new"})
	slot.reset("next")
	if slot.take() != nil {
		t.Fatal("old subscription survived reset")
	}
}
func TestStreamContractAndAppendPayload(t *testing.T) {
	var encoder streamEncoder
	turns := []any{map[string]any{"turnId": "t", "items": []any{map[string]any{"id": "q", "type": "userMessage", "text": strings.Repeat("question ", 800)}, map[string]any{"id": "a", "type": "agentMessage", "text": strings.Repeat("prefix ", 80)}}}}
	snapshot := map[string]any{"turns": turns, "threadRuntimeStatus": map[string]any{"type": "active"}, "requests": []any{}}
	var frames []any
	for i := 0; i < 5; i++ {
		state := desktop.State{ID: "thread", Connected: true, Sequence: uint64(i + 1), Snapshot: mustMarshal(snapshot)}
		event := encoder.next(state, "fixture-stream")
		expected := map[string]any{}
		_ = json.Unmarshal(mustMarshal(state), &expected)
		expected = mapWithStream(expected, "fixture-stream", uint64(i+1))
		frames = append(frames, map[string]any{"event": event, "expected": expected})
		if i == 1 {
			if event.Type != "chat.delta" || len(mustMarshal(event)) > 1000 {
				t.Fatal("small update retransmitted body")
			}
		}
		// Changes include Unicode suffixes, explicit null, array additions/shrinking and missing fields.
		if i == 0 {
			a := turns[0].(map[string]any)["items"].([]any)[1].(map[string]any)
			a["text"] = a["text"].(string) + "世界🙂"
			a["deferred"] = true
			a["bodyRevision"] = "transient"
		}
		if i == 1 {
			a := turns[0].(map[string]any)["items"].([]any)[1].(map[string]any)
			delete(a, "deferred")
			delete(a, "bodyRevision")
			snapshot["requests"] = []any{map[string]any{"id": "approval", "params": nil}}
			turns = append(turns, map[string]any{"turnId": "next", "items": []any{map[string]any{"id": "q2", "type": "userMessage", "text": "Next"}}})
			snapshot["turns"] = turns
		}
		if i == 2 {
			snapshot["turns"] = turns[1:]
			delete(snapshot, "requests")
		}
		if i == 3 {
			snapshot["threadRuntimeStatus"] = nil
		}
	}
	if path := os.Getenv("CXPALM_STREAM_FIXTURE"); path != "" {
		raw, _ := json.MarshalIndent(frames, "", "  ")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func mustMarshal(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
