package desktop

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cxpalm/internal/attachments"
	"cxpalm/internal/store"
)

type FileReference struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Source string `json:"-"`
}

var fileCitation = regexp.MustCompile(`:codex-file-citation\{[^{}\r\n]+\}`)
var citationPath = regexp.MustCompile(`(?:^|[ \t])path="([^"\r\n]+)"`)
var markdownFile = regexp.MustCompile(`!?\[[^\]\r\n]{0,512}\]\(\s*(<[^>\r\n]+>|[^\s\r\n]+?)(?:\s+"[^"\r\n]*")?\s*\)`)

func outputPath(source string) (string, string) {
	source = strings.Trim(source, "<> ")
	source = strings.TrimPrefix(source, "sandbox:")
	if strings.HasPrefix(source, "file://") {
		u, err := url.Parse(source)
		if err != nil || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
			return "", ""
		}
		source = u.EscapedPath()
	}
	var err error
	source, err = url.PathUnescape(source)
	if len(source) > 3 && source[0] == '/' && source[2] == ':' {
		source = source[1:]
	}
	if err != nil || source == "" || len(source) > 4096 || strings.ContainsAny(source, "\x00\r\n?#") {
		return "", ""
	}
	path := strings.ReplaceAll(source, `\`, "/")
	parts := strings.Split(filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))), "/")
	for index, part := range parts {
		lower := strings.ToLower(part)
		if strings.Contains(part, ":") && !(index == 0 && len(part) == 2 && part[1] == ':') {
			return "", ""
		}
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			if lower != ".codex" || index+1 == len(parts) || (parts[index+1] != "artifacts" && parts[index+1] != "visualizations") {
				return "", ""
			}
		}
		switch lower {
		case "node_modules", "auth.json", "credentials.json", "cxpalm.json", "desktop-preferences.json":
			return "", ""
		}
	}
	kind := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	switch kind {
	case "xlsx", "xls", "ods", "csv", "tsv", "pdf", "docx", "doc", "odt", "pptx", "ppt", "odp", "txt", "md", "json", "html", "htm", "zip":
		return filepath.FromSlash(path), kind
	}
	return "", ""
}

func citationSource(token string) string {
	if len(token) > 8192 {
		return ""
	}
	match := citationPath.FindStringSubmatch(strings.TrimSuffix(strings.TrimPrefix(token, ":codex-file-citation{"), "}"))
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func fileReferences(item map[string]any) []FileReference {
	if item["type"] != "agentMessage" {
		return nil
	}
	text := userItemText(item)
	refs := []FileReference{}
	seen := map[string]bool{}
	add := func(source string) {
		path, kind := outputPath(source)
		if path == "" || seen[path] || len(refs) >= 16 {
			return
		}
		seen[path] = true
		refs = append(refs, FileReference{ID: store.Hash(path), Name: filepath.Base(path), Kind: kind, Source: path})
	}
	for _, token := range fileCitation.FindAllString(text, 32) {
		add(citationSource(token))
	}
	for _, match := range markdownFile.FindAllStringSubmatch(text, 64) {
		if !strings.HasPrefix(match[0], "!") {
			add(match[1])
		}
	}
	return refs
}

func fileDisplayText(text string) string {
	return fileCitation.ReplaceAllStringFunc(text, func(token string) string {
		if path, _ := outputPath(citationSource(token)); path != "" {
			return filepath.Base(path)
		}
		return token
	})
}

func (b *Bridge) File(ctx context.Context, thread, turn, itemID, id string) ([]byte, string, error) {
	row, item, err := b.sourceItem(ctx, thread, turn, itemID)
	if err != nil {
		return nil, "", err
	}
	source := ""
	for _, ref := range fileReferences(item) {
		if ref.ID == id {
			source = ref.Source
			break
		}
	}
	if source == "" {
		return nil, "", errors.New("file is not referenced by this message")
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(row.CWD, source)
	}
	roots := []string{row.CWD, filepath.Join(b.config.CodexHome, "artifacts"), filepath.Join(b.config.CodexHome, "visualizations")}
	for _, rootPath := range roots {
		if !filepath.IsAbs(rootPath) {
			continue
		}
		rel, err := filepath.Rel(historyComparisonPath(rootPath), historyComparisonPath(source))
		if err != nil || !filepath.IsLocal(rel) {
			continue
		}
		// The rooted open prevents a referenced symlink from escaping this chat.
		root, err := os.OpenRoot(rootPath)
		if err != nil {
			continue
		}
		file, err := root.Open(rel)
		root.Close()
		if err != nil {
			continue
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > attachments.MaxSize {
			file.Close()
			return nil, "", errors.New("file type or size rejected")
		}
		data, err := io.ReadAll(io.LimitReader(file, attachments.MaxSize+1))
		file.Close()
		if err != nil || len(data) > attachments.MaxSize || ctx.Err() != nil {
			return nil, "", errors.New("file read failed")
		}
		return data, filepath.Base(source), nil
	}
	return nil, "", errors.New("file unavailable or outside allowed roots")
}
