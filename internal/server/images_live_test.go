package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLiveGeneratedImageReadOnlyHTTP(t *testing.T) { liveImagesReadOnlyHTTP(t, false) }
func TestLiveMessageImagesReadOnlyHTTP(t *testing.T)  { liveImagesReadOnlyHTTP(t, true) }

func liveImagesReadOnlyHTTP(t *testing.T, messageImages bool) {
	t.Helper()
	path, thread, expected := os.Getenv("CXPALM_LIVE_IMAGE_CONFIG"), os.Getenv("CXPALM_LIVE_IMAGE_THREAD"), os.Getenv("CXPALM_LIVE_IMAGE_SHA256")
	expectedFiles := map[string]string{}
	if messageImages {
		var files []string
		if json.Unmarshal([]byte(os.Getenv("CXPALM_LIVE_IMAGE_FILES")), &files) != nil || len(files) == 0 {
			t.Skip("opt-in existing message images")
		}
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			expectedFiles[filepath.Base(file)] = hex.EncodeToString(sum[:])
		}
	}
	if path == "" || thread == "" || (!messageImages && expected == "") {
		t.Skip("opt-in existing image read only, temporary authentication store")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = t.TempDir()
	db, err := store.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(cfg, db)
	defer s.Bridge.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	_, token, err := loginFixture(t, db, "isolated image verifier")
	if err != nil {
		t.Fatal(err)
	}
	origin, _ := url.Parse(cfg.Origin)
	check := func(turns []any, phase string) {
		t.Helper()
		found := map[string]bool{}
		for _, entry := range turns {
			turn := entry.(map[string]any)
			items, _ := turn["items"].([]any)
			for _, entry := range items {
				item := entry.(map[string]any)
				images, _ := item["images"].([]any)
				if len(images) == 0 {
					continue
				}
				if !messageImages && item["type"] != "imageGenerationCall" {
					continue
				}
				if item["deferred"] == true {
					t.Fatal("generated image deferred and hidden")
				}
				for _, entry := range images {
					image := entry.(map[string]any)
					name, _ := image["name"].(string)
					want := expected
					if messageImages {
						var ok bool
						want, ok = expectedFiles[name]
						if !ok {
							continue
						}
					}
					q := map[string]any{"thread": thread, "turn": turn["turnId"], "item": item["id"], "image": image["id"]}
					raw, _ := json.Marshal(q)
					req, _ := http.NewRequest("POST", ts.URL+"/api/v1/chat/image", bytes.NewReader(raw))
					req.Host = origin.Host
					req.Header.Set("Origin", cfg.Origin)
					req.Header.Set("Authorization", "Bearer "+token)
					req.Header.Set("Content-Type", "application/json")
					response, err := http.DefaultClient.Do(req)
					if err != nil {
						t.Fatal(err)
					}
					body, _ := io.ReadAll(response.Body)
					response.Body.Close()
					sum := sha256.Sum256(body)
					if response.StatusCode != 200 || response.Header.Get("Content-Type") != "image/png" || hex.EncodeToString(sum[:]) != want {
						t.Fatalf("%s image bytes do not match existing PNG: status=%d bytes=%d", phase, response.StatusCode, len(body))
					}
					t.Logf("%s: actual existing PNG matches SHA-256, bytes=%d, no model mutation", phase, len(body))
					found[name] = true
				}
			}
		}
		if len(found) == 0 || messageImages && len(found) != len(expectedFiles) {
			t.Fatal(phase, "generated image missing from actual history/state")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	page, err := s.Bridge.HistoryPage(ctx, thread, "latest", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	check(page.Turns, "rollout-history")
	states, remove, err := s.Bridge.Subscribe(ctx, thread)
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
			projected := s.Bridge.StreamState(state)
			var raw map[string]any
			if json.Unmarshal(projected.Snapshot, &raw) != nil {
				t.Fatal("invalid stream projection")
			}
			turns, _ := raw["turns"].([]any)
			check(turns, "native-stream")
			return
		case <-ctx.Done():
			t.Fatal("read-only native image follow timed out")
		}
	}
}
