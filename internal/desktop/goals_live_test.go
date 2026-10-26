package desktop

import (
	"context"
	"cxpalm/internal/config"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestLiveNativeGoalReadOnly(t *testing.T) {
	if os.Getenv("CXPALM_NATIVE_GOAL_READONLY") != "1" {
		t.Skip("optional official metadata read")
	}
	c, e := config.Load(os.Getenv("CXPALM_CONFIG"))
	if e != nil {
		t.Fatal("config unavailable")
	}
	b := NewBridge(c)
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rows, e := b.History.List(ctx, "", false)
	if e != nil || len(rows) == 0 {
		t.Fatal("allowed history unavailable")
	}
	result, e := metadataCall(ctx, c, "thread/goal/get", map[string]any{"threadId": rows[0].ID})
	if e != nil {
		t.Fatal(e)
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(result, &envelope) != nil {
		t.Fatal("invalid response")
	}
	if _, ok := envelope["goal"]; !ok {
		t.Fatal("native goal response missing")
	}
	t.Log("Official goal/get supported; no goal or turn mutation performed")
}
