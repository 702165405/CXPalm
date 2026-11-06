package desktop

import (
	"encoding/json"
	"strings"

	"cxpalm/internal/store"
)

func steerAccepted(raw json.RawMessage) bool {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	accepted := false
	for depth := 0; depth < 5 && value != nil; depth++ {
		status, _ := value["status"].(string)
		if status == "rejected" || status == "outcome-unknown" || status == "error" || value["error"] != nil {
			return false
		}
		if status == "accepted" || status == "success" || value["applied"] == true {
			accepted = true
		}
		if turn, _ := value["turnId"].(string); turn != "" {
			accepted = true
		}
		value, _ = value["result"].(map[string]any)
	}
	return accepted && value == nil
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

func userItemText(item map[string]any) string {
	if text, _ := item["text"].(string); text != "" {
		return text
	}
	switch content := item["content"].(type) {
	case string:
		return content
	case []any:
		return inputText(content)
	}
	return ""
}

func steerAnchor(raw map[string]any, turnID, text string) *store.SteerAnchor {
	for _, entry := range orderedTurns(raw) {
		turn, _ := entry.(map[string]any)
		if fmtID(turn["turnId"]) != turnID && fmtID(turn["id"]) != turnID {
			continue
		}
		items := orderedItems(turn["items"])
		anchor := &store.SteerAnchor{Offset: len(items)}
		for _, entry := range items {
			item, _ := entry.(map[string]any)
			anchor.AfterItem = fmtID(item["id"])
			if item["type"] == "userMessage" && userItemText(item) == text {
				anchor.TextMatches++
			}
		}
		return anchor
	}
	return nil
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
	for turnIndex, entry := range turns {
		turn, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		turnID := fmtID(turn["turnId"])
		if turnID == "" {
			turnID = fmtID(turn["id"])
		}
		items := orderedItems(turn["items"])
		matching := []store.SteerMessage{}
		for _, message := range messages {
			if message.Turn == turnID {
				matching = append(matching, message)
			}
		}
		if len(matching) == 0 {
			continue
		}
		matched := map[string]int{}
		used := map[int]bool{}
		identities := map[string][]int{}
		textRows := map[string][]int{}
		itemPositions := map[string]int{}
		for itemIndex, entry := range items {
			item, _ := entry.(map[string]any)
			if id := fmtID(item["id"]); id != "" {
				itemPositions[id] = itemIndex
			}
			if item["type"] != "userMessage" {
				continue
			}
			for _, key := range []string{"id", "clientId", "clientUserMessageId", "serverClientUserMessageId"} {
				if id := fmtID(item[key]); id != "" {
					identities[id] = append(identities[id], itemIndex)
				}
			}
			textRows[userItemText(item)] = append(textRows[userItemText(item)], itemIndex)
		}
		for _, message := range matching {
			for _, identity := range []string{message.ID, "steer:" + message.ID} {
				for _, itemIndex := range identities[identity] {
					if !used[itemIndex] {
						matched[message.ID], used[itemIndex] = itemIndex, true
						break
					}
				}
				if _, found := matched[message.ID]; found {
					break
				}
			}
		}
		for _, message := range matching {
			if _, exists := matched[message.ID]; exists || message.Text == "" || len(message.Images) > 0 {
				continue
			}
			for occurrence, itemIndex := range textRows[message.Text] {
				earlier := message.Anchor != nil && occurrence < message.Anchor.TextMatches
				if !earlier && !used[itemIndex] {
					matched[message.ID], used[itemIndex] = itemIndex, true
					break
				}
			}
		}
		projected := append([]any{}, items...)
		insertions := map[int][]any{}
		for _, message := range matching {
			if itemIndex, exists := matched[message.ID]; exists {
				item, _ := items[itemIndex].(map[string]any)
				copyItem := map[string]any{}
				for key, value := range item {
					copyItem[key] = value
				}
				copyItem["id"] = "steer:" + message.ID
				projected[itemIndex] = copyItem
				continue
			}
			position := len(items)
			if message.Anchor != nil {
				position = max(0, min(message.Anchor.Offset, len(items)))
				if message.Anchor.AfterItem != "" {
					if itemIndex, found := itemPositions[message.Anchor.AfterItem]; found {
						position = itemIndex + 1
					}
				}
			}
			content := []any{}
			for _, path := range message.Images {
				content = append(content, map[string]any{"type": "localImage", "path": path})
			}
			insertions[position] = append(insertions[position], map[string]any{"id": "steer:" + message.ID, "type": "userMessage", "text": message.Text, "content": content, "delivery": "accepted"})
		}
		ordered := []any{}
		for position := 0; position <= len(projected); position++ {
			ordered = append(ordered, insertions[position]...)
			if position < len(projected) {
				ordered = append(ordered, projected[position])
			}
		}
		copyTurn := map[string]any{}
		for key, value := range turn {
			copyTurn[key] = value
		}
		copyTurn["items"] = ordered
		out[turnIndex] = copyTurn
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
	raw["readSyncAvailable"] = b.ReadSyncAvailable()
	raw["officialReadSyncAvailable"] = b.OfficialReadSyncAvailable()
	s.Snapshot = mustJSON(raw)
	return s
}
