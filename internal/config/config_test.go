package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBoundary(t *testing.T) {
	root := t.TempDir()
	c := Config{Listen: "127.0.0.1:18789", Origin: "https://cx.example", DataDir: root, CodexHome: root}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "ipc", "ipc.sock")
	if runtime.GOOS == "windows" {
		want = `\\.\pipe\codex-ipc`
	}
	if c.IPCPath != want {
		t.Fatalf("IPC path: got %q want %q", c.IPCPath, want)
	}
	c.Listen = "0.0.0.0:18789"
	if c.Validate() == nil {
		t.Fatal("public bind accepted")
	}
	c.Listen = "127.0.0.1:18789"
	c.Origin = "https://user:secret@cx.example"
	if c.Validate() == nil {
		t.Fatal("credentials accepted")
	}
}
func TestSSHConfiguration(t *testing.T) {
	root := t.TempDir()
	c := Config{Listen: "127.0.0.1:18789", Origin: "https://cx.example", DataDir: root, CodexHome: root, SSHHost: "-oProxyCommand=bad", SSHRemotePort: 18790}
	if c.Validate() == nil {
		t.Fatal("SSH option injection accepted")
	}
	c.SSHHost = "my-saved-host"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.SSHRemotePort = 80
	if c.Validate() == nil {
		t.Fatal("privileged reverse port accepted")
	}
}
func TestRetiredFieldsHaveNoEffectAndAreNotSerialized(t *testing.T) {
	root := t.TempDir()
	raw, err := json.Marshal(map[string]any{"listen": "127.0.0.1:18789", "origin": "https://cx.example", "dataDir": root, "codexHome": root, "projects": map[string]string{"obsolete": "not-an-existing-directory"}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	clean, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(clean, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["projects"]; exists {
		t.Fatal("retired directory mapping serialized")
	}
}

func TestConnectionSettingsBoundary(t *testing.T) {
	root := t.TempDir()
	valid := Config{Listen: "127.0.0.1:18789", Origin: "https://cx.example:8443/", DataDir: root, CodexHome: root, SSHHost: "::1", SSHUser: "deploy-user", SSHPort: 2222, SSHRemotePort: 18791}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	if valid.Origin != "https://cx.example:8443" {
		t.Fatal("origin not normalized")
	}
	for _, edit := range []func(*Config){
		func(c *Config) { c.Listen = "127.0.0.1:notaport" }, func(c *Config) { c.Listen = "127.0.0.1:65536" },
		func(c *Config) { c.Origin = "http://cx.example" }, func(c *Config) { c.Origin = "https://cx.example/cxpalm/" },
		func(c *Config) { c.Origin = "https://cx.example?" }, func(c *Config) { c.Origin = "https://cx.example#" }, func(c *Config) { c.Origin = "https://cx.example:70000" },
		func(c *Config) { c.SSHHost = "user@host" }, func(c *Config) { c.SSHHost = "host\n-oProxyCommand=bad" },
		func(c *Config) { c.SSHUser = "-oProxyCommand=bad" }, func(c *Config) { c.SSHPort = 65536 },
		func(c *Config) { c.SSHPort = -1 }, func(c *Config) { c.SSHHost = "" },
	} {
		c := valid
		edit(&c)
		if c.Validate() == nil {
			t.Fatalf("invalid connection accepted: %#v", c)
		}
	}
}
