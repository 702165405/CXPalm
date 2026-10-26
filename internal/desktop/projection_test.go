package desktop

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPhoneProjectionDoesNotMutatePatchBase(t *testing.T) {
	raw := json.RawMessage(`{"id":"thread","turns":[{"turnId":"turn","status":"completed","items":[{"id":"item","type":"agentMessage","text":"` + strings.Repeat("a", 12000) + `"}]}]}`)
	s := State{ID: "thread", Connected: true, Snapshot: raw}
	out := PhoneState(s)
	if !strings.Contains(string(out.Snapshot), strings.Repeat("a", 12000)) || strings.Contains(string(out.Snapshot), `"truncated":true`) {
		t.Fatal("message text was cut")
	}
	if string(s.Snapshot) != string(raw) {
		t.Fatal("patch base modified")
	}
}
func TestCanonicalHistoryTakesPrecedenceOverEmptyTurns(t *testing.T) {
	raw := json.RawMessage(`{"id":"thread","turns":[],"turnHistory":{"kind":"canonical","history":{"entitiesByKey":{"t":{"turnId":"t","items":[{"id":"i","type":"agentMessage","text":"hello"}]}},"islands":[{"entries":[{"value":"t"}]}]}}}`)
	out := PhoneState(State{ID: "thread", Snapshot: raw})
	if !strings.Contains(string(out.Snapshot), "hello") {
		t.Fatal("canonical history hidden behind empty legacy turns")
	}
}

func TestReplyReadTokenMatchesListAndDoesNotChangeWithSettingsOrTools(t *testing.T) {
	raw := json.RawMessage(`{"id":"thread","hasUnreadTurn":true,"latestModel":"first","turns":[{"turnId":"t","status":"completed","items":[{"id":"a","type":"agentMessage","text":"reply"},{"id":"tool","type":"commandExecution","aggregatedOutput":"one"}]}]}`)
	s := State{ID: "thread", Connected: true, Snapshot: raw}
	first := PhoneState(s).ReadToken
	row := Session{}
	enrichSession(&row, s)
	if len(first) != 64 || row.ReadToken != first {
		t.Fatal("list and displayed reply differ", first, row.ReadToken)
	}
	changed := strings.ReplaceAll(string(raw), `"first"`, `"second"`)
	changed = strings.ReplaceAll(changed, `"one"`, `"two"`)
	changed = strings.ReplaceAll(changed, `"hasUnreadTurn":true`, `"hasUnreadTurn":false`)
	if PhoneState(State{ID: s.ID, Snapshot: json.RawMessage(changed)}).ReadToken != first {
		t.Fatal("non-reply change invalidated viewing receipt")
	}
	for _, next := range []string{strings.ReplaceAll(changed, `"reply"`, `"new reply"`), strings.ReplaceAll(changed, `"t"`, `"next-turn"`), strings.ReplaceAll(changed, `"completed"`, `"inProgress"`)} {
		if PhoneState(State{ID: s.ID, Snapshot: json.RawMessage(next)}).ReadToken == first {
			t.Fatal("new reply version reused old read receipt")
		}
	}
	if s.ReadToken != "" || string(s.Snapshot) != string(raw) {
		t.Fatal("canonical state mutated")
	}
	for _, missing := range []string{`{}`, `{"turns":[{"items":[{"type":"userMessage","text":"hello"}]}]}`, `{"turns":[{"items":[{"type":"agentMessage","text":"old"}]},{"items":[]}]}`} {
		if PhoneState(State{Snapshot: json.RawMessage(missing)}).ReadToken != "" {
			t.Fatal("empty latest reply acknowledged")
		}
	}
}

func TestProjectionPreservesOfficialTurnTiming(t *testing.T) {
	raw := json.RawMessage(`{"turns":[{"turnId":"t","status":"completed","turnStartedAtMs":1792156051000,"durationMs":764351,"items":[]}]}`)
	out := PhoneState(State{Snapshot: raw})
	var snapshot map[string]any
	if err := json.Unmarshal(out.Snapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	turn := snapshot["turns"].([]any)[0].(map[string]any)
	if turn["durationMs"] != float64(764351) || turn["turnStartedAtMs"] != float64(1792156051000) || snapshot["observedAtMs"].(float64) <= 0 {
		t.Fatal("official timing lost")
	}
	page, err := historyPage(map[string]any{"turns": []any{turn}}, "", 0, 20, "desktop")
	if err != nil || page.Turns[0].(map[string]any)["durationMs"] != float64(764351) {
		t.Fatal("history timing lost", err)
	}
}

func TestProcessProjectionPreservesCommandsAndVisibleReasoningOnly(t *testing.T) {
	raw := json.RawMessage(`{"turns":[{"turnId":"t","items":[
		{"id":"cmd","type":"commandExecution","command":"git status\n--short","aggregatedOutput":"output","status":"inProgress"},
		{"id":"empty","type":"commandExecution","command":"pwd","aggregatedOutput":""},
		{"id":"r","type":"reasoning","summary":["Visible",{"text":"summary"}],"rawContent":["PRIVATE_RAW"],"encrypted_content":"PRIVATE_ENCRYPTED"},
		{"id":"hidden","type":"reasoning","encrypted_content":"PRIVATE_ENCRYPTED"}
	]}]}`)
	out := PhoneState(State{Snapshot: raw})
	var projected map[string]any
	if err := json.Unmarshal(out.Snapshot, &projected); err != nil {
		t.Fatal(err)
	}
	items := projected["turns"].([]any)[0].(map[string]any)["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("expected only visible items: %s", out.Snapshot)
	}
	command := items[0].(map[string]any)
	if command["command"] != "git status\n--short" || command["text"] != "output" || command["status"] != "inProgress" {
		t.Fatal(command)
	}
	if items[1].(map[string]any)["text"] != "pwd" {
		t.Fatal("empty output dropped command")
	}
	if items[2].(map[string]any)["text"] != "Visible\nsummary" {
		t.Fatal("visible reasoning lost")
	}
	if strings.Contains(string(out.Snapshot), "PRIVATE_") {
		t.Fatal("non-display reasoning was forwarded")
	}
}
