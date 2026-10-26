package desktop

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Native goal metadata is read-only here. All mutations use the official API;
// only the existing desktop owner may run the activation turn.
func (b *Bridge) nativeGoal(id string) (map[string]any, string, error) {
	path := filepath.Join(b.config.CodexHome, "goals_1.sqlite")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, "none", nil
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p, RawQuery: "mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(200)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, "", err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var goalID, objective, status string
	var budget sql.NullInt64
	var used, seconds, created, updated int64
	err = db.QueryRowContext(ctx, `SELECT goal_id,objective,status,token_budget,tokens_used,time_used_seconds,created_at_ms,updated_at_ms FROM thread_goals WHERE thread_id=?`, id).Scan(&goalID, &objective, &status, &budget, &used, &seconds, &created, &updated)
	if err == sql.ErrNoRows {
		return nil, "none", nil
	}
	if err != nil {
		return nil, "", err
	}
	goal := map[string]any{"goalId": goalID, "objective": objective, "status": status, "tokensUsed": used, "timeUsedSeconds": seconds, "createdAtMs": created, "updatedAtMs": updated}
	if budget.Valid {
		goal["tokenBudget"] = budget.Int64
	}
	identity, _ := json.Marshal([]any{goalID, objective, status, created})
	hash := sha256.Sum256(identity)
	return goal, hex.EncodeToString(hash[:]), nil
}

func (b *Bridge) projectGoal(s State) State {
	if len(s.Snapshot) == 0 {
		return s
	}
	goal, revision, err := b.nativeGoal(s.ID)
	var raw map[string]any
	if json.Unmarshal(s.Snapshot, &raw) != nil {
		return s
	}
	raw["threadGoal"] = goal
	raw["goalRevision"] = revision
	raw["goalAvailable"] = err == nil
	s.Snapshot = mustJSON(raw)
	return s
}

func (b *Bridge) prepareGoal(ctx context.Context, id, objective string, o Options, raw map[string]any) (bool, error) {
	goal, revision, err := b.nativeGoal(id)
	if err != nil {
		return false, errors.New("原生目标状态不可读取，请在官方桌面核对")
	}
	if o.ExpectedGoal == "" || o.ExpectedGoal != revision {
		return false, errors.New("目标状态已改变，请刷新后重试")
	}
	action := o.GoalAction
	if action == "" {
		action = "create"
	}
	activate := action == "create" || action == "resume"
	if status, ok := raw["threadRuntimeStatus"].(map[string]any); activate && ok && status["type"] == "active" {
		return false, errors.New("请等待当前任务完成后开启目标")
	}
	params := map[string]any{"threadId": id}
	method := "thread/goal/set"
	switch action {
	case "create":
		if len(strings.TrimSpace(objective)) == 0 || len(objective) > 16000 {
			return false, errors.New("目标内容需要 1-16000 字节")
		}
		if goal != nil && goal["status"] != "complete" {
			return false, errors.New("已有原生目标，请先暂停或关闭旧目标")
		}
		params["objective"] = objective
		params["status"] = "active"
	case "pause":
		if goal == nil || goal["status"] != "active" {
			return false, errors.New("没有进行中的目标")
		}
		params["status"] = "paused"
	case "resume":
		if goal == nil || goal["status"] != "paused" {
			return false, errors.New("没有已暂停的目标")
		}
		params["status"] = "active"
	case "clear":
		if goal == nil {
			return false, errors.New("没有原生目标")
		}
		method = "thread/goal/clear"
	default:
		return false, errors.New("不支持的目标操作")
	}
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err = metadataCall(callCtx, b.config, method, params); err != nil {
		return false, err
	}
	b.mu.Lock()
	if s := b.sessions[id]; s != nil {
		b.emitLocked(s)
	}
	b.mu.Unlock()
	return activate, nil
}
