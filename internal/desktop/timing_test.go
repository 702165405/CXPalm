package desktop

import (
	"context"
	"cxpalm/internal/config"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimingVariantsAndImmutableBase(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		want   float64
		exists bool
	}{
		{`{"status":"completed","durationMs":0}`, 0, true},
		{`{"status":"failed","turnDurationMs":123000}`, 123000, true},
		{`{"status":"interrupted","turnStartedAtMs":1000,"turnCompletedAtMs":66000}`, 65000, true},
		{`{"status":"completed","durationMs":-1,"turnDurationMs":2000}`, 2000, true},
		{`{"status":"completed","turnStartedAtMs":2000,"turnCompletedAtMs":1000}`, 0, false},
		{`{"status":"completed","durationMs":"23"}`, 0, false},
		{`{"status":"inProgress","turnStartedAtMs":1000,"durationMs":2000}`, 0, false},
		{`{"status":"unknown","durationMs":4000}`, 0, false},
	} {
		var turn map[string]any
		json.Unmarshal([]byte(tc.raw), &turn)
		base := string(mustJSON(turn))
		out := map[string]any{}
		projectTiming(turn, out)
		duration, ok := out["durationMs"].(float64)
		if ok != tc.exists || ok && duration != tc.want {
			t.Fatalf("wrong timing %s: %v", tc.raw, out)
		}
		if string(mustJSON(turn)) != base {
			t.Fatal("native base mutated")
		}
	}
}

func TestRolloutTimingSurvivesHistoryPaging(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "rollout.jsonl")
	lines := []string{
		`{"type":"event_msg","timestamp":"2026-10-18T00:00:00Z","payload":{"type":"task_started","turn_id":"a"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"text":"question"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"text":"attachment text"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"text":"answer"}]}}`,
		`{"type":"event_msg","timestamp":"2026-10-18T00:02:00Z","payload":{"type":"task_complete","turn_id":"foreign","duration_ms":1}}`,
		`{"type":"event_msg","timestamp":"2026-10-18T00:02:00Z","payload":{"type":"task_complete","turn_id":"a"}}`,
		`{"type":"event_msg","timestamp":"2026-10-18T00:03:00Z","payload":{"type":"task_started","turn_id":"b"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"text":"question two"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"text":"answer two"}]}}`,
		`{"type":"event_msg","timestamp":"2026-10-18T00:04:00Z","payload":{"type":"turn_aborted","turn_id":"b","duration_ms":55000}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	for _, complete := range []bool{false, true} {
		raw, err := b.readHistory(context.Background(), Session{ID: "test", RolloutPath: path}, complete)
		if err != nil {
			t.Fatal(err)
		}
		var source map[string]any
		json.Unmarshal(raw, &source)
		page, err := historyPage(source, "", 0, 20, "rollout")
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Turns) != 2 {
			t.Fatalf("turn boundary lost: %d", len(page.Turns))
		}
		a := page.Turns[0].(map[string]any)
		bb := page.Turns[1].(map[string]any)
		if len(a["items"].([]any)) != 3 {
			t.Fatal("multiple user items split the official turn")
		}
		if a["turnId"] != "a" || a["durationMs"] != float64(120000) || bb["status"] != "interrupted" || bb["durationMs"] != float64(55000) {
			t.Fatalf("timing lost: %v %v", a, bb)
		}
	}
}

// Opt-in passive reads only; output contains field counts, never message text.
func TestLiveOfficialTurnTimingReadOnly(t *testing.T) {
	path := os.Getenv("CXPALM_TIMING_CONFIG")
	if path == "" {
		t.Skip("opt-in official timing inspection")
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
	states, remove, err := b.Subscribe(ctx, rows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer remove()
	for {
		select {
		case state := <-states:
			if !state.Connected {
				continue
			}
			var raw map[string]any
			json.Unmarshal(state.Snapshot, &raw)
			counts := map[string]int{}
			turns := orderedTurns(raw)
			for _, entry := range turns {
				turn, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				for _, key := range []string{"durationMs", "turnDurationMs", "turnStartedAtMs", "turnCompletedAtMs"} {
					if turn[key] != nil {
						counts[key]++
					}
				}
				projected := map[string]any{}
				projectTiming(turn, projected)
				if projected["durationMs"] != nil {
					counts["normalizedCompleted"]++
				}
			}
			if counts["normalizedCompleted"] == 0 {
				t.Fatalf("no real completed timing in %d turns: %v", len(turns), counts)
			}

			rollout, err := b.readHistory(ctx, rows[0], true)
			if err != nil {
				t.Fatal(err)
			}
			var history map[string]any
			if err := json.Unmarshal(rollout, &history); err != nil {
				t.Fatal(err)
			}
			timed := 0
			for _, entry := range orderedTurns(history) {
				if turn, ok := entry.(map[string]any); ok {
					out := map[string]any{}
					projectTiming(turn, out)
					if out["durationMs"] != nil {
						timed++
					}
				}
			}
			if timed == 0 {
				t.Fatal("official rollout lost all completed durations")
			}
			t.Logf("official timing: turns=%d fields=%v rolloutCompleted=%d nativeWrites=false", len(turns), counts, timed)
			return
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
