package server

import (
	"context"
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This uses isolated gateway authorizations and follows real chats read-only.
func TestOpenBrowserReadOnlyProfile(t *testing.T) {
	path := os.Getenv("CXPALM_PERF_CONFIG")
	if path == "" || os.Getenv("CXPALM_PERF_BROWSER") == "" {
		t.Skip("opt-in local browser profile")
	}
	cfg, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	root := os.Getenv("CXPALM_PERF_ROOT")
	if root == "" {
		t.Fatal("explicit source root required")
	}
	cfg.DataDir = t.TempDir()
	db, e := store.Open(cfg.DataDir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.SetLogin(store.LoginInput{Username: "fixture", Password: "fixture-test-password-012345"})
	if e != nil {
		t.Fatal(e)
	}
	s := New(cfg, db)
	defer s.Bridge.Close()
	web := os.Getenv("CXPALM_PERF_WEB")
	if web == "" {
		web = filepath.Join(root, "web")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cxpalm/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		if r.URL.Path == "/cxpalm/" {
			name = "index.html"
		}
		file := filepath.Join(web, name)
		if _, e := os.Stat(file); e != nil {
			file = filepath.Join(root, "web", name)
		}
		if name != "index.html" && name != "app-v1.mjs" && name != "protocol-v1.mjs" && name != "control-v1.css" && name != "icon.svg" && name != "markdown-v1.mjs" && name != "marked-vendor.mjs" && name != "purify-vendor.mjs" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, file)
	})
	mux.Handle("/", s.Handler())
	ts := httptest.NewTLSServer(mux)
	defer ts.Close()
	s.Config.Origin = ts.URL
	rows, e := s.Bridge.History.List(context.Background(), "", false)
	if e != nil || len(rows) < 2 {
		t.Fatal("need two real read-only chats", e)
	}
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, row.ID)
		if len(ids) == 5 {
			break
		}
	}
	input, _ := json.Marshal(map[string]any{"url": ts.URL, "username": "fixture", "password": "fixture-test-password-012345", "ids": ids, "output": os.Getenv("CXPALM_PERF_OUTPUT")})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(root, "scripts", "perf-open.cjs"))
	cmd.Env = append(os.Environ(), "CXPALM_PERF_INPUT="+string(input))
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("browser profile failed: %v: %s", e, out)
	}
	t.Log(string(out))
}
