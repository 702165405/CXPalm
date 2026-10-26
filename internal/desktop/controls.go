package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Model struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Efforts []string `json:"efforts"`
	Context int      `json:"contextWindow"`
}
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"-"`
}
type Catalog struct {
	Models []Model `json:"models"`
	Skills []Skill `json:"skills"`
	Source string  `json:"source"`
}
type Options struct {
	ExpectedGoal      string `json:"expectedGoal,omitempty"`
	GoalAction        string `json:"goalAction,omitempty"`
	Model             string `json:"model,omitempty"`
	Effort            string `json:"effort,omitempty"`
	Mode              string `json:"mode,omitempty"`
	Permission        string `json:"permission,omitempty"`
	Expected          string `json:"expected,omitempty"`
	ConfirmFullAccess bool   `json:"confirmFullAccess,omitempty"`
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("metadata exceeds safety limit")
	}
	return b, err
}

// Read only known metadata roots, never a phone-supplied filename. No catalog
// request starts/resumes an app-server executor or returns SKILL.md bodies.
func (b *Bridge) Catalog(cwd string) (Catalog, error) {
	c := Catalog{Models: []Model{}, Skills: []Skill{}, Source: "电脑当前模型缓存与已启用的本地 Skill 元数据"}
	raw, err := readBounded(filepath.Join(b.config.CodexHome, "models_cache.json"), 8<<20)
	if err == nil {
		var cache struct {
			Models []struct {
				Slug    string `json:"slug"`
				Name    string `json:"display_name"`
				Context int    `json:"context_window"`
				Levels  []struct {
					Effort string `json:"effort"`
				} `json:"supported_reasoning_levels"`
			} `json:"models"`
		}
		if json.Unmarshal(raw, &cache) == nil {
			for _, m := range cache.Models {
				if m.Slug == "" || len(m.Slug) > 200 {
					continue
				}
				v := Model{ID: m.Slug, Name: m.Name, Context: m.Context, Efforts: []string{}}
				for _, level := range m.Levels {
					v.Efforts = append(v.Efforts, level.Effort)
				}
				c.Models = append(c.Models, v)
			}
		}
	}
	roots := []string{filepath.Join(b.config.CodexHome, "skills"), filepath.Join(cwd, ".agents", "skills"), filepath.Join(cwd, ".codex", "skills")}
	// Plugin configuration is consulted only for enabled names; no credentials or
	// other config values leave this process. Cached but disabled plugins are skipped.
	if raw, err := readBounded(filepath.Join(b.config.CodexHome, "config.toml"), 1<<20); err == nil {
		plugin := ""
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "[") {
				plugin = ""
				if strings.HasPrefix(line, "[plugins.\"") && strings.HasSuffix(line, "\"]") {
					plugin = strings.TrimSuffix(strings.TrimPrefix(line, "[plugins.\""), "\"]")
				}
			}
			if plugin != "" && strings.ReplaceAll(strings.Split(line, "#")[0], " ", "") == "enabled=true" {
				parts := strings.Split(plugin, "@")
				if len(parts) != 2 || !safeName(parts[0]) || !safeName(parts[1]) {
					continue
				}
				dir := filepath.Join(b.config.CodexHome, "plugins", "cache", parts[1], parts[0])
				versions, _ := os.ReadDir(dir)
				var latest string
				var newest int64
				for _, v := range versions {
					if !v.IsDir() {
						continue
					}
					info, e := v.Info()
					if e == nil && (latest == "" || info.ModTime().UnixNano() > newest) {
						latest = v.Name()
						newest = info.ModTime().UnixNano()
					}
				}
				if latest != "" {
					roots = append(roots, filepath.Join(dir, latest, "skills"))
				}
			}
		}
	}
	seen := map[string]bool{}
	count := 0
	for _, root := range roots {
		real, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		filepath.WalkDir(real, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			count++
			if count > 6000 || len(c.Skills) >= 300 {
				return filepath.SkipAll
			}
			if d.IsDir() {
				rel, _ := filepath.Rel(real, path)
				if strings.Count(rel, string(filepath.Separator)) > 5 {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() != "SKILL.md" || d.Type()&os.ModeSymlink != 0 || seen[path] {
				return nil
			}
			raw, e := readBounded(path, 128<<10)
			if e != nil {
				return nil
			}
			lines := strings.Split(string(raw), "\n")
			if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
				return nil
			}
			skill := Skill{Path: path}
			for _, line := range lines[1:] {
				line = strings.TrimSpace(line)
				if line == "---" {
					break
				}
				if strings.HasPrefix(line, "name:") {
					skill.Name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name:")), "\"'")
				}
				if strings.HasPrefix(line, "description:") {
					skill.Description = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), "\"'")
				}
			}
			if skill.Name == "" || len(skill.Name) > 200 {
				return nil
			}
			if len(skill.Description) > 1500 {
				skill.Description = skill.Description[:1500]
			}
			h := sha256.Sum256([]byte(path))
			skill.ID = hex.EncodeToString(h[:])
			seen[path] = true
			c.Skills = append(c.Skills, skill)
			return nil
		})
	}
	sort.Slice(c.Skills, func(i, j int) bool { return c.Skills[i].Name < c.Skills[j].Name })
	return c, nil
}
func safeName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func ControlsRevision(raw map[string]any) string {
	values := map[string]any{}
	for _, key := range []string{"latestModel", "latestReasoningEffort", "latestThreadSettings", "latestCollaborationMode", "currentPermissions"} {
		values[key] = raw[key]
	}
	encoded, _ := json.Marshal(values)
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

func buildSettings(raw map[string]any, o Options, c Catalog) (map[string]any, error) {
	if o.Expected == "" || o.Expected != ControlsRevision(raw) {
		return nil, errors.New("会话设置已改变，请重新打开设置后重试")
	}
	settings := map[string]any{}
	model, _ := raw["latestModel"].(string)
	effort, _ := raw["latestReasoningEffort"].(string)
	if o.Model != "" {
		model = o.Model
		settings["model"] = model
	}
	if o.Model != "" || o.Effort != "" {
		var found *Model
		for i := range c.Models {
			if c.Models[i].ID == model {
				found = &c.Models[i]
				break
			}
		}
		if found == nil {
			return nil, errors.New("模型不在电脑当前模型目录中")
		}
		if o.Model != "" && o.Effort == "" {
			supported := false
			for _, v := range found.Efforts {
				supported = supported || v == effort
			}
			if !supported {
				settings["effort"] = nil
				effort = ""
			}
		}
		if o.Effort != "" {
			if o.Effort == "default" {
				settings["effort"] = nil
				effort = ""
			} else {
				valid := false
				for _, v := range found.Efforts {
					valid = valid || v == o.Effort
				}
				if !valid {
					return nil, errors.New("该模型不支持所选推理强度")
				}
				effort = o.Effort
				settings["effort"] = effort
			}
		}
	}
	if o.Mode != "" {
		if o.Mode != "plan" && o.Mode != "default" {
			return nil, errors.New("不支持的计划模式")
		}
		var e any
		if effort != "" {
			e = effort
		}
		settings["collaborationMode"] = map[string]any{"mode": o.Mode, "settings": map[string]any{"model": model, "reasoning_effort": e, "developer_instructions": nil}}
	}
	if o.Permission != "" {
		id := ""
		switch o.Permission {
		case "read-only":
			id = ":read-only"
		case "workspace", "auto-review":
			id = ":workspace"
		case "full":
			if !o.ConfirmFullAccess {
				return nil, errors.New("完全访问权限需要再次明确确认")
			}
			id = ":danger-full-access"
		default:
			return nil, errors.New("不支持的权限配置")
		}
		settings["permissions"] = id
		settings["approvalPolicy"] = "on-request"
		settings["approvalsReviewer"] = "user"
		if o.Permission == "auto-review" {
			settings["approvalsReviewer"] = "auto_review"
		}
		if o.Permission == "full" {
			settings["approvalPolicy"] = "never"
		}
	}
	if len(settings) == 0 {
		return nil, errors.New("没有需要更改的设置")
	}
	return settings, nil
}
