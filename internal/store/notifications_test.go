package store

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestNotificationJournalSurvivesRestartAndIsBounded(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 260; i++ {
		if err := s.SaveNotification("account-a", Notification{ID: fmt.Sprint(i), Kind: "complete", Thread: "chat", Time: int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveNotification("account-b", Notification{ID: "private", Title: "Other account"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveNotificationCheckpoint("account-a", "chat", []byte(`{"initialized":true,"active":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	events, err := s.Notifications("account-a")
	if err != nil || len(events) != 256 || events[0].ID != "4" || events[255].ID != "259" {
		t.Fatalf("journal lost/boundary failed: %d %v", len(events), err)
	}
	if err := s.SaveNotification("account-a", events[0]); err != nil {
		t.Fatal(err)
	}
	events, _ = s.Notifications("account-a")
	if len(events) != 256 || events[255].ID != "259" {
		t.Fatal("duplicate reordered journal")
	}
	cp, err := s.NotificationCheckpoints("account-a")
	if err != nil || !json.Valid(cp["chat"]) {
		t.Fatal("checkpoint lost", err)
	}
	other, err := s.NotificationCheckpoints("account-b")
	if err != nil || len(other) != 0 {
		t.Fatal("account checkpoint leak", err)
	}
}
