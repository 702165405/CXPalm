package server

import (
	"cxpalm/internal/store"
	"encoding/json"
	"testing"
)

func TestNotificationsRequireExplicitChatOptInForLiveAndReplay(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	feed := notificationFeed{journal: db, scope: "account", checkpoints: map[string]json.RawMessage{}}
	feed.once.Do(func() {})
	received := []Event{}
	stop := feed.subscribe(nil, "", "watch", func(e Event) bool { received = append(received, e); return true })
	defer stop()
	feed.publish(Notification{ID: "muted", Thread: "chat", Kind: "complete"})
	if len(received) != 1 {
		t.Fatal("default chat notified")
	}
	pref, err := db.SetNotificationPreference("account", "chat", true)
	if err != nil {
		t.Fatal(err)
	}
	feed.publish(Notification{ID: "enabled", Thread: "chat", Kind: "complete", Subscription: pref.Generation})
	feed.publish(Notification{ID: "other-chat", Thread: "other", Kind: "complete", Subscription: pref.Generation})
	if len(received) != 2 {
		t.Fatal("explicit opt-in fanout failed")
	}
	db.SetNotificationPreference("account", "chat", false)
	feed.publish(Notification{ID: "disabled", Thread: "chat", Kind: "complete", Subscription: pref.Generation})
	if len(received) != 2 {
		t.Fatal("disabled chat notified")
	}
	var replay Event
	feed.subscribe(nil, "baseline:account", "replay", func(e Event) bool { replay = e; return true })()
	if len(replay.Data.(map[string]any)["events"].([]Notification)) != 0 {
		t.Fatal("disabled chat replayed")
	}
	next, _ := db.SetNotificationPreference("account", "chat", true)
	feed.publish(Notification{ID: "stale-watcher", Thread: "chat", Kind: "complete", Subscription: pref.Generation})
	feed.subscribe(nil, "baseline:account", "replay2", func(e Event) bool { replay = e; return true })()
	if len(replay.Data.(map[string]any)["events"].([]Notification)) != 0 {
		t.Fatal("old generation replayed")
	}
	feed.publish(Notification{ID: "new-generation", Thread: "chat", Kind: "approval", Subscription: next.Generation})
	if len(received) != 3 {
		t.Fatal("new opt-in did not resume")
	}
}
