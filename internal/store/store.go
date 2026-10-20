package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var ErrUnauthorized = errors.New("authorization expired or revoked")
var ErrConflict = errors.New("operation ID already belongs to different content")

type Store struct{ db *sql.DB }
type Device struct {
	LastIP   string `json:"lastIp"`
	LastSeen int64  `json:"lastSeen"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Created  int64  `json:"created"`
	Expires  int64  `json:"expires"`
	Revoked  bool   `json:"revoked"`
}
type Receipt struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
}

func Secret() string {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func Open(dir string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	p := filepath.Join(dir, "cxpalm.sqlite")
	db, e := sql.Open("sqlite", p)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db}
	_, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS notification_events(seq INTEGER PRIMARY KEY AUTOINCREMENT,scope TEXT NOT NULL,id TEXT NOT NULL,payload BLOB NOT NULL,UNIQUE(scope,id));
 CREATE TABLE IF NOT EXISTS notification_checkpoints(scope TEXT NOT NULL,thread TEXT NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(scope,thread));
 CREATE TABLE IF NOT EXISTS queued_messages(seq INTEGER PRIMARY KEY AUTOINCREMENT,id TEXT UNIQUE NOT NULL,device TEXT NOT NULL,thread TEXT NOT NULL,text TEXT NOT NULL,skills BLOB NOT NULL,status TEXT NOT NULL DEFAULT 'queued',error TEXT NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS queued_attachments(message TEXT PRIMARY KEY,ids BLOB NOT NULL);
 CREATE TRIGGER IF NOT EXISTS queued_attachments_cleanup AFTER DELETE ON queued_messages BEGIN DELETE FROM queued_attachments WHERE message=OLD.id; END;
 CREATE TABLE IF NOT EXISTS steer_messages(seq INTEGER PRIMARY KEY AUTOINCREMENT,thread TEXT NOT NULL,id TEXT NOT NULL,payload BLOB NOT NULL,UNIQUE(thread,id));
 CREATE TABLE IF NOT EXISTS thread_reads(thread TEXT PRIMARY KEY,token TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS devices(id TEXT PRIMARY KEY,hash TEXT UNIQUE NOT NULL,name TEXT NOT NULL,created INTEGER NOT NULL,expires INTEGER NOT NULL,revoked INTEGER NOT NULL DEFAULT 0,last_ip TEXT NOT NULL DEFAULT '',last_seen INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS device_events(id INTEGER PRIMARY KEY AUTOINCREMENT,device TEXT NOT NULL,name TEXT NOT NULL,action TEXT NOT NULL,at INTEGER NOT NULL,ip TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS operations(device TEXT NOT NULL,id TEXT NOT NULL,fingerprint TEXT NOT NULL,status TEXT NOT NULL,result BLOB,created INTEGER NOT NULL,PRIMARY KEY(device,id));`)
	if e != nil {
		db.Close()
		return nil, e
	}
	if e = s.migrateDeviceMetadata(); e != nil {
		db.Close()
		return nil, e
	}
	if e = os.Chmod(p, 0600); e != nil {
		db.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Recover() error {
	_, e := s.db.Exec("UPDATE operations SET status='unknown' WHERE status='dispatched'; UPDATE queued_messages SET status='unknown',error='网关曾在发送期间退出，请在官方桌面核对；不会自动重发' WHERE status='sending'")
	return e
}
func (s *Store) Authorize(token string) (Device, error) {
	var d Device
	e := s.db.QueryRow("SELECT id,name,created,expires,revoked,last_ip,last_seen FROM devices WHERE hash=? AND revoked=0 AND expires>?", Hash(token), time.Now().Unix()).Scan(&d.ID, &d.Name, &d.Created, &d.Expires, &d.Revoked, &d.LastIP, &d.LastSeen)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrUnauthorized
	}
	return d, e
}
func (s *Store) Valid(id string) bool {
	var n int
	e := s.db.QueryRow("SELECT COUNT(*) FROM devices WHERE id=? AND revoked=0 AND expires>?", id, time.Now().Unix()).Scan(&n)
	return e == nil && n == 1
}
func (s *Store) Devices() ([]Device, error) {
	rows, e := s.db.Query("SELECT id,name,created,expires,revoked,last_ip,last_seen FROM devices ORDER BY created DESC")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if e = rows.Scan(&d.ID, &d.Name, &d.Created, &d.Expires, &d.Revoked, &d.LastIP, &d.LastSeen); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) Revoke(id string) error { return s.RevokeFrom(id, "") }
func (s *Store) RevokeFrom(id, ip string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var name string
	err = tx.QueryRow("UPDATE devices SET revoked=1 WHERE id=? AND revoked=0 RETURNING name", id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		var count int
		if err = tx.QueryRow("SELECT COUNT(*) FROM devices WHERE id=?", id).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return errors.New("device not found")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO device_events(device,name,action,at,ip) VALUES(?,?,'revoked',?,?)", id, name, time.Now().Unix(), ip); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Begin(device, id, fingerprint string) (Receipt, bool, error) {
	r, e := s.db.Exec("INSERT OR IGNORE INTO operations(device,id,fingerprint,status,created) VALUES(?,?,?,'dispatched',?)", device, id, fingerprint, time.Now().Unix())
	if e != nil {
		return Receipt{}, false, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return Receipt{}, false, e
	}
	var receipt Receipt
	var fp string
	var result []byte
	e = s.db.QueryRow("SELECT id,status,result,fingerprint FROM operations WHERE device=? AND id=?", device, id).Scan(&receipt.ID, &receipt.Status, &result, &fp)
	receipt.Result = result
	if e == nil && receipt.Status != "not_sent" && fp != fingerprint {
		e = ErrConflict
	}
	return receipt, n == 1, e
}
func (s *Store) Finish(device, id, status string, result any) (Receipt, error) {
	b, e := json.Marshal(result)
	if e != nil {
		return Receipt{}, e
	}
	_, e = s.db.Exec("UPDATE operations SET status=?,result=? WHERE device=? AND id=?", status, b, device, id)
	return Receipt{ID: id, Status: status, Result: b}, e
}
func (s *Store) Receipt(device, id string) (Receipt, error) {
	var r Receipt
	var b []byte
	e := s.db.QueryRow("SELECT id,status,result FROM operations WHERE device=? AND id=?", device, id).Scan(&r.ID, &r.Status, &b)
	r.Result = b
	return r, e
}

// Reserve absent IDs before reporting not_sent: a delayed command on another
// connection must never execute after a client has released its pending lock.
func (s *Store) ResolveReceipt(device, id string) (Receipt, error) {
	if id == "" || len(id) > 128 {
		return Receipt{}, errors.New("valid operation id required")
	}
	_, err := s.db.Exec("INSERT OR IGNORE INTO operations(device,id,fingerprint,status,created) VALUES(?,?,'','not_sent',?)", device, id, time.Now().Unix())
	if err != nil {
		return Receipt{}, err
	}
	return s.Receipt(device, id)
}
func (s *Store) Acknowledge(device, id string) (Receipt, error) {
	r, e := s.db.Exec("UPDATE operations SET status='acknowledged' WHERE device=? AND id=? AND status IN ('unknown','not_sent')", device, id)
	if e != nil {
		return Receipt{}, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return Receipt{}, e
	}
	if n == 0 {
		return Receipt{}, errors.New("only an unknown operation can be acknowledged; active dispatch cannot be cleared")
	}
	return s.Receipt(device, id)
}
