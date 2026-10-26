package desktop

import (
	"context"
	"cxpalm/internal/config"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestLiveReadOnlyPayloadProfile(t *testing.T) {
	path := os.Getenv("CXPALM_PERF_CONFIG")
	if path == "" {
		t.Skip("opt-in read-only payload profile")
	}
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBridge(c)
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	rows, err := b.History.ListPage(ctx, "", false, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := []map[string]any{}
	for _, row := range rows {
		start := time.Now()
		states, remove, err := b.Subscribe(ctx, row.ID)
		if err != nil {
			t.Fatal(err)
		}
		timer := time.NewTimer(3 * time.Second)
		record := map[string]any{"connected": false}
		done := false
		for !done {
			select {
			case state := <-states:
				if !state.Connected {
					continue
				}
				record["connected"] = true
				record["ownerReadyMs"] = float64(time.Since(start)) / float64(time.Millisecond)
				record["nativeBytes"] = len(state.Snapshot)
				pstart := time.Now()
				phone := b.PhoneState(state)
				record["phoneProjectionMs"] = float64(time.Since(pstart)) / float64(time.Millisecond)
				record["phoneBytes"] = len(phone.Snapshot)
				leanStart := time.Now()
				lean := b.StreamState(state)
				record["streamProjectionMs"] = float64(time.Since(leanStart)) / float64(time.Millisecond)
				record["streamBytes"] = len(lean.Snapshot)
				hstart := time.Now()
				page, err := b.HistoryPage(ctx, row.ID, "latest", 0, 20)
				if err == nil {
					encoded, _ := json.Marshal(page)
					record["historyMs"] = float64(time.Since(hstart)) / float64(time.Millisecond)
					record["historyBytes"] = len(encoded)
					istart := time.Now()
					index, ie := b.HistoryIndex(ctx, row.ID, 20)
					if ie == nil {
						encoded, _ := json.Marshal(index)
						record["indexMs"] = float64(time.Since(istart)) / float64(time.Millisecond)
						record["indexBytes"] = len(encoded)
					}
				}
				done = true
			case <-timer.C:
				done = true
			case <-ctx.Done():
				done = true
			}
		}
		timer.Stop()
		remove()
		out = append(out, record)
	}
	raw, _ := json.MarshalIndent(out, "", "  ")
	if file := os.Getenv("CXPALM_PAYLOAD_OUTPUT"); file != "" {
		if err = os.WriteFile(file, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("read-only live payload stages measured for %d chats; no content or identifiers logged", len(out))
}
