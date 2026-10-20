package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQueueFIFOClaimCancelRecoveryAndRevocation(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := loginFixture(t, s, "queue fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Enqueue(d.ID, "a", "thread", "first", []string{"skill-id"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Enqueue(d.ID, "b", "thread", "second", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimQueued(b); err != nil || ok {
		t.Fatal("queue overtook head", err)
	}
	if ok, err := s.ClaimQueued(a); err != nil || !ok {
		t.Fatal(err)
	}
	if ok, _ := s.ClaimQueued(a); ok {
		t.Fatal("claimed twice")
	}
	if err = s.CancelQueued("thread", a); err == nil {
		t.Fatal("cancelled in-flight IPC")
	}
	if err = s.CancelQueued("other", b); err == nil {
		t.Fatal("cancelled across thread")
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Recover(); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Queue("thread")
	if err != nil || len(rows) != 2 || rows[0].Status != "unknown" || rows[0].Skills[0] != "skill-id" {
		t.Fatal(rows, err)
	}
	raw, _ := json.Marshal(rows)
	if strings.Contains(string(raw), d.ID) || strings.Contains(string(raw), "skill-id") {
		t.Fatal("private metadata projected")
	}
	if ok, _ := s.ClaimQueued(b); ok {
		t.Fatal("unknown head did not pause queue")
	}
	if err = s.CancelQueued("thread", a); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimQueued(b); err != nil || !ok {
		t.Fatal(err)
	}
	if err = s.FinishQueued(b, "sent", ""); err != nil {
		t.Fatal(err)
	}
	if err = s.Recover(); err != nil {
		t.Fatal(err)
	}
	rows, _ = s.Queue("thread")
	if len(rows) != 1 || rows[0].Status != "sent" {
		t.Fatal("replayed accepted message")
	}
	s.ObserveQueued(b)
	rows, _ = s.Queue("thread")
	if len(rows) != 0 {
		t.Fatal(rows)
	}
	s.Revoke(d.ID)
	if _, err = s.Enqueue(d.ID, "revoked", "thread", "no", nil); err == nil {
		t.Fatal("revoked device enqueued")
	}
}
func TestQueueCapacityIsBounded(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, _, err := loginFixture(t, s, "fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		if _, err = s.Enqueue(d.ID, string(rune('a'+i)), "thread", "text", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Enqueue(d.ID, "overflow", "thread", "text", nil); err == nil {
		t.Fatal("unbounded queue")
	}
}
