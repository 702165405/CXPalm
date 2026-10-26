package control

import (
	"bytes"
	"context"
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) *Host {
	t.Helper()
	home := t.TempDir()
	db, err := store.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	h := New(config.Config{Listen: address, Origin: "https://cx.example", DataDir: home, CodexHome: home, IPCPath: filepath.Join(home, "nonexistent-ipc")}, db)
	t.Cleanup(func() { h.Close(); db.Close() })
	history, err := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = history.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER);
 INSERT INTO threads VALUES('desktop','fixture','missing-dir',1,'vscode','Codex Desktop',0);`)
	history.Close()
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func TestOwnedLifecycleAndExternalPortSafety(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	// An unrelated listener must neither be stopped nor trigger receipt recovery.
	listener, err := net.Listen("tcp", h.config.Listen)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	if _, _, err = h.db.Begin("device", "operation", "fingerprint"); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Execute(ctx, Request{Method: "start"}); err == nil {
		t.Fatal("occupied listener accepted")
	}
	receipt, err := h.db.Receipt("device", "operation")
	if err != nil || receipt.Status != "dispatched" {
		t.Fatalf("external operation changed: %+v %v", receipt, err)
	}
	if _, err = h.Execute(ctx, Request{Method: "stop"}); err == nil {
		t.Fatal("external stop accepted")
	}
	listener.Close()
	for i := 0; i < 2; i++ {
		data, err := h.Execute(ctx, Request{Method: "start"})
		if err != nil {
			t.Fatal(err)
		}
		if data.(map[string]any)["state"] != "running" {
			t.Fatal(data)
		}
	}
	response, err := http.Get("http://" + h.config.Listen + "/health")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	h.Close()
	if conn, err := net.Dial("tcp", h.config.Listen); err == nil {
		conn.Close()
		t.Fatal("owned listener survived close")
	}
}
func TestRevocationAndProtocolBoundary(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	_, err := h.db.SetLogin(store.LoginInput{Username: "fixture", Password: "fixture-test-password-012345"})
	if err != nil {
		t.Fatal(err)
	}
	device, _, err := h.db.Login("fixture", "fixture-test-password-012345", "Test device", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.Execute(ctx, Request{Method: "devices.revoke", Device: device.ID}); err != nil {
		t.Fatal(err)
	}
	if h.db.Valid(device.ID) {
		t.Fatal("revoked device remains valid")
	}
	if _, err = h.Execute(ctx, Request{Method: "devices.revoke", Device: "../../file"}); err == nil {
		t.Fatal("invalid ID accepted")
	}
	for _, method := range []string{"files.read", "terminal.start", "git.run", "chat.send", "config.write", "rpc", "pair.create", "pair.status", "pair.revoke"} {
		if _, err = h.Execute(ctx, Request{Method: method}); err == nil {
			t.Fatalf("unsupported operation accepted: %s", method)
		}
	}
	input := strings.NewReader("{\"id\":\"bad\",\"method\":\"status\",\"path\":\"secret\"}\n{\"id\":\"ok\",\"method\":\"status\"}\n")
	var output bytes.Buffer
	if err = Serve(ctx, input, &output, h); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var bad, good Response
	if err = decoder.Decode(&bad); err != nil {
		t.Fatal(err)
	}
	if bad.Error == "" {
		t.Fatal("extra field accepted")
	}
	if err = decoder.Decode(&good); err != nil || good.ID != "ok" || good.Error != "" {
		t.Fatalf("valid request: %+v %v", good, err)
	}
}
func TestPolicyManagementDoesNotClaimExternalOrUntrustedProxyEnforcement(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	h.config.SSHHost = "saved-host"
	if _, err := h.Execute(ctx, Request{Method: "ip-policy.set", Policy: &store.IPPolicy{Enabled: true, Entries: []string{"203.0.113.1"}}}); err == nil {
		t.Fatal("unconfigured trusted proxy accepted")
	}
	h.config.TrustedProxyIP = true
	listener, err := net.Listen("tcp", h.config.Listen)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if _, err = h.Execute(ctx, Request{Method: "ip-policy.set", Policy: &store.IPPolicy{Enabled: true, Entries: []string{"203.0.113.1"}}}); err == nil {
		t.Fatal("external Agent enforcement claimed")
	}
	policy, err := h.db.IPPolicy()
	if err != nil || policy.Enabled {
		t.Fatal("failed policy operation changed stored policy")
	}
	if _, err = h.Execute(ctx, Request{Method: "ip-policy.set", Policy: &store.IPPolicy{}}); err != nil {
		t.Fatal(err)
	}
}

func TestLoginSettingsAreLocalAndBounded(t *testing.T) {
	h := fixture(t)
	ctx := context.Background()
	value, err := h.Execute(ctx, Request{Method: "login.get"})
	if err != nil || value.(store.LoginSettings).Configured {
		t.Fatal("default account exists")
	}
	input := &store.LoginInput{Username: "owner", Password: "fixture-local-password"}
	if _, err = h.Execute(ctx, Request{Method: "status", Login: input}); err == nil {
		t.Fatal("login fields accepted by other method")
	}
	value, err = h.Execute(ctx, Request{Method: "login.set", Login: input})
	if err != nil || !value.(store.LoginSettings).Configured {
		t.Fatal("local setup failed", err)
	}
	encoded, _ := json.Marshal(value)
	if strings.Contains(string(encoded), input.Password) || strings.Contains(string(encoded), "salt") {
		t.Fatal("credentials leaked")
	}
}
