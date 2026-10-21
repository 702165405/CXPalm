package desktop

import (
	"encoding/json"
	"strings"
)

func steerAccepted(raw json.RawMessage) bool {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	for depth := 0; depth < 5 && value != nil; depth++ {
		status, _ := value["status"].(string)
		if status == "rejected" || status == "outcome-unknown" || status == "error" || value["error"] != nil {
			return false
		}
		if status == "accepted" || status == "success" || value["applied"] == true {
			return true
		}
		if turn, _ := value["turnId"].(string); turn != "" {
			return true
		}
		value, _ = value["result"].(map[string]any)
	}
	return false
}
func inputText(input []any) string {
	parts := []string{}
	for _, part := range input {
		if p, ok := part.(map[string]any); ok {
			if text, ok := p["text"].(string); ok {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// Some owners acknowledge steering before adding a user item to their stream.
// Overlay only durable, explicitly accepted receipts; never alter native patch bases.
func (b *Bridge) withSteers(id string, raw map[string]any) map[string]any {
	if b.ReadStore == nil || raw == nil {
		return raw
	}
	messages, err := b.ReadStore.SteerMessages(id)
	if err != nil || len(messages) == 0 {
		return raw
	}
	turns := orderedTurns(raw)
	out := make([]any, len(turns))
	copy(out, turns)
	for i, entry := range turns {
		turn, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		turnID := fmtID(turn["turnId"])
		if turnID == "" {
			turnID = fmtID(turn["id"])
		}
		items := orderedItems(turn["items"])
		seenIDs := map[string]bool{}
		nativeText := map[string]int{}
		for _, entry := range items {
			if item, ok := entry.(map[string]any); ok && item["type"] == "userMessage" {
				for _, key := range []string{"id", "clientId", "serverClientUserMessageId"} {
					if v, ok := item[key].(string); ok {
						seenIDs[v] = true
					}
				}
				text, _ := item["text"].(string)
				if text == "" {
					if content, ok := item["content"].([]any); ok {
						text = inputText(content)
					}
				}
				nativeText[text]++
			}
		}
		extra := []any{}
		for _, message := range messages {
			if message.Turn != turnID {
				continue
			}
			if seenIDs[message.ID] {
				if nativeText[message.Text] > 0 {
					nativeText[message.Text]--
				}
				continue
			}
			if nativeText[message.Text] > 0 {
				nativeText[message.Text]--
				continue
			}
			content := []any{}
			for _, path := range message.Images {
				content = append(content, map[string]any{"type": "localImage", "path": path})
			}
			extra = append(extra, map[string]any{"id": "steer:" + message.ID, "type": "userMessage", "text": message.Text, "content": content, "delivery": "accepted"})
		}
		if len(extra) > 0 {
			copyTurn := map[string]any{}
			for key, value := range turn {
				copyTurn[key] = value
			}
			copyTurn["items"] = append(append([]any{}, items...), extra...)
			out[i] = copyTurn
		}
	}
	copyRaw := map[string]any{}
	for key, value := range raw {
		copyRaw[key] = value
	}
	delete(copyRaw, "turnHistory")
	copyRaw["turns"] = out
	return copyRaw
}
func (b *Bridge) NotifyChanged(id string) { b.queueChanged(id) }

func (b *Bridge) projectNotifications(s State) State {
	if b.ReadStore == nil || len(s.Snapshot) == 0 {
		return s
	}
	pref, err := b.NotificationPreference(s.ID)
	if err != nil {
		return s
	}
	var raw map[string]any
	if json.Unmarshal(s.Snapshot, &raw) != nil {
		return s
	}
	raw["notificationPreference"] = pref
	raw["readSyncAvailable"] = pref.Available
	s.Snapshot = mustJSON(raw)
	return s
}
