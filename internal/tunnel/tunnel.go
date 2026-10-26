package tunnel

import (
	"context"
	"cxpalm/internal/config"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"os/exec"
	"strconv"
	"time"
)

// Run uses existing OpenSSH authentication and pinned host keys. It never
// installs server software, requests sudo, or accepts phone-supplied SSH options.
func Run(ctx context.Context, c config.Config) {
	delay := time.Second
	for ctx.Err() == nil {
		args := arguments(c)
		cmd := exec.CommandContext(ctx, "ssh", args...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		started := time.Now()
		err := cmd.Run()
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			delay = time.Second
		}
		if err != nil {
			log.Print("SSH reverse tunnel unavailable; check the saved alias, host key, authentication and forwarding permission")
		}
		wait := delay + time.Duration(rand.IntN(1000))*time.Millisecond
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}

// Keep every user-supplied value separate from OpenSSH flags.
func arguments(c config.Config) []string {
	args := []string{"-N", "-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ExitOnForwardFailure=yes", "-o", "ControlMaster=no", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3", "-R", fmt.Sprintf("127.0.0.1:%d:%s", c.SSHRemotePort, c.Listen)}
	if c.SSHUser != "" {
		args = append(args, "-l", c.SSHUser)
	}
	if c.SSHPort != 0 {
		args = append(args, "-p", strconv.Itoa(c.SSHPort))
	}
	return append(args, c.SSHHost)
}
