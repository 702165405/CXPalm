package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func projectlessBase() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "", errors.New("无法确定无项目会话的工作目录")
	}
	return filepath.Join(home, "Documents", "Codex", "CXPalm"), nil
}
func isPalmProjectless(cwd string) bool {
	base, err := projectlessBase()
	if err != nil {
		return false
	}
	base = projectComparisonPath(base)
	cwd = projectComparisonPath(cwd)
	leaf := strings.TrimPrefix(cwd, base+"/")
	return leaf != cwd && strings.HasPrefix(leaf, "chat-") && !strings.Contains(leaf, "/")
}
func newProjectlessDirectory(base string) (string, error) {
	if !filepath.IsAbs(base) {
		return "", errors.New("无项目工作目录必须为绝对路径")
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, "chat-")
}
func (b *Bridge) newChatDirectory(ctx context.Context, project string) (string, error) {
	if project == "" || project == "none" {
		base, err := projectlessBase()
		if err != nil {
			return "", err
		}
		return newProjectlessDirectory(base)
	}
	// A client may select a known project, never an arbitrary filesystem path.
	cwd := ""
	if p, ok := b.History.projects().Projects[project]; ok && len(p.Roots) > 0 {
		cwd = p.Roots[0]
	}
	if cwd == "" && strings.HasPrefix(project, "cwd:") {
		for offset := 0; ; offset += 500 {
			rows, err := b.History.ListPage(ctx, "", false, 500, offset)
			if err != nil {
				return "", err
			}
			for _, row := range rows {
				if "cwd:"+row.CWD == project {
					cwd = row.CWD
					break
				}
			}
			if cwd != "" || len(rows) < 500 {
				break
			}
		}
	}
	if cwd == "" || !filepath.IsAbs(cwd) {
		return "", errors.New("项目已不可用，请刷新项目列表或选择无项目")
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return "", errors.New("项目目录不可用，请选择其他项目或无项目")
	}
	return cwd, nil
}
func (b *Bridge) openCreatedChat(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(b.ctx, 8*time.Second)
		defer cancel()
		if err := openOfficialChat(ctx, id); err != nil {
			b.mu.Lock()
			defer b.mu.Unlock()
			state := b.sessions[id]
			if state == nil {
				state = &State{ID: id}
				b.sessions[id] = state
			}
			if !state.Connected {
				state.Error = "会话已创建，但未能打开官方桌面。请在电脑打开此会话，不要重复创建。"
				b.emitLocked(state)
			}
		}
	}()
}
