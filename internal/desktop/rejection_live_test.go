package desktop

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"cxpalm/internal/config"
)

// Opt-in, empty CODEX_HOME only. No real chats, resume calls or model turns.
func TestIsolatedArchiveWithLivingWriterIsDefinitelyRejected(t *testing.T) {
	bin := os.Getenv("CXPALM_ISOLATED_METADATA_CLI")
	if bin == "" {
		t.Skip("isolated official CLI integration not requested")
	}
	c := config.Config{CodexHome: t.TempDir(), CodexBin: bin}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	owner := exec.CommandContext(ctx, bin, "app-server", "--listen", "stdio://")
	quietProcess(owner)
	owner.Env = append(os.Environ(), "CODEX_HOME="+c.CodexHome)
	in, err := owner.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := owner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close(); _ = owner.Process.Kill(); _ = owner.Wait() }()
	enc := json.NewEncoder(in)
	scan := bufio.NewScanner(out)
	scan.Buffer(make([]byte, 4096), 16<<20)
	call := func(id int, method string, params any) json.RawMessage {
		t.Helper()
		if err := enc.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
		for scan.Scan() {
			var frame struct {
				ID     *int            `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if json.Unmarshal(scan.Bytes(), &frame) != nil || frame.ID == nil || *frame.ID != id {
				continue
			}
			if len(frame.Error) > 0 {
				t.Fatalf("isolated owner rejected %s: %s", method, frame.Error)
			}
			return frame.Result
		}
		t.Fatal("isolated owner disconnected", scan.Err())
		return nil
	}
	call(1, "initialize", map[string]any{"clientInfo": map[string]any{"name": "cxpalm-test", "version": config.Version}, "capabilities": map[string]any{"experimentalApi": true}})
	_ = enc.Encode(map[string]any{"method": "initialized"})
	raw := call(2, "thread/start", map[string]any{"cwd": t.TempDir(), "ephemeral": false, "historyMode": "paginated"})
	var created struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if json.Unmarshal(raw, &created) != nil || !threadID.MatchString(created.Thread.ID) {
		t.Fatal("invalid isolated thread")
	}
	call(3, "thread/name/set", map[string]any{"threadId": created.Thread.ID, "name": "Isolated archive writer fixture"})
	_, err = metadataCall(ctx, c, "thread/archive", map[string]any{"threadId": created.Thread.ID})
	var rejected *Rejected
	if !errors.As(archiveFailure(err, created.Thread.ID), &rejected) || rejected.Code != "active_writer" {
		t.Fatal("archive must be a known rejection", err)
	}
	row, err := (History{Config: c}).Get(ctx, created.Thread.ID)
	if err != nil || row.Archived {
		t.Fatal("rejected archive changed persisted state", row.Archived, err)
	}
}
