package desktop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"cxpalm/internal/config"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// Opt-in, read-only profile: never sends a turn, marks read, or writes Codex files.
func TestOpenReadOnlyProfile(t *testing.T) {
	path := os.Getenv("CXPALM_PERF_CONFIG")
	if path == "" {
		t.Skip("opt-in local read-only performance profile")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBridge(cfg)
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	rows, err := b.History.ListPage(ctx, "", false, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	selected := []Session{}
	skipped := 0
	for _, row := range rows {
		f, e := b.openRollout(row.RolloutPath)
		if e != nil {
			skipped++
			continue
		}
		f.Close()
		selected = append(selected, row)
		if len(selected) == 5 {
			break
		}
	}
	rows = selected
	if len(rows) == 0 {
		t.Fatal("no readable rollout sample; no empty-state performance claims")
	}
	t.Logf("skipped %d missing/unavailable rollout files", skipped)
	result := map[string]any{"home": cfg.CodexHome, "samples": len(rows)}
	ids := []string{}
	hashes := []string{}
	sizes := []int64{}
	lookup, preview, catalog, projection, cold, warm := []float64{}, []float64{}, []float64{}, []float64{}, []float64{}, []float64{}
	historyMs, historyBytes := []float64{}, []int{}
	ms := func(start time.Time) float64 { return float64(time.Since(start)) / float64(time.Millisecond) }
	for _, row := range rows {
		ids = append(ids, row.ID)
		f, e := b.openRollout(row.RolloutPath)
		if e != nil {
			t.Fatal("valid preview path rejected", e)
		}
		stat, e := f.Stat()
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		sizes = append(sizes, stat.Size())
		for i := 0; i < 5; i++ {
			start := time.Now()
			if _, err = b.History.Get(ctx, row.ID); err != nil {
				t.Fatal(err)
			}
			lookup = append(lookup, ms(start))
			start = time.Now()
			raw := b.historyPreview(ctx, row)
			if len(raw) == 0 {
				t.Fatal("empty preview is not a speed result")
			}
			if i == 0 {
				var decoded map[string]any
				_ = json.Unmarshal(raw, &decoded)
				last := ""
				for _, entry := range orderedTurns(decoded) {
					turn, _ := entry.(map[string]any)
					for _, entry := range orderedItems(turn["items"]) {
						item, _ := entry.(map[string]any)
						if item["type"] == "agentMessage" {
							last, _ = item["text"].(string)
						}
					}
				}
				if last == "" {
					t.Fatal("no actual latest assistant reply")
				}
				hashes = append(hashes, fmt.Sprintf("%x", sha256.Sum256([]byte(last))))
			}
			preview = append(preview, ms(start))
			start = time.Now()
			PhoneState(State{ID: row.ID, Snapshot: raw})
			projection = append(projection, ms(start))
			start = time.Now()
			if _, err = b.Catalog(row.CWD); err != nil {
				t.Fatal(err)
			}
			catalog = append(catalog, ms(start))
		}
		startPage := time.Now()
		page, e := b.HistoryPage(ctx, row.ID, "latest", 0, 20)
		if e != nil {
			t.Fatal(e)
		}
		encoded, _ := json.Marshal(page)
		historyMs = append(historyMs, ms(startPage))
		historyBytes = append(historyBytes, len(encoded))
		for i := 0; i < 4; i++ {
			target := b
			if i == 0 {
				target = NewBridge(cfg)
			}
			start := time.Now()
			states, remove, e := target.Subscribe(ctx, row.ID)
			if e != nil {
				t.Fatal(e)
			}
			wait := time.NewTimer(2 * time.Second)
			found := false
			for !found {
				select {
				case state := <-states:
					phone := PhoneState(state)
					found = bytes.Contains(phone.Snapshot, []byte(`"agentMessage"`))
				case <-wait.C:
					remove()
					target.Close()
					t.Fatal("no actual reply within 2 seconds; empty previews are not a speed result")
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			wait.Stop()
			elapsed := ms(start)
			remove()
			if i == 0 {
				cold = append(cold, elapsed)
				target.Close()
			} else {
				warm = append(warm, elapsed)
			}
		}
	}
	result["latestHistoryMs"] = historyMs
	result["latestHistoryBytes"] = historyBytes
	result["ids"] = ids
	result["latestReplyHashes"] = hashes
	result["fileSizes"] = sizes
	result["lookupMs"] = lookup
	result["previewMs"] = preview
	result["catalogMs"] = catalog
	result["projectionMs"] = projection
	result["coldSubscribeMs"] = cold
	result["repeatSubscribeMs"] = warm
	data, _ := json.MarshalIndent(result, "", "  ")
	if path = os.Getenv("CXPALM_PERF_OUTPUT"); path != "" {
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("read-only stage timings collected: %d chats, %d parse samples, %d cold, %d repeat opens; no content logged", len(rows), len(preview), len(cold), len(warm))
}
