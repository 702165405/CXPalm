package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Question struct {
	Turn   string `json:"turn"`
	Offset int    `json:"offset"`
	Text   string `json:"text"`
}

type HistoryPage struct {
	Turns     []any      `json:"turns"`
	Questions []Question `json:"questions"`
	Offset    int        `json:"offset"`
	Total     int        `json:"total"`
	HasMore   bool       `json:"hasMore"`
	Source    string     `json:"source"`
}

// History reads are never execution snapshots and never replace native patch bases.
func (b *Bridge) HistoryPage(ctx context.Context, id, cursor string, offset, limit int) (HistoryPage, error) {
	return b.historyPage(ctx, id, cursor, offset, limit, false, false)
}
func (b *Bridge) HistoryIndex(ctx context.Context, id string, limit int) (HistoryPage, error) {
	return b.historyPage(ctx, id, "latest", 0, limit, true, false)
}
func (b *Bridge) HistoryPageLean(ctx context.Context, id, cursor string, offset, limit int) (HistoryPage, error) {
	return b.historyPage(ctx, id, cursor, offset, limit, false, true)
}
func (b *Bridge) historyPage(ctx context.Context, id, cursor string, offset, limit int, indexOnly, lazy bool) (HistoryPage, error) {
	if offset < 0 || limit < 0 || limit > 20 {
		return HistoryPage{}, errors.New("invalid history page")
	}
	if limit == 0 {
		limit = 20
	}
	row, err := b.History.Get(ctx, id)
	if err != nil {
		return HistoryPage{}, err
	}
	b.mu.Lock()
	var state State
	if s := b.sessions[id]; s != nil {
		state = *s
	}
	b.mu.Unlock()
	raw := state.snapshotValue()
	source := "desktop"
	if raw == nil || raw["historySource"] == "rollout" {
		raw, err = b.readHistoryValue(ctx, row, true)
		if err != nil {
			return HistoryPage{}, fmt.Errorf("history unavailable: %w", err)
		}
		if raw == nil {
			return historyPageProjection(map[string]any{"turns": []any{}}, cursor, offset, limit, "rollout", indexOnly, lazy)
		}
		source = "rollout"
	}
	return historyPageProjection(b.withSteers(id, raw), cursor, offset, limit, source, indexOnly, lazy)
}

func historyPage(raw map[string]any, cursor string, offset, limit int, source string) (HistoryPage, error) {
	return historyPageMode(raw, cursor, offset, limit, source, false)
}
func historyPageMode(raw map[string]any, cursor string, offset, limit int, source string, indexOnly bool) (HistoryPage, error) {
	return historyPageProjection(raw, cursor, offset, limit, source, indexOnly, false)
}
func historyPageProjection(raw map[string]any, cursor string, offset, limit int, source string, indexOnly, lazy bool) (HistoryPage, error) {
	turns := orderedTurns(raw)
	page := HistoryPage{Total: len(turns), Source: source, Questions: []Question{}, Turns: []any{}}
	found := cursor == "" || cursor == "latest"
	ids := make([]string, len(turns))
	for i, entry := range turns {
		turn, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		id, _ := turn["turnId"].(string)
		if id == "" {
			id, _ = turn["id"].(string)
		}
		if id == "" {
			id = fmt.Sprintf("turn-%d", i)
		}
		ids[i] = id
		if cursor == id {
			offset = i
			found = true
		}
		for _, entry := range orderedItems(turn["items"]) {
			item, ok := entry.(map[string]any)
			if !ok || item["type"] != "userMessage" {
				continue
			}
			value, _ := item["text"].(string)
			if value == "" {
				switch content := item["content"].(type) {
				case string:
					value = content
				case []any:
					for _, part := range content {
						if p, ok := part.(map[string]any); ok {
							if t, ok := p["text"].(string); ok {
								value += t + " "
							}
						}
					}
				}
			}
			title := []rune(strings.Join(strings.Fields(value), " "))
			if len(title) == 0 {
				continue
			}
			if len(title) > 160 {
				title = append(title[:160], '.', '.', '.')
			}
			page.Questions = append(page.Questions, Question{Turn: id, Offset: i, Text: string(title)})
		}
	}
	if !found {
		return page, errors.New("history changed; refresh the question index")
	}
	if cursor == "latest" {
		offset = max(0, len(turns)-limit)
	}
	offset = min(offset, len(turns))
	end := min(offset+limit, len(turns))
	page.Offset = offset
	page.HasMore = end < len(turns)
	if indexOnly {
		return page, nil
	}
	projected := projectState(State{document: &nativeDocument{value: map[string]any{"turns": turns[offset:end]}}}, lazy)
	var p map[string]any
	if err := json.Unmarshal(projected.Snapshot, &p); err != nil {
		return page, err
	}
	page.Turns = p["turns"].([]any)
	for i, entry := range page.Turns {
		entry.(map[string]any)["turnId"] = ids[offset+i]
	}
	page.Offset = offset
	page.HasMore = end < len(turns)
	return page, nil
}

func mustJSON(v any) json.RawMessage { raw, _ := json.Marshal(v); return raw }
