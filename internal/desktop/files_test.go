package desktop

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cxpalm/internal/attachments"
)

func TestOutputFilesRecognizeCitationsAndLinksAndKeepNativeText(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	row, err := b.History.Get(context.Background(), "thread")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(row.CWD, "review result.xlsx")
	data := []byte("synthetic document transport fixture")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	body := `Download :codex-file-citation{path="` + filepath.ToSlash(path) + `" purpose="output"}` +
		"\n[Same file](<" + filepath.ToSlash(path) + ">)\n[Web list](review.html)\n[Remote](https://example.test/review.xlsx)" +
		"\n![image](not-a-document.xlsx)"
	item := map[string]any{"id": "output", "type": "agentMessage", "text": body}
	refs := fileReferences(item)
	if len(refs) != 2 || refs[0].Name != "review result.xlsx" || refs[1].Kind != "html" {
		t.Fatal("output references missing, remote/image admitted, or duplicate retained", refs)
	}
	b.sessions["thread"] = &State{ID: "thread", Snapshot: mustJSON(map[string]any{"turns": []any{map[string]any{"turnId": "turn", "items": []any{item}}}})}
	got, name, err := b.File(context.Background(), "thread", "turn", "output", refs[0].ID)
	if err != nil || name != refs[0].Name || !bytes.Equal(got, data) {
		t.Fatal("referenced output unavailable", name, err)
	}
	projected := PhoneState(*b.sessions["thread"])
	if strings.Contains(string(projected.Snapshot), ":codex-file-citation{") || !strings.Contains(string(projected.Snapshot), `"files"`) {
		t.Fatal("citation not presented as a file")
	}
	if item["text"] != body {
		t.Fatal("native text changed")
	}
	if _, _, err := b.File(context.Background(), "thread", "turn", "output", strings.Repeat("f", 64)); err == nil {
		t.Fatal("unreferenced output opened")
	}
	if refs := fileReferences(map[string]any{"type": "commandExecution", "text": body}); len(refs) != 0 {
		t.Fatal("hidden tool output published files")
	}
}

func TestOutputFilePathsRejectPrivateRemoteAndActiveFormats(t *testing.T) {
	for _, path := range []string{
		"https://example.test/report.xlsx", "file://remote/report.xlsx", "file:///report.xlsx?token=bad",
		"javascript:report.html", "data:text/html,report.html", "safe.txt:secret.xlsx",
		".env", ".env.json", ".codex/auth.json", ".aws/credentials.json", "auth.json",
		"nested/../.local/secrets.txt", "credentials.json", "cxpalm.json",
		".codex/artifacts/../private.txt",
		"script.exe", "script.svg", "script.js", "secret.pem", "chat.jsonl", "database.sqlite",
		"file%00.xlsx", "file%0a.xlsx", "file.xlsx?secret", "file.xlsx#secret",
	} {
		if source, _ := outputPath(path); source != "" {
			t.Error("unsafe output accepted", path)
		}
	}
	for _, path := range []string{"report.xlsx", "outputs/report%20result.xlsx", "/workspace/outputs/report.pdf", "sandbox:/mnt/data/report.csv", "file:///workspace/report.html", "/home/fixture/.codex/artifacts/report.xlsx"} {
		if source, _ := outputPath(path); source == "" {
			t.Error("valid output lost", path)
		}
	}
}

func TestOutputFilesStayRootedAndBounded(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	row, err := b.History.Get(context.Background(), "thread")
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	os.WriteFile(outside, []byte("private outside-root fixture"), 0600)
	oversized := filepath.Join(row.CWD, "oversized.txt")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	file.Truncate(attachments.MaxSize + 1)
	file.Close()
	paths := []string{outside, oversized, filepath.Join(row.CWD, "missing.xlsx"), "../outside.txt"}
	link := filepath.Join(row.CWD, "escape.txt")
	if os.Symlink(outside, link) == nil {
		paths = append(paths, link)
	}
	for _, path := range paths {
		item := map[string]any{"id": "output", "type": "agentMessage", "text": "[output](<" + filepath.ToSlash(path) + ">)"}
		b.sessions["thread"] = &State{ID: "thread", Snapshot: mustJSON(map[string]any{"turns": []any{map[string]any{"turnId": "turn", "items": []any{item}}}})}
		refs := fileReferences(item)
		if len(refs) != 1 {
			t.Fatal("fixture reference missing")
		}
		if _, _, err := b.File(context.Background(), "thread", "turn", "output", refs[0].ID); err == nil {
			t.Error("out-of-root, symlink, missing or oversized file served", path)
		}
	}
}
