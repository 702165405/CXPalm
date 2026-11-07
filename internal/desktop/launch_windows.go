//go:build windows

package desktop

import (
	"context"
	"os/exec"
	"syscall"
)

func quietProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func openOfficialChat(ctx context.Context, id string) error {
	if !threadID.MatchString(id) {
		return errPaging
	}
	cmd := exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", "codex://threads/"+id)
	quietProcess(cmd)
	return cmd.Run()
}
