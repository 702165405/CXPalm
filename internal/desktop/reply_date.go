package desktop

import (
	"bufio"
	"encoding/json"
	"io"

	"time"
)

type rolloutSummary struct {
	size     int64
	modified time.Time
	reply    int64
	status   string
}

func (b *Bridge) replyDate(s *Session) {
	if s.StatusSource == "history" {
		s.Status = "unknown"
		s.StatusSource = ""
	}
	if s.RolloutPath == "" {
		return
	}
	f, e := b.openRollout(s.RolloutPath)
	if e != nil {
		return
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil || !stat.Mode().IsRegular() {
		return
	}
	b.mu.Lock()
	cached, ok := b.rollouts[s.RolloutPath]
	b.mu.Unlock()
	if ok && cached.size == stat.Size() && cached.modified.Equal(stat.ModTime()) {
		s.LastReplyAt = cached.reply
		if cached.status != "" {
			s.Status = cached.status
			s.StatusSource = "history"
		}
		return
	}
	// ponytail: bounded tail, not full transcripts; large trailing tool items can
	// leave the reply time unknown, in which case clients label activity instead.
	start := stat.Size() - (1 << 20)
	if start < 0 {
		start = 0
	}
	if _, e = f.Seek(start, io.SeekStart); e != nil {
		return
	}
	scan := bufio.NewScanner(io.LimitReader(f, 1<<20))
	scan.Buffer(make([]byte, 4096), 1<<20)
	if start > 0 {
		scan.Scan()
	}
	var last int64
	status := ""
	for scan.Scan() {
		var entry struct {
			Timestamp string `json:"timestamp"`
			Type      string `json:"type"`
			Payload   struct {
				Type string `json:"type"`
				Role string `json:"role"`
			} `json:"payload"`
		}
		if json.Unmarshal(scan.Bytes(), &entry) != nil {
			continue
		}
		if entry.Type == "event_msg" {
			switch entry.Payload.Type {
			case "task_started":
				status = ""
			case "task_complete":
				status = "completed"
			case "turn_aborted":
				status = "interrupted"
			}
		}
		if entry.Type == "response_item" && entry.Payload.Type == "message" && entry.Payload.Role == "assistant" {
			if date, e := time.Parse(time.RFC3339Nano, entry.Timestamp); e == nil && date.Unix() > last {
				last = date.Unix()
			}
		}
	}
	b.mu.Lock()
	if b.rollouts == nil {
		b.rollouts = map[string]rolloutSummary{}
	}
	if scan.Err() != nil {
		status = ""
	}
	b.rollouts[s.RolloutPath] = rolloutSummary{stat.Size(), stat.ModTime(), last, status}
	b.mu.Unlock()
	s.LastReplyAt = last
	if status != "" {
		s.Status = status
		s.StatusSource = "history"
	}
}
