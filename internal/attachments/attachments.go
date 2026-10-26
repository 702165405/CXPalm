package attachments

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"cxpalm/internal/store"
)

const MaxSize = 20 << 20
const MaxCount = 8

var validID = regexp.MustCompile(`^[a-f0-9]{64}$`)

type File struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Image  bool   `json:"image"`
	Device string `json:"-"`
	Thread string `json:"-"`
	Path   string `json:"-"`
}
type metadata struct {
	File   File   `json:"file"`
	Device string `json:"device"`
	Thread string `json:"thread"`
}
type Manager struct {
	Root string
	mu   sync.Mutex
}

func readMetadata(root *os.Root, path string) ([]byte, error) {
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 16384))
}
func writeMetadata(root *os.Root, path string, raw []byte) error {
	f, err := root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func cleanName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	runes := []rune(name)
	if len(runes) > 120 {
		name = string(runes[len(runes)-120:])
	}
	if name == "" {
		name = "file"
	}
	return "upload-" + name
}

func (m *Manager) Save(device, thread, name string, content io.Reader) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if device == "" || thread == "" {
		return File{}, errors.New("attachment scope required")
	}
	if err := os.MkdirAll(m.Root, 0700); err != nil {
		return File{}, err
	}
	root, err := os.OpenRoot(m.Root)
	if err != nil {
		return File{}, err
	}
	defer root.Close()
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		return File{}, err
	}
	var total, owned int64
	count := 0
	for _, entry := range entries {
		if !validID.MatchString(entry.Name()) {
			continue
		}
		raw, e := readMetadata(root, filepath.Join(entry.Name(), "meta.json"))
		if e != nil {
			return File{}, errors.New("attachment storage unavailable")
		}
		var meta metadata
		if json.Unmarshal(raw, &meta) != nil {
			return File{}, errors.New("attachment metadata unavailable")
		}
		total += meta.File.Size
		if meta.Device == device {
			owned += meta.File.Size
			count++
		}
	}
	if total+MaxSize > 512<<20 || owned+MaxSize > 256<<20 || len(entries) >= 512 || count >= 128 {
		return File{}, errors.New("attachment storage limit reached; clean unused uploads on the computer")
	}
	f := File{ID: store.Secret(), Name: cleanName(name), Device: device, Thread: thread}
	if err = root.Mkdir(f.ID, 0700); err != nil {
		return File{}, err
	}
	path := filepath.Join(f.ID, f.Name)
	ok := false
	defer func() {
		if !ok {
			root.Remove(path)
			root.Remove(filepath.Join(f.ID, "meta.json"))
			root.Remove(f.ID)
		}
	}()
	out, err := root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return File{}, err
	}
	f.Size, err = io.Copy(out, io.LimitReader(content, MaxSize+1))
	closeErr := out.Close()
	if err != nil {
		return File{}, err
	}
	if closeErr != nil {
		return File{}, closeErr
	}
	if f.Size == 0 || f.Size > MaxSize {
		return File{}, errors.New("file must contain 1-20 MiB")
	}
	in, err := root.Open(path)
	if err != nil {
		return File{}, err
	}
	dim, _, imageErr := image.DecodeConfig(in)
	in.Close()
	f.Image = imageErr == nil && dim.Width > 0 && dim.Height > 0 && int64(dim.Width)*int64(dim.Height) <= 40_000_000
	raw, err := json.Marshal(metadata{File: f, Device: device, Thread: thread})
	if err != nil {
		return File{}, err
	}
	if err = writeMetadata(root, filepath.Join(f.ID, "meta.json"), raw); err != nil {
		return File{}, err
	}
	ok = true
	return f, nil
}

func (m *Manager) Resolve(device, thread string, ids []string) ([]File, error) {
	if len(ids) > MaxCount {
		return nil, errors.New("at most 8 attachments per message")
	}
	files := []File{}
	seen := map[string]bool{}
	if len(ids) == 0 {
		return files, nil
	}
	root, err := os.OpenRoot(m.Root)
	if err != nil {
		return nil, errors.New("attachment storage unavailable")
	}
	defer root.Close()
	for _, id := range ids {
		if !validID.MatchString(id) || seen[id] {
			return nil, errors.New("invalid or duplicate attachment")
		}
		seen[id] = true
		raw, err := readMetadata(root, filepath.Join(id, "meta.json"))
		if err != nil {
			return nil, errors.New("attachment unavailable")
		}
		var meta metadata
		if json.Unmarshal(raw, &meta) != nil || meta.Device != device || meta.Thread != thread || meta.File.ID != id || meta.File.Name != filepath.Base(meta.File.Name) {
			return nil, errors.New("attachment scope rejected")
		}
		rel := filepath.Join(id, meta.File.Name)
		info, err := root.Lstat(rel)
		if err != nil || !info.Mode().IsRegular() || info.Size() != meta.File.Size {
			return nil, errors.New("attachment file unavailable")
		}
		path, err := filepath.EvalSymlinks(filepath.Join(m.Root, rel))
		if err != nil {
			return nil, err
		}
		base, err := filepath.EvalSymlinks(m.Root)
		if err != nil {
			return nil, err
		}
		within, err := filepath.Rel(base, path)
		if err != nil || !filepath.IsLocal(within) {
			return nil, errors.New("attachment path rejected")
		}
		meta.File.Path = path
		meta.File.Device = device
		meta.File.Thread = thread
		files = append(files, meta.File)
	}
	return files, nil
}

func Input(text string, files []File) []any {
	input := []any{}
	if strings.TrimSpace(text) != "" {
		input = append(input, map[string]any{"type": "text", "text": text, "text_elements": []any{}})
	}
	for _, f := range files {
		label := fmt.Sprintf("Attached file: %s\nLocal path: %s", f.Name, f.Path)
		input = append(input, map[string]any{"type": "text", "text": label, "text_elements": []any{}})
		if f.Image {
			input = append(input, map[string]any{"type": "localImage", "path": f.Path})
		}
	}
	return input
}
