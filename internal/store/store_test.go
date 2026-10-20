package store

import (
	"errors"
	"testing"
)

func loginFixture(t *testing.T, s *Store, name, ip string) (Device, string, error) {
	t.Helper()
	status, err := s.LoginSettings()
	if err != nil {
		return Device{}, "", err
	}
	if !status.Configured {
		if _, err = s.SetLogin(LoginInput{Username: "fixture", Password: "fixture-test-password-012345"}); err != nil {
			return Device{}, "", err
		}
	}
	return s.Login("fixture", "fixture-test-password-012345", name, ip, true)
}
func TestLoginAndRevocation(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	d, token, e := loginFixture(t, s, "phone", "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Authorize(token); e != nil {
		t.Fatal(e)
	}
	if e = s.Revoke(d.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Authorize(token); !errors.Is(e, ErrUnauthorized) {
		t.Fatal("revocation failed")
	}
}
func TestUnknownMutationIsNotReplayed(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	_, fresh, e := s.Begin("d", "op", "fp")
	if e != nil || !fresh {
		t.Fatal(e)
	}
	if e = s.Recover(); e != nil {
		t.Fatal(e)
	}
	r, fresh, e := s.Begin("d", "op", "fp")
	if e != nil || fresh || r.Status != "unknown" {
		t.Fatal(r, fresh, e)
	}
	if _, _, e = s.Begin("d", "op", "changed"); !errors.Is(e, ErrConflict) {
		t.Fatal("fingerprint collision accepted")
	}
}
func TestAcknowledgementDoesNotReplay(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.Begin("d", "op", "fp")
	if _, e = s.Acknowledge("d", "op"); e == nil {
		t.Fatal("in-flight mutation cleared")
	}
	s.Finish("d", "op", "unknown", nil)
	if _, e = s.Acknowledge("other-device", "op"); e == nil {
		t.Fatal("another device cleared receipt")
	}
	if _, e = s.Acknowledge("d", "op"); e != nil {
		t.Fatal(e)
	}
	r, fresh, e := s.Begin("d", "op", "fp")
	if e != nil || fresh || r.Status != "acknowledged" {
		t.Fatal("acknowledgement re-enabled replay", r, fresh, e)
	}
}
func TestUnusedHistoricalTableIsNotCreatedOrDeleted(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sidechats'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("new installation created unused historical table")
	}
	if _, err = s.db.Exec("CREATE TABLE sidechats(id TEXT PRIMARY KEY, thread TEXT, project TEXT, created INTEGER); INSERT INTO sidechats VALUES('historical','thread','obsolete',1)"); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.db.QueryRow("SELECT COUNT(*) FROM sidechats WHERE id='historical'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("historical data changed: %d %v", count, err)
	}
}

func TestNewStoreHasNoLinkExchangeTableAndPreservesHistoricalRows(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='pairs'").Scan(&count); err != nil || count != 0 {
		t.Fatal("retired table created", err)
	}
	if _, err = s.db.Exec("CREATE TABLE pairs(hash TEXT PRIMARY KEY,name TEXT,expires INTEGER); INSERT INTO pairs VALUES('historical','old',0)"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.db.QueryRow("SELECT COUNT(*) FROM pairs WHERE hash='historical'").Scan(&count); err != nil || count != 1 {
		t.Fatal("historical data modified", err)
	}
}
