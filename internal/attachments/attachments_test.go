package attachments

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadScopeNativeInputAndRestart(t *testing.T) {
	m := &Manager{Root: filepath.Join(t.TempDir(), "uploads")}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	f, err := m.Save("phone", "chat", "../../photo.png", &data)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Image || strings.ContainsAny(f.Name, `/\`) {
		t.Fatal("unsafe image metadata", f)
	}
	encoded, _ := json.Marshal(f)
	if strings.Contains(string(encoded), "phone") || strings.Contains(string(encoded), m.Root) {
		t.Fatal("private metadata leaked")
	}
	next := &Manager{Root: m.Root}
	for _, scope := range [][2]string{{"other", "chat"}, {"phone", "other"}} {
		if _, err = next.Resolve(scope[0], scope[1], []string{f.ID}); err == nil {
			t.Fatal("cross-scope attachment accepted")
		}
	}
	for _, ids := range [][]string{{"../outside"}, {f.ID, f.ID}, make([]string, 9)} {
		if _, err = next.Resolve("phone", "chat", ids); err == nil {
			t.Fatal("invalid references accepted")
		}
	}
	files, err := next.Resolve("phone", "chat", []string{f.ID})
	if err != nil {
		t.Fatal(err)
	}
	input := Input("", files)
	if len(input) != 2 || input[1].(map[string]any)["type"] != "localImage" {
		t.Fatal("image not sent natively")
	}
	if _, err = m.Save("phone", "chat", "empty.txt", strings.NewReader("")); err == nil {
		t.Fatal("empty accepted")
	}
	if _, err = m.Save("phone", "chat", "large.txt", io.LimitReader(zeroReader{}, MaxSize+1)); err == nil {
		t.Fatal("oversized accepted")
	}
	entries, _ := os.ReadDir(m.Root)
	if len(entries) != 1 {
		t.Fatal("failed uploads left storage behind")
	}
	regular, err := m.Save("phone", "chat", "sample.txt", strings.NewReader("file contents"))
	if err != nil || regular.Image {
		t.Fatal(regular, err)
	}
	files, err = m.Resolve("phone", "chat", []string{regular.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(Input("read this", files)) != 2 {
		t.Fatal("file reference missing")
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }
