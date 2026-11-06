package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"cxpalm/internal/config"
)

func TestSuggestionMCPProbe(t *testing.T) {
	if marker := os.Getenv("CXPALM_SUGGESTION_MCP_PROBE"); marker != "" {
		_ = os.WriteFile(marker, []byte("unexpected MCP process"), 0600)
	}
}

func TestIsolatedOfficialSuggestionWithoutExternalModel(t *testing.T) {
	executable := os.Getenv("CXPALM_ISOLATED_SUGGESTION_CLI")
	if executable == "" {
		t.Skip("isolated official CLI with local mock provider not requested")
	}
	home := t.TempDir()
	marker := filepath.Join(home, "unexpected-mcp")
	t.Setenv("CXPALM_SUGGESTION_MCP_PROBE", marker)
	requests := make(chan []string, 2)
	var interactive atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var body struct {
			Tools []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2<<20)).Decode(&body); err != nil {
			t.Error(err)
			response.WriteHeader(400)
			return
		}
		names := []string{}
		for _, tool := range body.Tools {
			name := tool.Name
			if name == "" {
				name = tool.Type
			}
			names = append(names, name)
			if name != "request_user_input" {
				t.Errorf("executable or unknown tool exposed to prediction: %s", name)
			}
		}
		select {
		case requests <- names:
		default:
			t.Error("unexpected follow-up model request")
			response.WriteHeader(400)
			return
		}
		text := `{"text":"Check the remaining tests."}`
		item := map[string]any{"id": "msg_fixture", "type": "message", "status": "completed", "role": "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}}
		if interactive.Load() {
			item = map[string]any{"type": "function_call", "id": "fc_fixture", "call_id": "call_fixture", "name": "request_user_input",
				"arguments": `{"questions":[{"id":"fixture","header":"Fixture","question":"Choose a fixture option?","options":[{"label":"A","description":"Fixture A"},{"label":"B","description":"Fixture B"}]}]}`}
		}
		response.Header().Set("Content-Type", "text/event-stream")
		events := []any{
			map[string]any{"type": "response.created", "response": map[string]any{"id": "resp_fixture", "status": "in_progress"}},
			map[string]any{"type": "response.output_item.done", "output_index": 0, "item": item},
			map[string]any{"type": "response.completed", "response": map[string]any{"id": "resp_fixture", "status": "completed", "output": []any{item},
				"usage": map[string]any{"input_tokens": 10, "output_tokens": 10, "total_tokens": 20, "input_tokens_details": map[string]any{"cached_tokens": 0}, "output_tokens_details": map[string]any{"reasoning_tokens": 0}}}},
		}
		for _, event := range events {
			fmt.Fprintf(response, "data: %s\n\n", mustJSON(event))
		}
	}))
	defer provider.Close()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	configuration := "model_provider=\"fixture\"\nmodel=\"gpt-5.4\"\n[model_providers.fixture]\nname=\"Local fixture\"\nwire_api=\"responses\"\nbase_url=" +
		strconv.Quote(provider.URL+"/v1") + "\nrequires_openai_auth=false\n[mcp_servers.must_not_run]\ncommand=" + strconv.Quote(self) +
		"\nargs=[\"-test.run=^TestSuggestionMCPProbe$\"]\n"
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	text, err := generateSuggestion(ctx, config.Config{CodexBin: executable, CodexHome: home, DataDir: home},
		"00000000-0000-0000-0000-000000000001", "gpt-5.4", `[{"role":"userMessage","text":"Synthetic fixture only."}]`)
	if err != nil || text != "Check the remaining tests." {
		t.Fatalf("native fixture prediction: %q %v", text, err)
	}
	if len(requests) != 1 {
		t.Fatal("prediction did not make exactly one local fixture request", len(requests))
	}
	interactive.Store(true)
	text, err = generateSuggestion(ctx, config.Config{CodexBin: executable, CodexHome: home, DataDir: home},
		"00000000-0000-0000-0000-000000000001", "gpt-5.4", `[{"role":"userMessage","text":"Interactive fixture only."}]`)
	if text != "" || err == nil || len(requests) != 2 {
		t.Fatalf("interactive prediction did not abort without a follow-up request: %q %v requests=%d", text, err, len(requests))
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("configured MCP command was not disabled")
	}
	rollouts, _ := filepath.Glob(filepath.Join(home, "sessions", "*", "*", "*", "*.jsonl"))
	workspaces, _ := filepath.Glob(filepath.Join(home, "composer-*"))
	if len(rollouts) != 0 || len(workspaces) != 0 {
		t.Fatal("ephemeral prediction left a rollout or workspace", rollouts, workspaces)
	}
	t.Log("Official CLI + localhost synthetic provider: separate ephemeral identity, restricted tools, MCP disabled, no persistent rollout; no real model or main chat used.")
}
