package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectlessWorkspaceIsIndependentAndCannotBindToParentProject(t *testing.T) {
	base := t.TempDir()
	a, err := newProjectlessDirectory(base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := newProjectlessDirectory(base)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || filepath.Dir(a) != base || filepath.Dir(b) != base {
		t.Fatal("shared or escaped workspace")
	}
	for _, p := range []string{a, b} {
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			t.Fatal(err)
		}
	}
	if _, err = newProjectlessDirectory("relative"); err == nil {
		t.Fatal("accepted arbitrary relative workspace")
	}
	home, err := projectlessBase()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "chat-fixture")
	if !isPalmProjectless(path) || isPalmProjectless(filepath.Join(path, "child")) {
		t.Fatal("invalid projectless boundary")
	}
	var m projectMetadata
	raw, _ := json.Marshal(map[string]any{"local-projects": map[string]any{"parent": map[string]any{"rootPaths": []string{filepath.Dir(home)}}}})
	json.Unmarshal(raw, &m)
	row := Session{ID: "new", CWD: path}
	m.assign(&row)
	if row.ProjectID != "none" {
		t.Fatal("projectless inherited a parent project")
	}
}
func TestNewProjectSelectionRejectsUnknownAndAcceptsKnownRoot(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	cwd := t.TempDir()
	raw, _ := json.Marshal(map[string]any{"local-projects": map[string]any{"known": map[string]any{"rootPaths": []string{cwd}}}})
	os.WriteFile(filepath.Join(b.config.CodexHome, ".codex-global-state.json"), raw, 0600)
	if got, err := b.newChatDirectory(context.Background(), "known"); err != nil || got != cwd {
		t.Fatal(got, err)
	}
	for _, bad := range []string{"../../private", "missing", "cwd:" + t.TempDir()} {
		if _, err := b.newChatDirectory(context.Background(), bad); err == nil {
			t.Fatal("arbitrary project accepted", bad)
		}
	}
}
func TestOfficialCLIHonorsNonNpmExplicitOverride(t *testing.T) {
	candidate := filepath.Join(t.TempDir(), "custom-codex")
	got, err := officialCLI(candidate)
	if err != nil || got != candidate {
		t.Fatal("custom CLI override lost")
	}
}
