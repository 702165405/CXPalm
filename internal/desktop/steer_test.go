package desktop

import (
	"encoding/json"
	"testing"

	"cxpalm/internal/store"
)

func TestConfirmedSteerSurvivesReconnectAndNativeEchoDeduplicates(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	message := store.SteerMessage{ID: "steer-op", Turn: "active-turn", Text: "additional guidance"}
	if err = db.SaveSteer("chat", message); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	b := &Bridge{ReadStore: db}
	raw := map[string]any{"turns": []any{map[string]any{"turnId": "active-turn", "items": []any{map[string]any{"id": "initial", "type": "userMessage", "text": "original prompt"}}}}}
	out := b.withSteers("chat", raw)
	if len(orderedItems(orderedTurns(out)[0].(map[string]any)["items"])) != 2 {
		t.Fatal("accepted guidance missing")
	}
	if len(orderedItems(orderedTurns(raw)[0].(map[string]any)["items"])) != 1 {
		t.Fatal("native patch base mutated")
	}
	if len(orderedItems(orderedTurns(b.withSteers("chat", out))[0].(map[string]any)["items"])) != 2 {
		t.Fatal("duplicate overlay")
	}
	turn := orderedTurns(raw)[0].(map[string]any)
	turn["items"] = append(turn["items"].([]any), map[string]any{"id": "native-user", "type": "userMessage", "clientId": "steer-op", "content": []any{map[string]any{"type": "text", "text": message.Text}}})
	if len(orderedItems(orderedTurns(b.withSteers("chat", raw))[0].(map[string]any)["items"])) != 2 {
		t.Fatal("native echo duplicated")
	}
	if len(orderedItems(orderedTurns(b.withSteers("another", raw))[0].(map[string]any)["items"])) != 2 {
		t.Fatal("cross-chat overlay")
	}
	for _, raw := range []string{`{}`, `{"result":{"status":"rejected"}}`, `{"result":{"status":"outcome-unknown"}}`} {
		if steerAccepted(json.RawMessage(raw)) {
			t.Fatal("ambiguous guidance accepted", raw)
		}
	}
	if !steerAccepted(json.RawMessage(`{"result":{"turnId":"active-turn"}}`)) {
		t.Fatal("actual owner acknowledgement rejected")
	}
}
