//go:build !windows

package desktop

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
)

func quietProcess(cmd *exec.Cmd) {}
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
