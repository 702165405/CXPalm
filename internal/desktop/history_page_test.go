package desktop

import (
	"context"
	"cxpalm/internal/config"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoryPageAllTurnsAndFullContent(t *testing.T) {
	turns := []any{}
	long := strings.Repeat("full text ", 2000)
	for i := 0; i < 65; i++ {
		turns = append(turns, map[string]any{"turnId": fmt.Sprint("t", i), "status": "completed", "items": []any{map[string]any{"id": "user", "type": "userMessage", "text": fmt.Sprint("question ", i)}, map[string]any{"id": "reply", "type": "agentMessage", "text": long}, map[string]any{"id": "tool", "type": "commandExecution", "aggregatedOutput": "tool output"}}})
	}
	raw := map[string]any{"turns": turns}
	baseline := mustJSON(raw)
	for _, tc := range []struct {
		cursor       string
		offset, want int
	}{{"latest", 0, 45}, {"", 0, 0}, {"", 20, 20}, {"t32", 0, 32}, {"t64", 0, 64}} {
		page, err := historyPage(raw, tc.cursor, tc.offset, 20, "desktop")
		if err != nil {
			t.Fatal(err)
		}
		if page.Offset != tc.want || page.Total != 65 || len(page.Questions) != 65 || len(page.Turns) != min(20, 65-tc.want) {
			t.Fatal("unreachable history", page.Offset, len(page.Turns))
		}
		text := page.Turns[0].(map[string]any)["items"].([]any)[1].(map[string]any)["text"]
		if text != long {
			t.Fatal("full message cut")
		}
		if len(page.Turns[0].(map[string]any)["items"].([]any)) != 3 {
			t.Fatal("tool details lost")
		}
	}
	if string(baseline) != string(mustJSON(raw)) {
		t.Fatal("native patch base modified")
	}
	if _, err := historyPage(raw, "missing", 0, 20, "desktop"); err == nil {
		t.Fatal("stale question silently redirected")
	}
}

func TestHistoryPageReadsWholeArchivedRolloutAndChecksBoundary(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "rollout.jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 45; i++ {
		for _, role := range []string{"system", "developer", "user", "assistant"} {
			line, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": role, "content": []any{map[string]any{"text": fmt.Sprintf("%s-%d %s", role, i, strings.Repeat("x", 15000))}}}})
			file.Write(append(line, '\n'))
		}
	}
	file.Close()
	db, err := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE threads(id TEXT,cwd TEXT,source TEXT,archived INTEGER,updated_at INTEGER,rollout_path TEXT);INSERT INTO threads VALUES('chat','.','vscode',1,1,?)`, path)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	for _, offset := range []int{0, 20, 40} {
		page, err := b.HistoryPage(context.Background(), "chat", "", offset, 20)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 45 || len(page.Questions) != 45 || page.Offset != offset {
			t.Fatal("rollout history incomplete", page.Total, page.Offset)
		}
		text := string(mustJSON(page))
		if strings.Contains(text, "system-0") || strings.Contains(text, "developer-0") {
			t.Fatal("private roles leaked")
		}
	}
	for _, args := range []struct {
		id            string
		offset, limit int
	}{{"missing", 0, 20}, {"chat", -1, 20}, {"chat", 0, 21}} {
		if _, err := b.HistoryPage(context.Background(), args.id, "", args.offset, args.limit); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	empty, err := b.HistoryPage(context.Background(), "chat", "latest", 0, 20)
	if err != nil || empty.Total != 0 || len(empty.Turns) != 0 {
		t.Fatal("empty new chat treated as missing history", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, []byte("private"), 0600)
	if _, err := b.readHistory(context.Background(), Session{RolloutPath: outside}, true); err == nil {
		t.Fatal("outside root accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.HistoryPage(ctx, "chat", "", 0, 20); err == nil {
		t.Fatal("canceled read succeeded")
	}
}

// Opt-in official desktop read only; records counts/hashes, never message bodies.
func TestLiveCompleteHistoryReadOnly(t *testing.T) {
	path := os.Getenv("CXPALM_HISTORY_CONFIG")
	if path == "" {
		t.Skip("opt-in read-only official desktop history")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBridge(cfg)
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	rows, err := b.History.List(ctx, "", false)
	if err != nil || len(rows) == 0 {
		t.Fatal("history unavailable", err)
	}
	ch, remove, err := b.Subscribe(ctx, rows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer remove()
	for {
		select {
		case state := <-ch:
			if state.Connected {
				goto connected
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
connected:
	first, err := b.HistoryPage(ctx, rows[0].ID, "", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	last, err := b.HistoryPage(ctx, rows[0].ID, "latest", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if first.Offset != 0 || last.Total != first.Total || len(first.Questions) == 0 || len(first.Turns) == 0 || len(last.Turns) == 0 {
		t.Fatal("incomplete live history")
	}
	target := first.Questions[0]
	jump, err := b.HistoryPage(ctx, rows[0].ID, target.Turn, 0, 20)
	if err != nil || jump.Offset != target.Offset {
		t.Fatal("cannot reach first question", err)
	}
	result := map[string]any{"source": first.Source, "totalTurns": first.Total, "questions": len(first.Questions), "firstPage": len(first.Turns), "lastOffset": last.Offset, "lastPage": len(last.Turns), "firstQuestionReachable": true, "nativeWrites": false}
	if path := os.Getenv("CXPALM_HISTORY_RESULT"); path != "" {
		raw, _ := json.MarshalIndent(result, "", "  ")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("read-only full history: %v", result)
}

func TestLatestIndexNeverDuplicatesFullMessageBodies(t *testing.T) {
	text := strings.Repeat("full message body ", 10000)
	raw := map[string]any{"turns": []any{map[string]any{"turnId": "t", "items": []any{map[string]any{"type": "userMessage", "text": "question"}, map[string]any{"type": "agentMessage", "text": text}}}}}
	full, err := historyPage(raw, "latest", 0, 20, "desktop")
	if err != nil {
		t.Fatal(err)
	}
	index, err := historyPageMode(raw, "latest", 0, 20, "desktop", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Turns) != 0 || len(index.Questions) != 1 || index.Total != full.Total || index.Offset != full.Offset || index.HasMore != full.HasMore {
		t.Fatal("history index lost navigation metadata")
	}
	a, _ := json.Marshal(full)
	b, _ := json.Marshal(index)
	if len(b)*100 >= len(a) {
		t.Fatal("index still duplicates body")
	}
	if full.Turns[0].(map[string]any)["items"].([]any)[1].(map[string]any)["text"] != text {
		t.Fatal("speed fix truncated history")
	}
}
