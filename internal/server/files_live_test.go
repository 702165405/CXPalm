package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cxpalm/internal/config"
	"cxpalm/internal/store"
)

func TestLiveReturnedFilesReadOnlyHTTP(t *testing.T) {
	path, cwd := os.Getenv("CXPALM_LIVE_FILE_CONFIG"), os.Getenv("CXPALM_LIVE_FILE_CWD")
	var names []string
	if path == "" || cwd == "" || json.Unmarshal([]byte(os.Getenv("CXPALM_LIVE_FILE_NAMES")), &names) != nil || len(names) == 0 {
		t.Skip("opt-in existing output reads with isolated authentication")
	}
	directory := os.Getenv("CXPALM_LIVE_FILE_DIR")
	if directory == "" {
		directory = cwd
	}
	if relative, err := filepath.Rel(cwd, directory); err != nil || !filepath.IsLocal(relative) {
		t.Fatal("expected outputs must stay within the selected workspace")
	}
	expected := map[string][32]byte{}
	for _, name := range names {
		if filepath.Base(name) != name {
			t.Fatal("expected a filename, not a caller path")
		}
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal("expected output unavailable")
		}
		expected[name] = sha256.Sum256(data)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("private configuration unavailable")
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
	_, token, err := loginFixture(t, db, "isolated returned-file verifier")
	if err != nil {
		t.Fatal(err)
	}
	origin, _ := url.Parse(cfg.Origin)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	found := map[string]bool{}
	checked, references := 0, 0
	for _, archived := range []bool{false, true} {
		sessions, err := s.Bridge.History.ListPage(ctx, filepath.Base(cwd), archived, 100, 0)
		if err != nil {
			t.Fatal("read-only history lookup failed")
		}
		for _, session := range sessions {
			normalize := func(value string) string { return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(value)), "//?/") }
			if !strings.EqualFold(normalize(session.CWD), normalize(cwd)) {
				continue
			}
			checked++
			page, err := s.Bridge.HistoryPage(ctx, session.ID, "latest", 0, 20)
			if err != nil {
				t.Fatal("read-only output history failed")
			}
			for _, entry := range page.Turns {
				turn := entry.(map[string]any)
				items, _ := turn["items"].([]any)
				for _, entry := range items {
					item := entry.(map[string]any)
					files, _ := item["files"].([]any)
					references += len(files)
					for _, entry := range files {
						file := entry.(map[string]any)
						name, _ := file["name"].(string)
						digest, needed := expected[name]
						if !needed || found[name] {
							continue
						}
						query, _ := json.Marshal(map[string]any{"thread": session.ID, "turn": turn["turnId"], "item": item["id"], "file": file["id"]})
						req, _ := http.NewRequestWithContext(ctx, "POST", ts.URL+"/api/v1/chat/file", bytes.NewReader(query))
						req.Host = origin.Host
						req.Header.Set("Origin", cfg.Origin)
						req.Header.Set("Authorization", "Bearer "+token)
						req.Header.Set("Content-Type", "application/json")
						response, err := http.DefaultClient.Do(req)
						if err != nil {
							t.Fatal("local authenticated download failed")
						}
						data, err := io.ReadAll(io.LimitReader(response.Body, (20<<20)+1))
						response.Body.Close()
						if err != nil || response.StatusCode != 200 || response.Header.Get("Content-Type") != "application/octet-stream" || sha256.Sum256(data) != digest {
							t.Fatal("actual output bytes did not match the referenced local file")
						}
						found[name] = true
						t.Logf("Actual referenced output downloaded through isolated HTTP authentication: %d bytes; hash matches; no model mutation", len(data))
					}
				}
			}
		}
	}
	if len(found) != len(expected) {
		t.Logf("Read-only candidates: %d chats, %d projected file references, %d requested matches", checked, references, len(found))
		t.Fatal("requested output references not found in current history")
	}
}
