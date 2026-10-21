package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"cxpalm/internal/desktop"
	"cxpalm/internal/store"
)

type Notification = store.Notification
type notificationTracker struct {
	Generation  string `json:"generation,omitempty"`
	Initialized bool   `json:"initialized"`
	sequence    uint64
	Active      bool            `json:"active"`
	Requests    map[string]bool `json:"requests"`
}

func (t *notificationTracker) observe(s desktop.State, title string) []Notification {
	s = desktop.NotificationState(s)
	if !s.Connected || len(s.Snapshot) == 0 || s.Sequence < t.sequence {
		return nil
	}
	var raw struct {
		Runtime struct {
			Type string `json:"type"`
		} `json:"threadRuntimeStatus"`
		Requests []struct {
			ID any `json:"id"`
		} `json:"requests"`
	}
	if json.Unmarshal(s.Snapshot, &raw) != nil {
		return nil
	}
	if raw.Runtime.Type != "active" && raw.Runtime.Type != "idle" {
		return nil
	}
	requests := map[string]bool{}
	result := []Notification{}
	emit := func(kind, key string) {
		hash := sha256.Sum256([]byte(s.ID + "\x00" + kind + "\x00" + key))
		label := []rune(title)
		if len(label) > 160 {
			label = label[:160]
		}
		result = append(result, Notification{ID: hex.EncodeToString(hash[:]), Kind: kind, Thread: s.ID, Title: string(label), Time: time.Now().Unix()})
	}
	if t.Initialized && t.Active && raw.Runtime.Type == "idle" {
		// Use the exact completed reply identity so reloading history does not duplicate an alert.
		key := s.ReadToken
		if key == "" {
			sum := sha256.Sum256(s.Snapshot)
			key = hex.EncodeToString(sum[:])
		}
		emit("complete", key)
	}
	for _, request := range raw.Requests {
		if request.ID == nil {
			continue
		}
		id := fmt.Sprint(request.ID)
		requests[id] = true
		if t.Initialized && !t.Requests[id] {
			emit("approval", id)
		}
	}
	t.Initialized = true
	t.sequence = s.Sequence
	t.Active = raw.Runtime.Type == "active"
	t.Requests = requests
	return result
}

type notificationFeed struct {
	once          sync.Once
	mu            sync.Mutex
	listeners     map[uint64]func(Event) bool
	next          uint64
	recent        []Notification
	lastError     string
	journalFailed bool
	journal       *store.Store
	scope         string
	checkpoints   map[string]json.RawMessage
}

func (f *notificationFeed) allowedLocked(event Notification) bool {
	if f.journal == nil {
		return true
	}
	pref, err := f.journal.NotificationPreference(f.scope, event.Thread)
	return err == nil && pref.Enabled && pref.Generation == event.Subscription
}

func (f *notificationFeed) syncJournal(s *Server) error {
	if s == nil {
		return nil
	}
	scope, err := s.Bridge.NotificationScope()
	if err != nil {
		f.mu.Lock()
		// Invalidate only the in-memory view. Never replay a previous account's
		// journal while the current identity cannot be verified.
		f.journal, f.scope, f.recent, f.checkpoints = nil, "", nil, nil
		f.mu.Unlock()
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.journal != nil && f.scope == scope {
		return nil
	}
	recent, err := s.Store.Notifications(scope)
	if err != nil {
		return err
	}
	checkpoints, err := s.Store.NotificationCheckpoints(scope)
	if err != nil {
		return err
	}
	f.journal = s.Store
	f.scope = scope
	f.recent = recent
	f.checkpoints = checkpoints
	return nil
}
func (f *notificationFeed) headLocked() string {
	if len(f.recent) > 0 {
		return f.recent[len(f.recent)-1].ID
	}
	return "baseline:" + f.scope
}
func (f *notificationFeed) checkpoint(thread string) (notificationTracker, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var tracker notificationTracker
	_ = json.Unmarshal(f.checkpoints[thread], &tracker)
	return tracker, f.scope
}
func (f *notificationFeed) saveCheckpoint(thread, scope string, tracker notificationTracker) {
	raw, _ := json.Marshal(tracker)
	f.mu.Lock()
	defer f.mu.Unlock()
	if scope != f.scope || f.journal == nil || string(raw) == string(f.checkpoints[thread]) {
		return
	}
	if err := f.journal.SaveNotificationCheckpoint(scope, thread, raw); err != nil {
		f.journalFailed = true
		f.statusLocked("提醒记录同步暂不可用，正在重试")
		return
	}
	f.checkpoints[thread] = raw
	if f.journalFailed {
		f.journalFailed = false
		f.statusLocked("")
	}
}
func (f *notificationFeed) subscribe(s *Server, cursor, id string, send func(Event) bool) func() {
	if err := f.syncJournal(s); err != nil {
		f.status("提醒记录同步暂不可用，正在重试")
	}
	f.mu.Lock()
	if f.listeners == nil {
		f.listeners = map[uint64]func(Event) bool{}
	}
	f.next++
	key := f.next
	f.listeners[key] = send
	events := []Notification{}
	resynced := false
	if cursor != "" {
		found := -1
		for i, event := range f.recent {
			if event.ID == cursor {
				found = i
				break
			}
		}
		if cursor == "baseline:"+f.scope {
			events = append(events, f.recent...)
		} else if found >= 0 {
			events = append(events, f.recent[found+1:]...)
		} else {
			resynced = true
		}
	}
	filtered := events[:0]
	for _, event := range events {
		if f.allowedLocked(event) {
			filtered = append(filtered, event)
		}
	}
	events = filtered
	// Queue replay before allowing a live event to overtake it.
	send(Event{ID: id, Type: "result", Data: map[string]any{"events": events, "gap": false, "resynced": resynced, "cursor": f.headLocked(), "scope": "opted_in_chats", "error": f.lastError}})
	f.mu.Unlock()
	f.once.Do(func() { go f.run(s) })
	return func() { f.mu.Lock(); delete(f.listeners, key); f.mu.Unlock() }
}
func (f *notificationFeed) publish(event Notification) { f.publishScoped("", event) }
func (f *notificationFeed) publishScoped(scope string, event Notification) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if scope != "" && scope != f.scope {
		return false
	}
	if f.journal != nil {
		pref, err := f.journal.NotificationPreference(f.scope, event.Thread)
		if err != nil {
			f.journalFailed = true
			f.statusLocked("提醒记录同步暂不可用，正在重试")
			return false
		}
		if !pref.Enabled || pref.Generation != event.Subscription {
			return true
		}
	}
	for _, old := range f.recent {
		if old.ID == event.ID {
			return !f.journalFailed
		}
	}
	if f.journal != nil {
		if err := f.journal.SaveNotification(f.scope, event); err != nil {
			f.journalFailed = true
			f.statusLocked("提醒记录同步暂不可用，正在重试")
			return false
		} else if f.journalFailed {
			f.journalFailed = false
			f.statusLocked("")
		}
	}
	f.recent = append(f.recent, event)
	if len(f.recent) > 256 {
		f.recent = f.recent[len(f.recent)-256:]
	}
	for id, send := range f.listeners {
		if !send(Event{Type: "notification", Data: event}) {
			delete(f.listeners, id)
		}
	}
	return true
}
func (f *notificationFeed) status(message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusLocked(message)
}
func (f *notificationFeed) statusLocked(message string) {
	if message == "" && f.journalFailed {
		return
	}
	if message == f.lastError {
		return
	}
	f.lastError = message
	for id, send := range f.listeners {
		if !send(Event{Type: "notifications.status", Data: map[string]string{"error": message}}) {
			delete(f.listeners, id)
		}
	}
}
func (f *notificationFeed) run(s *Server) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.Bridge.Done():
			cancel()
		case <-ctx.Done():
		}
	}()
	type watch struct {
		stop       context.CancelFunc
		done       chan struct{}
		generation string
	}
	// ponytail: one read-only follower per chat; replace with a native global feed if very large histories become expensive.
	watchers := map[string]watch{}
	slots := make(chan struct{}, 4)
	defer func() {
		for _, entry := range watchers {
			entry.stop()
		}
	}()
	refresh := func() {
		if err := f.syncJournal(s); err != nil {
			for id, entry := range watchers {
				entry.stop()
				delete(watchers, id)
			}
			f.status("提醒记录同步暂不可用，正在重试")
			return
		}
		call, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()
		sessions := []desktop.Session{}
		for offset := 0; ; offset += 500 {
			page, err := s.Bridge.History.ListPage(call, "", false, 500, offset)
			if err != nil {
				f.status("notification discovery unavailable; retrying automatically")
				return
			}
			sessions = append(sessions, page...)
			if len(page) < 500 {
				break
			}
		}
		f.status("")
		live := map[string]bool{}
		for _, session := range sessions {
			pref, err := s.Store.NotificationPreference(f.scope, session.ID)
			if err != nil || !pref.Enabled {
				continue
			}
			live[session.ID] = true
			if entry, ok := watchers[session.ID]; ok {
				if entry.generation != pref.Generation {
					entry.stop()
					delete(watchers, session.ID)
				} else {
					select {
					case <-entry.done:
						delete(watchers, session.ID)
					default:
						continue
					}
				}
			}
			sub, stop := context.WithCancel(ctx)
			finished := make(chan struct{})
			watchers[session.ID] = watch{stop, finished, pref.Generation}
			go func(session desktop.Session, generation string) {
				defer close(finished)
				defer stop()
				select {
				case slots <- struct{}{}:
				case <-sub.Done():
					return
				}
				// Reuse the existing read-only follower; never create an executor.
				call, done := context.WithTimeout(sub, 8*time.Second)
				states, remove, err := s.Bridge.Subscribe(call, session.ID)
				done()
				<-slots
				if err != nil {
					f.status("notification subscription unavailable; retrying automatically")
					return
				}
				defer remove()
				tracker, scope := f.checkpoint(session.ID)
				if tracker.Generation != generation {
					tracker = notificationTracker{Generation: generation}
				}
				for {
					select {
					case <-sub.Done():
						return
					case state := <-states:
						if currentScope, err := s.Bridge.NotificationScope(); err != nil || currentScope != scope {
							return
						}
						previous := tracker
						durable := true
						for _, event := range tracker.observe(state, session.Title) {
							event.Subscription = generation
							if !f.publishScoped(scope, event) {
								durable = false
							}
						}
						if durable {
							f.saveCheckpoint(session.ID, scope, tracker)
						} else {
							tracker = previous
						}
					}
				}
			}(session, pref.Generation)
		}
		for id, entry := range watchers {
			if !live[id] {
				entry.stop()
				delete(watchers, id)
			}
		}
	}
	refresh()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}
