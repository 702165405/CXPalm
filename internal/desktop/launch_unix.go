//go:build !windows

package desktop

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"syscall"
)

func quietProcess(cmd *exec.Cmd) {}
func configureSuggestionProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
func stopSuggestionProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
}
func openOfficialChat(ctx context.Context, id string) error {
	if !threadID.MatchString(id) {
		return errors.New("invalid thread ID")
	}
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	return exec.CommandContext(ctx, command, "codex://threads/"+id).Run()
}
