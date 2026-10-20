package desktop

import (
	"context"
	"cxpalm/internal/config"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"
)

func TestLiveDesktopReadOnly(t *testing.T) {
	path := os.Getenv("CXPALM_LIVE_CONFIG")
	if path == "" {
		t.Skip("opt-in read-only desktop integration")
	}
	c, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	bridge := NewBridge(c)
	defer bridge.Close()
	sessions, e := bridge.History.List(ctx, "", false)
	if e != nil {
		t.Fatal(e)
	}
	if len(sessions) == 0 {
		t.Fatal("no official desktop chat found")
	}
	states, remove, e := bridge.Subscribe(ctx, sessions[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	defer remove()
	for {
		select {
		case state := <-states:
			if !state.Connected {
				if state.Error != "" {
					t.Fatal(state.Error)
				}
				continue
			}
			phone := PhoneState(state)
			var snapshot struct {
				Turns []struct {
					Items []json.RawMessage `json:"items"`
				} `json:"turns"`
			}
			if e = json.Unmarshal(phone.Snapshot, &snapshot); e != nil {
				t.Fatal(e)
			}
			items := 0
			for _, turn := range snapshot.Turns {
				items += len(turn.Items)
			}
			if items == 0 {
				var raw map[string]any
				json.Unmarshal(state.Snapshot, &raw)
				t.Logf("root keys=%v", keys(raw))
				turns := orderedTurns(raw)
				t.Logf("ordered turns=%d", len(turns))
				if len(turns) > 0 {
					if turn, ok := turns[len(turns)-1].(map[string]any); ok {
						t.Logf("turn keys=%v", keys(turn))
						entries := orderedItems(turn["items"])
						t.Logf("ordered items=%d", len(entries))
						if len(entries) > 0 {
							t.Logf("item shape=%v", shape(entries[0], 3))
						}
					}
				}
				t.Fatal("native snapshot has no renderable phone messages")
			}
			t.Logf("authorized chats=%d; native bytes=%d; phone bytes=%d; displayed turns=%d; messages=%d", len(sessions), len(state.Snapshot), len(phone.Snapshot), len(snapshot.Turns), items)
			return
		case <-ctx.Done():
			t.Fatal("passive desktop subscription timed out")
		}
	}
}
func keys(m map[string]any) []string {
	out := []string{}
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
func shape(v any, depth int) any {
	if depth < 0 {
		return "..."
	}
	switch value := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for _, key := range keys(value) {
			out[key] = shape(value[key], depth-1)
		}
		return out
	case []any:
		if len(value) == 0 {
			return "empty array"
		}
		return []any{len(value), shape(value[0], depth-1)}
	case string:
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	default:
		return "null"
	}
}
