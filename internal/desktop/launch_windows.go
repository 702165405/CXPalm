//go:build windows

package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func quietProcess(cmd *exec.Cmd)                   { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func configureSuggestionProcess(command *exec.Cmd) { quietProcess(command) }
func stopSuggestionProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stop := exec.CommandContext(ctx, filepath.Join(os.Getenv("SystemRoot"), "System32", "taskkill.exe"), "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F")
	quietProcess(stop)
	_ = stop.Run()
	_ = command.Process.Kill()
}
func openOfficialChat(ctx context.Context, id string) error {
	if !threadID.MatchString(id) {
		return errPaging
	}
	cmd := exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", "codex://threads/"+id)
	quietProcess(cmd)
	return cmd.Run()
}
