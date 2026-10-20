package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func (b *Bridge) openRollout(path string) (*os.File, error) {
	root, e := filepath.EvalSymlinks(b.config.CodexHome)
	if e != nil {
		return nil, e
	}
	path, e = filepath.EvalSymlinks(path)
	if e != nil {
		return nil, e
	}
	rel, e := filepath.Rel(historyComparisonPath(root), historyComparisonPath(path))
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, errors.New("history path is outside Codex home")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("history is not a regular file")
	}
	return f, nil
}

// Historical text is a bounded read-only preview, never an execution snapshot.
func (b *Bridge) historyPreview(ctx context.Context, row Session) json.RawMessage {
	raw, _ := b.readHistory(ctx, row, false)
	return raw
}

type historyRecord struct {
	key    string
	info   os.FileInfo
	weight int64
	value  map[string]any
}

func (b *Bridge) readHistory(ctx context.Context, row Session, complete bool) (json.RawMessage, error) {
	value, err := b.readHistoryValue(ctx, row, complete)
	if err != nil || value == nil {
		return nil, err
	}
	return mustJSON(value), nil
}
func (b *Bridge) readHistoryValue(ctx context.Context, row Session, complete bool) (map[string]any, error) {
	f, e := b.openRollout(row.RolloutPath)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	key := row.ID + "\x00" + f.Name() + strconv.FormatBool(complete)
	b.historyMu.Lock()
	for _, record := range b.historyCache {
		if record.key == key && os.SameFile(record.info, info) && record.info.Size() == info.Size() && record.info.ModTime() == info.ModTime() {
			b.historyMu.Unlock()
			return record.value, nil
		}
	}
	b.historyMu.Unlock()
	limit := int64(2 << 20)
	if complete {
		limit = info.Size()
	}
	start := info.Size() - limit
	if start < 0 {
		start = 0
	}
	if _, e = f.Seek(start, io.SeekStart); e != nil {
		return nil, e
	}
	scan := bufio.NewScanner(io.LimitReader(f, limit))
	scan.Buffer(make([]byte, 4096), 64<<20)
	if start > 0 {
		scan.Scan()
	}
	turns := []any{}
	items := []any{}
	number := 0
	current := map[string]any{}
	imageCalls := map[string]bool{}
	flush := func() {
		if len(items) > 0 {
			if _, ok := current["turnId"]; !ok {
				current["turnId"] = "history-" + strconv.Itoa(number)
			}
			if _, ok := current["status"]; !ok {
				current["status"] = "unknown"
			}
			current["items"] = items
			turns = append(turns, current)
			number++
			items = []any{}
		}
		current = map[string]any{}
	}
	for scan.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		line := scan.Bytes()
		message := bytes.Contains(line, []byte(`"response_item"`)) && (bytes.Contains(line, []byte(`"message"`)) || bytes.Contains(line, []byte(`"image_generation_call"`)))
		timing := bytes.Contains(line, []byte(`"event_msg"`)) && (bytes.Contains(line, []byte(`"task_started"`)) || bytes.Contains(line, []byte(`"task_complete"`)) || bytes.Contains(line, []byte(`"turn_aborted"`)))
		imageTool := bytes.Contains(line, []byte(`"function_call"`)) && bytes.Contains(line, []byte(`"imagegen"`)) || bytes.Contains(line, []byte(`"function_call_output"`)) && bytes.Contains(line, []byte(`"input_image"`))
		if !message && !timing && !imageTool {
			continue
		}
		var event struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Payload   struct {
				Type     string           `json:"type"`
				Name     string           `json:"name"`
				CallID   string           `json:"call_id"`
				Output   []map[string]any `json:"output"`
				TurnID   string           `json:"turn_id"`
				Duration any              `json:"duration_ms"`
				Role     string           `json:"role"`
				Result   string           `json:"result"`
				Content  []map[string]any `json:"content"`
			} `json:"payload"`
		}
		if json.Unmarshal(scan.Bytes(), &event) != nil {
			continue
		}
		if event.Type == "response_item" && event.Payload.Type == "function_call" && event.Payload.Name == "imagegen" && event.Payload.CallID != "" {
			imageCalls[event.Payload.CallID] = true
			continue
		}
		if event.Type == "response_item" && event.Payload.Type == "function_call_output" && imageCalls[event.Payload.CallID] {
			content := []any{}
			for _, part := range event.Payload.Output {
				if part["type"] == "input_image" {
					content = append(content, part)
				}
			}
			entry := map[string]any{"id": event.Payload.CallID, "type": "imageGenerationCall", "content": content}
			if len(imageReferences(entry)) > 0 {
				items = append(items, entry)
			}
			delete(imageCalls, event.Payload.CallID)
			continue
		}
		if event.Type == "event_msg" {
			timestamp, err := time.Parse(time.RFC3339Nano, event.Timestamp)
			switch event.Payload.Type {
			case "task_started":
				flush()
				if event.Payload.TurnID != "" {
					current["turnId"] = event.Payload.TurnID
				}
				current["status"] = "inProgress"
				if err == nil {
					current["turnStartedAtMs"] = timestamp.UnixMilli()
				}
			case "task_complete", "turn_aborted":
				if id, _ := current["turnId"].(string); event.Payload.TurnID != "" && id != "" && event.Payload.TurnID != id {
					continue
				}
				current["status"] = "completed"
				if event.Payload.Type == "turn_aborted" {
					current["status"] = "interrupted"
				}
				if err == nil {
					current["turnCompletedAtMs"] = timestamp.UnixMilli()
				}
				if event.Payload.Duration != nil {
					current["durationMs"] = event.Payload.Duration
				}
			}
			continue
		}
		if event.Type == "response_item" && event.Payload.Type == "image_generation_call" && event.Payload.Result != "" {
			items = append(items, map[string]any{"id": "history-item-" + strconv.Itoa(number) + "-" + strconv.Itoa(len(items)), "type": "imageGenerationCall", "result": event.Payload.Result})
			continue
		}
		if event.Type != "response_item" || event.Payload.Type != "message" {
			continue
		}
		kind := ""
		switch event.Payload.Role {
		case "user":
			// A desktop turn may contain several user items (text and attachments).
			if len(items) > 0 && current["status"] != "inProgress" {
				flush()
			}
			kind = "userMessage"
		case "assistant":
			kind = "agentMessage"
		default:
			continue
		}
		texts := []string{}
		for _, part := range event.Payload.Content {
			if text, _ := part["text"].(string); text != "" {
				texts = append(texts, text)
			}
		}
		entry := map[string]any{"id": "history-item-" + strconv.Itoa(number) + "-" + strconv.Itoa(len(items)), "type": kind, "text": strings.Join(texts, "\n"), "content": event.Payload.Content}
		if len(texts) == 0 && len(imageReferences(entry)) == 0 {
			continue
		}
		items = append(items, entry)
		// Trim during parsing, not only when sending the projection.
		if !complete && len(turns) > 20 {
			turns = turns[len(turns)-20:]
		}
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	flush()
	if len(turns) == 0 {
		return nil, nil
	}
	value := map[string]any{"id": row.ID, "turns": turns, "historyMore": !complete && (start > 0 || len(turns) > 20), "historySource": "rollout", "threadRuntimeStatus": map[string]any{"type": "unknown"}}
	weight := min(info.Size(), limit)
	if after, err := f.Stat(); err == nil && after.Size() == info.Size() && after.ModTime() == info.ModTime() && weight <= 64<<20 {
		b.historyMu.Lock()
		keep := make([]historyRecord, 0, len(b.historyCache)+1)
		for _, record := range b.historyCache {
			if record.key != key {
				keep = append(keep, record)
			}
		}
		b.historyCache = append(keep, historyRecord{key: key, info: info, weight: weight, value: value})
		for {
			used := int64(0)
			for _, record := range b.historyCache {
				used += record.weight
			}
			if len(b.historyCache) <= 2 && used <= 64<<20 {
				break
			}
			b.historyCache[0] = historyRecord{}
			b.historyCache = b.historyCache[1:]
		}
		b.historyMu.Unlock()
	}
	return value, nil
}

// Compare equivalent Win32/extended paths after resolving symlinks. Open the
// original resolved path so long filenames keep working; containment is unchanged.
func historyComparisonPath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	if len(path) >= 8 && strings.EqualFold(path[:8], `\\?\UNC\`) {
		return `\\` + path[8:]
	}
	if len(path) >= 7 && strings.HasPrefix(path, `\\?\`) && path[5] == ':' && path[6] == '\\' && ((path[4] >= 'A' && path[4] <= 'Z') || (path[4] >= 'a' && path[4] <= 'z')) {
		return path[4:]
	}
	return path
}
