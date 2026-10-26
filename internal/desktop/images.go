package desktop

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cxpalm/internal/attachments"
	"cxpalm/internal/store"
)

type ImageReference struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"-"`
}

var markdownImage = regexp.MustCompile(`!\[[^\]\r\n]*\]\(\s*(<[^>\r\n]+>|[^\s\r\n]+?)(?:\s+"[^"\r\n]*")?\s*\)`)

func (b *Bridge) sourceItem(ctx context.Context, id, turnID, itemID string) (Session, map[string]any, error) {
	if len(turnID) == 0 || len(turnID) > 256 || len(itemID) == 0 || len(itemID) > 256 {
		return Session{}, nil, errors.New("invalid item identity")
	}
	row, err := b.History.Get(ctx, id)
	if err != nil {
		return row, nil, err
	}
	b.mu.Lock()
	var state State
	if value := b.sessions[id]; value != nil {
		state = *value
	}
	b.mu.Unlock()
	find := func(raw map[string]any) map[string]any {
		for index, value := range orderedTurns(raw) {
			turn, ok := value.(map[string]any)
			if !ok {
				continue
			}
			key := fmtID(turn["turnId"])
			if key == "" {
				key = fmtID(turn["id"])
			}
			if key == "" {
				key = fmt.Sprintf("turn-%d", index)
			}
			if key != turnID {
				continue
			}
			for itemIndex, value := range orderedItems(turn["items"]) {
				item, ok := value.(map[string]any)
				if !ok {
					continue
				}
				key := fmtID(item["id"])
				if key == "" {
					key = fmt.Sprintf("%s:%d", item["type"], itemIndex)
				}
				if key == itemID {
					return item
				}
			}
		}
		return nil
	}
	if item := find(b.withSteers(id, state.snapshotValue())); item != nil {
		return row, item, nil
	}
	raw, err := b.readHistoryValue(ctx, row, true)
	if err != nil {
		return row, nil, err
	}
	if item := find(b.withSteers(id, raw)); item != nil {
		return row, item, nil
	}
	return row, nil, errors.New("item no longer available; refresh history")
}

func imageReferences(item map[string]any) []ImageReference {
	if kind := item["type"]; kind != "userMessage" && kind != "agentMessage" && kind != "imageGenerationCall" && kind != "imageGeneration" {
		return nil
	}
	refs := []ImageReference{}
	seen := map[string]bool{}
	add := func(source string) {
		source = strings.Trim(source, "<> ")
		if source == "" || len(refs) >= 8 || seen[source] {
			return
		}
		if strings.HasPrefix(source, "data:") {
			if !strings.HasPrefix(source, "data:image/") || len(source) > 28<<20 {
				return
			}
		} else {
			if strings.Contains(source, "://") && !strings.HasPrefix(source, "file://") {
				return
			}
			if strings.HasPrefix(source, "#") || strings.HasPrefix(source, "javascript:") {
				return
			}
		}
		seen[source] = true
		name := filepath.Base(strings.ReplaceAll(source, `\`, "/"))
		if strings.HasPrefix(source, "data:") {
			name = "image"
		}
		refs = append(refs, ImageReference{ID: store.Hash(source), Name: name, Source: source})
	}
	text, _ := item["text"].(string)
	if content, ok := item["content"].(string); ok {
		text += "\n" + content
	}
	parts := orderedItems(item["content"])
	if typed, ok := item["content"].([]map[string]any); ok {
		for _, part := range typed {
			parts = append(parts, part)
		}
	}
	for _, part := range parts {
		p, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if value, ok := p["text"].(string); ok {
			text += "\n" + value
		}
		if p["type"] == "localImage" || p["type"] == "local_image" {
			value, _ := p["path"].(string)
			add(value)
		}
		if p["type"] == "image" || p["type"] == "input_image" || p["type"] == "output_image" {
			for _, key := range []string{"url", "image_url", "path"} {
				if value, ok := p[key].(string); ok {
					add(value)
				}
			}
		}
	}
	for _, match := range markdownImage.FindAllStringSubmatch(text, 8) {
		add(match[1])
	}
	if item["type"] == "imageGenerationCall" || item["type"] == "imageGeneration" {
		if saved, ok := item["savedPath"].(string); ok {
			add(saved)
		}
		if len(refs) == 0 {
			if src, ok := item["src"].(string); ok {
				add(src)
			}
		}
		if result, ok := item["result"].(string); ok && len(refs) == 0 && result != "" {
			if !strings.HasPrefix(result, "data:") {
				result = "data:image/png;base64," + result
			}
			add(result)
		}
	}
	return refs
}

func (b *Bridge) Image(ctx context.Context, thread, turn, itemID, id string) ([]byte, string, error) {
	row, item, err := b.sourceItem(ctx, thread, turn, itemID)
	if err != nil {
		return nil, "", err
	}
	source := ""
	for _, ref := range imageReferences(item) {
		if ref.ID == id {
			source = ref.Source
			break
		}
	}
	if source == "" {
		return nil, "", errors.New("image is not referenced by this message")
	}
	var data []byte
	if strings.HasPrefix(source, "data:") {
		_, body, ok := strings.Cut(source, ";base64,")
		if !ok {
			return nil, "", errors.New("unsupported inline image")
		}
		data, err = base64.StdEncoding.DecodeString(body)
	} else {
		if strings.HasPrefix(source, "sandbox:") {
			source = strings.TrimPrefix(source, "sandbox:")
		}
		if strings.HasPrefix(source, "file://") {
			u, e := url.Parse(source)
			if e != nil || u.Host != "" {
				return nil, "", errors.New("image path rejected")
			}
			source = u.Path
			if len(source) > 3 && source[0] == '/' && source[2] == ':' {
				source = source[1:]
			}
		}
		if decoded, e := url.PathUnescape(source); e == nil {
			source = decoded
		}
		if !filepath.IsAbs(source) {
			source = filepath.Join(row.CWD, source)
		}
		roots := []string{row.CWD, filepath.Join(b.config.CodexHome, "generated_images"), filepath.Join(b.config.CodexHome, "artifacts"), filepath.Join(b.config.CodexHome, "visualizations")}
		if b.Uploads != nil {
			roots = append(roots, b.Uploads.Root)
		}
		var file *os.File
		for _, rootPath := range roots {
			if !filepath.IsAbs(rootPath) {
				continue
			}
			rel, e := filepath.Rel(historyComparisonPath(rootPath), historyComparisonPath(source))
			if e != nil || !filepath.IsLocal(rel) {
				continue
			}
			root, e := os.OpenRoot(rootPath)
			if e != nil {
				continue
			}
			file, e = root.Open(rel)
			root.Close()
			if e != nil {
				file = nil
				continue
			}
			break
		}
		if file == nil {
			return nil, "", errors.New("image unavailable or outside allowed roots")
		}
		defer file.Close()
		info, e := file.Stat()
		if e != nil || !info.Mode().IsRegular() || info.Size() > attachments.MaxSize {
			return nil, "", errors.New("image size rejected")
		}
		data, err = io.ReadAll(io.LimitReader(file, attachments.MaxSize+1))
	}
	if err != nil || len(data) == 0 || len(data) > attachments.MaxSize {
		return nil, "", errors.New("image data unavailable")
	}
	mime := http.DetectContentType(data)
	if mime != "image/png" && mime != "image/jpeg" && mime != "image/gif" && mime != "image/webp" {
		return nil, "", errors.New("unsupported image type")
	}
	if mime == "image/webp" {
		width, height := webpSize(data)
		if width < 1 || height < 1 || int64(width)*int64(height) > 40_000_000 {
			return nil, "", errors.New("image dimensions rejected")
		}
	} else {
		dim, _, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || dim.Width < 1 || dim.Height < 1 || int64(dim.Width)*int64(dim.Height) > 40_000_000 {
			return nil, "", errors.New("image dimensions rejected")
		}
	}
	return data, mime, nil
}

func webpSize(data []byte) (int, int) {
	if len(data) < 30 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0
	}
	switch string(data[12:16]) {
	case "VP8X":
		return 1 + int(data[24]) + (int(data[25]) << 8) + (int(data[26]) << 16), 1 + int(data[27]) + (int(data[28]) << 8) + (int(data[29]) << 16)
	case "VP8L":
		if data[20] != 0x2f {
			return 0, 0
		}
		bits := binary.LittleEndian.Uint32(data[21:25])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1
	case "VP8 ":
		if !bytes.Equal(data[23:26], []byte{0x9d, 0x01, 0x2a}) {
			return 0, 0
		}
		return int(binary.LittleEndian.Uint16(data[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(data[28:30]) & 0x3fff)
	}
	return 0, 0
}
