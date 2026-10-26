package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func validateNewChatOptions(o Options, catalog Catalog) error {
	if o.Mode != "" || o.Expected != "" || o.ExpectedGoal != "" || o.GoalAction != "" {
		return errors.New("新建会话仅支持模型、推理强度和权限设置")
	}
	if o.Model == "" && o.Effort == "" && o.Permission == "" {
		return nil
	}
	raw := map[string]any{}
	o.Expected = ControlsRevision(raw)
	_, err := buildSettings(raw, o, catalog)
	return err
}

func (b *Bridge) CreateChat(ctx context.Context, project string, options Options) (map[string]any, error) {
	catalog, err := b.Catalog("")
	if err != nil {
		return nil, err
	}
	if err = validateNewChatOptions(options, catalog); err != nil {
		return nil, &Rejected{Code: "invalid_new_chat_settings", Message: err.Error()}
	}
	cwd, err := b.newChatDirectory(ctx, project)
	if err != nil {
		return nil, &Rejected{Code: "invalid_new_chat_project", Message: err.Error()}
	}
	result, err := metadataCall(ctx, b.config, "thread/start", map[string]any{"cwd": cwd, "ephemeral": false})
	if err != nil {
		return nil, err
	}
	var data struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if json.Unmarshal(result, &data) != nil || !threadID.MatchString(data.Thread.ID) {
		return nil, errors.New("cannot verify newly created chat; inspect desktop before retrying")
	}
	// Verify persistence before asking the original desktop owner to open it.
	if _, err := metadataCall(ctx, b.config, "thread/read", map[string]any{"threadId": data.Thread.ID, "includeTurns": true}); err != nil {
		return map[string]any{"id": data.Thread.ID, "desktopReady": false, "opening": false, "message": "会话已创建，但历史持久化未确认，请核对桌面，不要重复创建。"}, nil
	}
	b.openCreatedChat(data.Thread.ID)
	if options.Model != "" || options.Effort != "" || options.Permission != "" {
		// Empty-thread startup overrides are not durable. Only the original owner
		// can persist these settings; a timeout must not replay this write later.
		err = b.initializeCreatedChat(ctx, data.Thread.ID, options)
		if err != nil {
			return map[string]any{"id": data.Thread.ID, "desktopReady": false, "opening": true, "settingsApplied": false, "message": "会话已创建，但沿用设置未确认；请在会话中核对模型、推理强度和权限，不要重复创建。"}, nil
		}
		return map[string]any{"id": data.Thread.ID, "desktopReady": true, "settingsApplied": true}, nil
	}
	return map[string]any{"id": data.Thread.ID, "desktopReady": false, "opening": true}, nil
}

func (b *Bridge) initializeCreatedChat(ctx context.Context, id string, options Options) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	states, stop, err := b.Subscribe(ctx, id)
	if err != nil {
		return err
	}
	defer stop()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-b.done:
			return errors.New("bridge closed")
		case <-tick.C:
			b.followAsync(id)
		case state := <-states:
			if !state.Connected || state.Owner == "" {
				continue
			}
			raw := state.snapshotValue()
			if raw == nil {
				continue
			}
			if len(orderedTurns(raw)) > 0 || activeTurn(raw) != "" {
				return errors.New("new chat is no longer empty; settings not dispatched")
			}
			options.Expected = ControlsRevision(raw)
			options.newChatOnly = true
			result, err := b.ControlAction(ctx, id, "new-chat-settings:"+id, "chat.settings", "", "", nil, options, nil)
			if err != nil {
				return err
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				return err
			}
			var reply struct {
				Applied bool `json:"applied"`
			}
			if json.Unmarshal(encoded, &reply) != nil || !reply.Applied {
				return errors.New("desktop did not confirm new chat settings")
			}
			return nil
		}
	}
}

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
