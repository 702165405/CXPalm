package store

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const loginKey = "login-account-v1"
const passwordIterations = 600000
const RememberLogin = 30 * 24 * time.Hour
const TemporaryLogin = 24 * time.Hour

type LoginSettings struct {
	Configured bool   `json:"configured"`
	Username   string `json:"username"`
	Revision   string `json:"revision"`
}
type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Expected string `json:"expected"`
}
type loginRecord struct {
	Username string `json:"username"`
	Salt     string `json:"salt"`
	Digest   string `json:"digest"`
	Revision string `json:"revision"`
}

func loginRevision(raw string) string {
	if raw == "" {
		return ""
	}
	return Hash(raw)
}
func (s *Store) loginRecord() (string, error) {
	var raw string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key=?", loginKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return raw, err
}
func (s *Store) LoginSettings() (LoginSettings, error) {
	raw, err := s.loginRecord()
	if err != nil {
		return LoginSettings{}, err
	}
	if raw == "" {
		return LoginSettings{}, nil
	}
	var record loginRecord
	if json.Unmarshal([]byte(raw), &record) != nil {
		return LoginSettings{}, errors.New("Cannot read login settings")
	}
	return LoginSettings{true, record.Username, loginRevision(raw)}, nil
}
func validLoginName(name string) bool {
	if name == "" || len(name) > 80 || strings.TrimSpace(name) != name || !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (s *Store) SetLogin(input LoginInput) (LoginSettings, error) {
	if !validLoginName(input.Username) || !utf8.ValidString(input.Password) || utf8.RuneCountInString(input.Password) < 12 || len(input.Password) > 256 || strings.TrimSpace(input.Password) == "" {
		return LoginSettings{}, errors.New("Use an account name up to 80 bytes and a password of 12 or more characters (up to 256 bytes)")
	}
	salt, _ := hex.DecodeString(Secret())
	digest, err := pbkdf2.Key(sha256.New, input.Password, salt, passwordIterations, 32)
	if err != nil {
		return LoginSettings{}, errors.New("Cannot secure password")
	}
	record := loginRecord{input.Username, hex.EncodeToString(salt), hex.EncodeToString(digest), Secret()}
	raw, err := json.Marshal(record)
	if err != nil {
		return LoginSettings{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return LoginSettings{}, err
	}
	defer tx.Rollback()
	var previous string
	err = tx.QueryRow("SELECT value FROM settings WHERE key=?", loginKey).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return LoginSettings{}, err
	}
	if loginRevision(previous) != input.Expected {
		return LoginSettings{}, errors.New("Login settings changed; reload before saving")
	}
	if previous != "" {
		// Password rotation and device revocation commit together, including active sockets.
		if _, err = tx.Exec("INSERT INTO device_events(device,name,action,at,ip) SELECT id,name,'revoked',?,'' FROM devices WHERE revoked=0", time.Now().Unix()); err != nil {
			return LoginSettings{}, err
		}
		if _, err = tx.Exec("UPDATE devices SET revoked=1 WHERE revoked=0"); err != nil {
			return LoginSettings{}, err
		}
	}
	if _, err = tx.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", loginKey, string(raw)); err != nil {
		return LoginSettings{}, err
	}
	if _, err = tx.Exec("INSERT INTO device_events(device,name,action,at,ip) VALUES('','','login_settings_changed',?,'')", time.Now().Unix()); err != nil {
		return LoginSettings{}, err
	}
	if err = tx.Commit(); err != nil {
		return LoginSettings{}, err
	}
	return LoginSettings{true, record.Username, loginRevision(string(raw))}, nil
}
func (s *Store) Login(username, password, name, ip string, remember bool) (Device, string, error) {
	if len(username) > 80 || len(password) > 256 || !validLoginName(name) {
		return Device{}, "", ErrUnauthorized
	}
	raw, err := s.loginRecord()
	if err != nil {
		return Device{}, "", err
	}
	var record loginRecord
	// Invalid/absent accounts take the same KDF path, without exposing account existence.
	valid := raw != "" && json.Unmarshal([]byte(raw), &record) == nil
	salt, saltErr := hex.DecodeString(record.Salt)
	wanted, digestErr := hex.DecodeString(record.Digest)
	valid = valid && saltErr == nil && len(salt) == 32 && digestErr == nil && len(wanted) == 32
	if !valid {
		salt = make([]byte, 32)
		wanted = make([]byte, 32)
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	if err != nil {
		return Device{}, "", ErrUnauthorized
	}
	userHash, wantedUser := sha256.Sum256([]byte(username)), sha256.Sum256([]byte(record.Username))
	userOK := subtle.ConstantTimeCompare(userHash[:], wantedUser[:])
	passwordOK := subtle.ConstantTimeCompare(actual, wanted)
	if !valid || userOK != 1 || passwordOK != 1 {
		return Device{}, "", ErrUnauthorized
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Device{}, "", err
	}
	defer tx.Rollback()
	var latest string
	if err = tx.QueryRow("SELECT value FROM settings WHERE key=?", loginKey).Scan(&latest); err != nil {
		return Device{}, "", ErrUnauthorized
	}
	if latest != raw {
		return Device{}, "", ErrUnauthorized
	}
	lifetime := TemporaryLogin
	if remember {
		lifetime = RememberLogin
	}
	token := Secret()
	device := Device{ID: Secret()[:32], Name: name, Created: time.Now().Unix(), Expires: time.Now().Add(lifetime).Unix(), LastIP: ip}
	if _, err = tx.Exec("INSERT INTO devices(id,hash,name,created,expires,last_ip) VALUES(?,?,?,?,?,?)", device.ID, Hash(token), device.Name, device.Created, device.Expires, ip); err != nil {
		return Device{}, "", err
	}
	if _, err = tx.Exec("INSERT INTO device_events(device,name,action,at,ip) VALUES(?,?,'login',?,?)", device.ID, device.Name, device.Created, ip); err != nil {
		return Device{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return Device{}, "", err
	}
	return device, token, nil
}
