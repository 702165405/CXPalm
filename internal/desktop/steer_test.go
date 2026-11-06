package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"cxpalm/internal/store"
)

func TestSteerPositionAndIdentityRemainStableAcrossOutputAndNativeEcho(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bridge := &Bridge{ReadStore: db}
	items := []any{
		map[string]any{"id": "original", "type": "userMessage", "text": "continue"},
		map[string]any{"id": "before", "type": "agentMessage", "text": "earlier output"},
	}
	raw := map[string]any{"turns": []any{map[string]any{"turnId": "turn", "status": "inProgress", "items": items}}}
	anchor := steerAnchor(raw, "turn", "continue")
	for _, id := range []string{"one", "two"} {
		if err := db.SaveSteer("chat", store.SteerMessage{ID: id, Turn: "turn", Text: "continue", Anchor: anchor}); err != nil {
			t.Fatal(err)
		}
	}
	messages, _ := db.SteerMessages("chat")
	if messages[0].Anchor.AfterItem != "before" || messages[0].Anchor.TextMatches != 1 {
		t.Fatal("durable anchor lost", messages)
	}
	turn := orderedTurns(raw)[0].(map[string]any)
	turn["items"] = append(items, map[string]any{"id": "after", "type": "agentMessage", "text": "later output"})
	ids := func(view map[string]any) []string {
		var ids []string
		for _, entry := range orderedItems(orderedTurns(view)[0].(map[string]any)["items"]) {
			ids = append(ids, fmtID(entry.(map[string]any)["id"]))
		}
		return ids
	}
	expected := []string{"original", "before", "steer:one", "steer:two", "after"}
	if actual := ids(bridge.withSteers("chat", raw)); !reflect.DeepEqual(actual, expected) {
		t.Fatal("guidance moved past later output or matched the original identical prompt", actual)
	}
	turn["items"] = []any{items[0], items[1],
		map[string]any{"id": "native-two", "type": "userMessage", "clientUserMessageId": "two", "text": "continue"},
		map[string]any{"id": "after", "type": "agentMessage", "text": "later output"}}
	if actual := ids(bridge.withSteers("chat", raw)); !reflect.DeepEqual(actual, expected) {
		t.Fatal("text fallback consumed another guidance operation's explicit echo", actual)
	}
	turn["items"] = []any{items[0], items[1], map[string]any{"id": "native-one", "type": "userMessage", "text": "continue"},
		map[string]any{"id": "native-two", "type": "userMessage", "clientUserMessageId": "two", "text": "continue"},
		map[string]any{"id": "after", "type": "agentMessage", "text": "later output"}}
	view := bridge.withSteers("chat", raw)
	if actual := ids(view); !reflect.DeepEqual(actual, expected) {
		t.Fatal("native echo changed the identity/order or consumed another operation", actual)
	}
	if actual := ids(bridge.withSteers("chat", view)); !reflect.DeepEqual(actual, expected) {
		t.Fatal("projection not idempotent", actual)
	}
	if ids(raw)[2] != "native-one" {
		t.Fatal("native patch base was changed")
	}
	page, err := historyPage(view, "latest", 0, 20, "desktop")
	if err != nil || len(page.Questions) != 3 || page.Questions[1].Item != "steer:one" || page.Questions[2].Item != "steer:two" {
		t.Fatal("same-turn questions lost individual message identity", page.Questions, err)
	}
}

func TestSteerRejectsMissingOrChangedClientTurnBeforeDispatch(t *testing.T) {
	for _, expected := range []string{"", "earlier-turn"} {
		t.Run(expected, func(t *testing.T) {
			bridge, _, _ := queueFixture(t)
			bridge.sessions["thread"].Snapshot = json.RawMessage(`{"threadRuntimeStatus":{"type":"active"},"turns":[{"turnId":"current-turn","status":"inProgress","items":[]}]}`)
			_, err := bridge.ControlAction(context.Background(), "thread", "operation", "chat.steer", "guidance", "", nil, Options{ExpectedTurn: expected}, nil)
			var rejected *Rejected
			if !errors.As(err, &rejected) || rejected.Code != "steer_turn_changed" {
				t.Fatal("stale or unbound guidance was not rejected before IPC", err)
			}
			bridge.ipc.mu.Lock()
			pending := len(bridge.ipc.pending)
			bridge.ipc.mu.Unlock()
			if pending != 0 {
				t.Fatal("rejected guidance reached IPC")
			}
		})
	}
}

func TestActiveTurnIgnoresUnreferencedPaginatedEntities(t *testing.T) {
	history := map[string]any{
		"entitiesByKey": map[string]any{
			"stale": map[string]any{"turnId": "stale-turn", "status": "inProgress"},
			"live":  map[string]any{"turnId": "live-turn", "status": "inProgress"},
		},
		"islands": []any{map[string]any{"entries": []any{map[string]any{"value": "live"}}}},
	}
	raw := map[string]any{"turnHistory": map[string]any{"history": history}}
	for attempt := 0; attempt < 100; attempt++ {
		if activeTurn(raw) != "live-turn" {
			t.Fatal("active turn depended on unordered map iteration")
		}
	}
	history["entitiesByKey"].(map[string]any)["live"].(map[string]any)["status"] = "completed"
	if activeTurn(raw) != "" {
		t.Fatal("unreferenced entity revived after the visible turn ended")
	}
	history["entitiesByKey"].(map[string]any)["live"].(map[string]any)["status"] = "inProgress"
	delete(history, "islands")
	if activeTurn(raw) != "" {
		t.Fatal("ambiguous unordered active turns were guessed")
	}
	if activeTurn(map[string]any{"turns": []any{map[string]any{"id": "flat", "status": "inProgress"}}}) != "flat" {
		t.Fatal("flat official turn ID variant unsupported")
	}
}

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
	for _, raw := range []string{`{}`, `{"result":{"status":"rejected"}}`, `{"result":{"status":"outcome-unknown"}}`, `{"status":"success","result":{"status":"rejected"}}`} {
		if steerAccepted(json.RawMessage(raw)) {
			t.Fatal("ambiguous guidance accepted", raw)
		}
	}
	if !steerAccepted(json.RawMessage(`{"result":{"turnId":"active-turn"}}`)) {
		t.Fatal("actual owner acknowledgement rejected")
	}
}
