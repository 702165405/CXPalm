package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func commandPreview(command string) string {
	end := len(command)
	count := 0
	for i := range command {
		if count == 200 {
			end = i
			break
		}
		count++
	}
	return strings.Join(strings.Fields(command[:end]), " ")
}
func bodyRevision(text, command string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d:", len(text))
	h.Write([]byte(text))
	h.Write([]byte(command))
	return hex.EncodeToString(h.Sum(nil))
}

// Detail and image reads share the same rooted, read-only message lookup.
func (b *Bridge) Item(ctx context.Context, id, turnID, itemID string) (any, error) {
	_, item, err := b.sourceItem(ctx, id, turnID, itemID)
	if err != nil {
		return nil, err
	}
	full := PhoneState(State{ID: id, Snapshot: mustJSON(map[string]any{"turns": []any{map[string]any{"turnId": turnID, "items": []any{item}}}})})
	var projected struct {
		Turns []struct {
			Items []map[string]any `json:"items"`
		} `json:"turns"`
	}
	if json.Unmarshal(full.Snapshot, &projected) != nil || len(projected.Turns) == 0 || len(projected.Turns[0].Items) == 0 {
		return nil, errors.New("item has no displayable content")
	}
	result := projected.Turns[0].Items[0]
	result["id"], result["turn"] = itemID, turnID
	text, _ := result["text"].(string)
	command, _ := result["command"].(string)
	result["bodyRevision"] = bodyRevision(text, command)
	return result, nil
}
