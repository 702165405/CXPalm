package store

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAccountPasswordLifetimeRotation(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	oldToken := Secret()
	oldDevice := Device{ID: Secret()[:32]}
	if _, err = s.db.Exec("INSERT INTO devices(id,hash,name,created,expires) VALUES(?,?,?,0,?)", oldDevice.ID, Hash(oldToken), "existing", time.Now().Add(RememberLogin).Unix()); err != nil {
		t.Fatal(err)
	}
	password := "fixture-only-password-012345"
	configured, err := s.SetLogin(LoginInput{Username: "owner", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	public, _ := json.Marshal(configured)
	if strings.Contains(string(public), password) || strings.Contains(string(public), "digest") || strings.Contains(string(public), "salt") {
		t.Fatal("settings exposed secret")
	}
	raw, _ := s.loginRecord()
	if strings.Contains(raw, password) {
		t.Fatal("plaintext password persisted")
	}
	if _, err = s.Authorize(oldToken); err != nil {
		t.Fatal("initial setup revoked existing trusted device")
	}
	for _, values := range [][2]string{{"wrong", password}, {"owner", "wrong"}, {"owner", ""}} {
		if _, _, err = s.Login(values[0], values[1], "Web browser", "127.0.0.1", true); !errors.Is(err, ErrUnauthorized) {
			t.Fatal("incorrect password accepted", err)
		}
	}
	permanent, token, err := s.Login("owner", password, "Web browser", "203.0.113.1", true)
	if err != nil {
		t.Fatal(err)
	}
	if permanent.Expires-permanent.Created != int64(RememberLogin/time.Second) {
		t.Fatal("remember lifetime")
	}
	temporary, tempToken, err := s.Login("owner", password, "Android App", "203.0.113.2", false)
	if err != nil {
		t.Fatal(err)
	}
	if temporary.Expires-temporary.Created != int64(TemporaryLogin/time.Second) {
		t.Fatal("temporary lifetime")
	}
	if _, err = s.Authorize(token); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("UPDATE devices SET expires=? WHERE id=?", time.Now().Unix()-1, temporary.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authorize(tempToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("expired session accepted")
	}
	if _, err = s.SetLogin(LoginInput{Username: "owner", Password: password, Expected: "stale"}); err == nil {
		t.Fatal("stale account write accepted")
	}
	changed, err := s.SetLogin(LoginInput{Username: "owner", Password: password + "-new", Expected: configured.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Revision == configured.Revision || s.Valid(permanent.ID) || s.Valid(oldDevice.ID) {
		t.Fatal("rotation did not revoke all devices")
	}
	if _, _, err = s.Login("owner", password, "Web browser", "", true); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old password accepted")
	}
	if _, _, err = s.Login("owner", password+"-new", "Web browser", "", true); err != nil {
		t.Fatal(err)
	}
}
func TestNoDefaultAccountAndPasswordBounds(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()
	status, err := s.LoginSettings()
	if err != nil || status.Configured || status.Username != "" {
		t.Fatal("default account exists")
	}
	if _, _, err = s.Login("admin", "admin", "Web browser", "", true); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("default password accepted")
	}
	for _, input := range []LoginInput{{Username: "", Password: "long-password-for-test"}, {Username: "owner", Password: "short"}, {Username: "owner", Password: strings.Repeat("x", 257)}} {
		if _, err = s.SetLogin(input); err == nil {
			t.Fatal("invalid credential accepted")
		}
	}
}
