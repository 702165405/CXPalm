package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIPPolicyValidationAndEnforcement(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !s.IPAllowed("") {
		t.Fatal("default is not off")
	}
	for _, p := range []IPPolicy{{Enabled: true}, {Entries: []string{"example.com"}}, {Entries: []string{"https://127.0.0.1"}}, {Entries: []string{"0.0.0.0/0"}}, {Entries: []string{"::/0"}}, {Entries: []string{"fe80::1%ethernet"}}, {Entries: []string{"invalid"}}} {
		if _, err = s.SetIPPolicy(p); err == nil {
			t.Fatalf("invalid policy accepted: %+v", p)
		}
	}
	p, err := s.SetIPPolicy(IPPolicy{Enabled: true, Entries: []string{"203.0.113.9/24", "203.0.113.0/24", "2001:db8::/48", "::ffff:192.0.2.1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Entries) != 3 {
		t.Fatal("CIDR not canonicalized/deduplicated")
	}
	for _, ip := range []string{"203.0.113.77", "2001:db8::1", "192.0.2.1", "::ffff:192.0.2.1"} {
		if !s.IPAllowed(ip) {
			t.Fatalf("allowed IP rejected: %s", ip)
		}
	}
	for _, ip := range []string{"", "bad", "203.0.114.1", "2001:db9::1", "192.0.2.2"} {
		if s.IPAllowed(ip) {
			t.Fatalf("outside IP accepted: %s", ip)
		}
	}
	if _, err = s.SetIPPolicy(IPPolicy{}); err != nil {
		t.Fatal(err)
	}
	if !s.IPAllowed("outside") {
		t.Fatal("disabled policy still filters")
	}
	if _, err = s.db.Exec("UPDATE settings SET value='broken' WHERE key='ip-policy'"); err != nil {
		t.Fatal(err)
	}
	if s.IPAllowed("203.0.113.1") {
		t.Fatal("corrupt policy failed open")
	}
}
func TestAuditMetadataHasNoCredentialsAndRevocationIsIdempotent(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	device, token, err := loginFixture(t, s, "Phone", "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.TouchDevice(device.ID, "203.0.113.2"); err != nil {
		t.Fatal(err)
	}
	if err = s.TouchDevice(device.ID, "203.0.113.2"); err != nil {
		t.Fatal(err)
	}
	if err = s.Revoke(device.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Revoke(device.ID); err != nil {
		t.Fatal(err)
	}
	events, err := s.DeviceEvents()
	if err != nil {
		t.Fatal(err)
	}
	events = slices.DeleteFunc(events, func(event DeviceEvent) bool { return event.Action == "login_settings_changed" })
	if len(events) != 3 {
		t.Fatalf("expected login/connect/revoke once, got %+v", events)
	}
	raw, _ := json.Marshal(events)
	if strings.Contains(string(raw), token) {
		t.Fatal("audit leaked credential")
	}
	devices, err := s.Devices()
	if err != nil || devices[0].LastIP != "203.0.113.2" || !devices[0].Revoked {
		t.Fatal(devices, err)
	}
}
func TestOldDeviceMigrationPreservesAuthorizationWithoutInventingIPs(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "cxpalm.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("CREATE TABLE devices(id TEXT PRIMARY KEY,hash TEXT UNIQUE NOT NULL,name TEXT NOT NULL,created INTEGER NOT NULL,expires INTEGER NOT NULL,revoked INTEGER NOT NULL DEFAULT 0);INSERT INTO devices VALUES('legacy',?,'Old device',100,9999999999,0)", Hash("legacy-token"))
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	device, err := s.Authorize("legacy-token")
	if err != nil || device.LastIP != "" || device.LastSeen != 0 {
		t.Fatal(device, err)
	}
	events, err := s.DeviceEvents()
	if err != nil || len(events) != 1 || events[0].Action != "legacy_paired" || events[0].At != 100 || events[0].IP != "" {
		t.Fatal(events, err)
	}
}
