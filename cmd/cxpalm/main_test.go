package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesOnlyConnectionConfiguration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cxpalm.json")
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{"cxpalm", "init", "--config", path, "--origin", "https://cx.example", "--codex-home", root}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["projects"]; ok {
		t.Fatal("retired mapping generated")
	}
	if fields["codexHome"] != root || fields["origin"] != "https://cx.example" {
		t.Fatal("connection configuration missing")
	}
}
func TestRetiredFlagRejectedBeforeCreatingFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cxpalm.json")
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{"cxpalm", "init", "--config", path, "--project", "obsolete=missing"}
	if err := run(); err == nil {
		t.Fatal("retired flag accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("failed command created config: %v", err)
	}
}

func TestLinkCommandsAreRemoved(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.json")
	raw, _ := json.Marshal(map[string]any{"listen": "127.0.0.1:18789", "origin": "https://example.test", "dataDir": root, "codexHome": root})
	os.WriteFile(configPath, raw, 0600)
	old := os.Args
	defer func() { os.Args = old }()
	for _, command := range []string{"pair", "pair-ui"} {
		os.Args = []string{"cxpalm", command, "--config", configPath}
		if err := run(); err == nil || err.Error() != "unknown command" {
			t.Fatalf("retired command %s accepted: %v", command, err)
		}
	}
}
