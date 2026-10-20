package tunnel

import (
	"cxpalm/internal/config"
	"slices"
	"testing"
)

func TestArguments(t *testing.T) {
	c := config.Config{Listen: "127.0.0.1:18789", SSHHost: "remote-host", SSHUser: "deploy", SSHPort: 2222, SSHRemotePort: 18791}
	args := arguments(c)
	want := []string{"-l", "deploy", "-p", "2222", "remote-host"}
	if !slices.Equal(args[len(args)-len(want):], want) {
		t.Fatalf("SSH values not separate: %q", args)
	}
	if !slices.Contains(args, "StrictHostKeyChecking=yes") || !slices.Contains(args, "BatchMode=yes") || !slices.Contains(args, "127.0.0.1:18791:127.0.0.1:18789") {
		t.Fatal("safe tunnel options lost")
	}
	c.SSHUser = ""
	c.SSHPort = 0
	args = arguments(c)
	if slices.Contains(args, "-l") || slices.Contains(args, "-p") || args[len(args)-1] != "remote-host" {
		t.Fatal("OpenSSH defaults not preserved")
	}
}
