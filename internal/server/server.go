package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cxpalm/internal/attachments"
	"cxpalm/internal/config"
	"cxpalm/internal/desktop"
	"cxpalm/internal/store"
	"github.com/coder/websocket"
)

type Command struct {
	Stream      bool            `json:"stream,omitempty"`
	Lazy        bool            `json:"lazy,omitempty"`
	Turn        string          `json:"turn,omitempty"`
	Item        string          `json:"item,omitempty"`
	IndexOnly   bool            `json:"indexOnly,omitempty"`
	ReadToken   string          `json:"readToken,omitempty"`
	Project     string          `json:"project,omitempty"`
	Cursor      string          `json:"cursor,omitempty"`
	Archived    bool            `json:"archived,omitempty"`
	Limit       int             `json:"limit,omitempty"`
	Offset      int             `json:"offset,omitempty"`
	ID          string          `json:"id"`
	Method      string          `json:"method"`
	Thread      string          `json:"thread,omitempty"`
	Text        string          `json:"text,omitempty"`
	RequestID   string          `json:"requestId,omitempty"`
	Decision    json.RawMessage `json:"decision,omitempty"`
	Options     desktop.Options `json:"options,omitempty"`
	Skills      []string        `json:"skills,omitempty"`
	Attachments []string        `json:"attachments,omitempty"`
	Enabled     *bool           `json:"enabled,omitempty"`
}
type Event struct {
	ID    string `json:"id,omitempty"`
	Type  string `json:"type"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}
type Server struct {
	Config        config.Config
	Store         *store.Store
	Bridge        *desktop.Bridge
	mu            sync.Mutex
	attempts      map[string]int
	window        time.Time
	notifications notificationFeed
	slots         chan struct{}
	loginSlots    chan struct{}
	uploadSlots   chan struct{}
}

func New(c config.Config, s *store.Store) *Server {
	bridge := desktop.NewBridge(c)
	bridge.ReadStore = s
	bridge.Uploads = &attachments.Manager{Root: filepath.Join(c.DataDir, "uploads")}
	bridge.StartQueues()
	return &Server{Config: c, Store: s, Bridge: bridge, attempts: map[string]int{}, window: time.Now(), slots: make(chan struct{}, 16), loginSlots: make(chan struct{}, 2), uploadSlots: make(chan struct{}, 2)}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if r.Host != s.Config.Listen {
			http.Error(w, "local health check only", 403)
			return
		}
		output(w, 200, map[string]any{"version": config.Version})
	})
	mux.HandleFunc("POST /api/v1/login", s.login)
	mux.HandleFunc("POST /api/v1/browser/login", s.login)
	mux.HandleFunc("POST /api/v1/logout", s.browserLogout)
	mux.HandleFunc("POST /api/v1/browser/session", s.browserSession)
	mux.HandleFunc("POST /api/v1/browser/logout", s.browserLogout)
	mux.HandleFunc("GET /api/v1/ws", s.ws)
	mux.HandleFunc("POST /api/v1/attachments", s.upload)
	mux.HandleFunc("POST /api/v1/chat/image", s.image)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != "/health" {
			u, _ := url.Parse(s.Config.Origin)
			if r.Host != u.Host || r.Header.Get("Origin") != s.Config.Origin {
				http.Error(w, "origin rejected", 403)
				return
			}
			if !s.Store.IPAllowed(s.clientIP(r)) {
				http.Error(w, "IP access denied", http.StatusForbidden)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func output(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	d, e := s.authorize(r)
	if e != nil {
		http.Error(w, "device authorization rejected", 401)
		return
	}
	ip := s.clientIP(r)
	if err := s.Store.TouchDevice(d.ID, ip); err != nil {
		http.Error(w, "device audit unavailable", 500)
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		http.Error(w, "connection limit reached", 503)
		return
	}
	c, e := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{strings.TrimPrefix(s.Config.Origin, "https://")}})
	if e != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(128 << 10)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	out := make(chan Event, 32)
	var latest latestState
	defer func() { c.Close(websocket.StatusNormalClosure, "closed") }()
	send := func(v Event) bool {
		select {
		case out <- v:
			return true
		case <-ctx.Done():
			return false
		}
	}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		var encoder streamEncoder
		lastPresentation, lastTraffic := time.Now(), time.Now()
		for {
			var event Event
			// A busy receipt/list queue must not starve the replaceable live slot.
			if time.Since(lastPresentation) >= 100*time.Millisecond {
				lastPresentation = time.Now()
				if value := latest.take(); value != nil {
					if value.stream {
						event = encoder.next(s.Bridge.StreamState(value.state), value.subscription)
					} else {
						event = Event{Type: "chat.state", Data: s.Bridge.PhoneState(value.state)}
					}
				}
			}
			if event.Type == "" && time.Since(lastTraffic) >= 10*time.Second {
				event = Event{Type: "connection.heartbeat"}
			}
			if event.Type == "" {
				select {
				case <-ctx.Done():
					return
				case event = <-out:
				case <-ticker.C:
					continue
				}
			}

			{
				if !s.Store.Valid(d.ID) || !s.Store.IPAllowed(ip) {
					cancel()
					c.Close(websocket.StatusPolicyViolation, "device revoked")
					return
				}
				b, err := json.Marshal(event)
				if err != nil {
					cancel()
					return
				}
				writeCtx, done := context.WithTimeout(ctx, 10*time.Second)
				err = c.Write(writeCtx, websocket.MessageText, b)
				done()
				if err != nil {
					cancel()
					return
				}
				lastTraffic = time.Now()
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !s.Store.Valid(d.ID) || !s.Store.IPAllowed(ip) {
					cancel()
					c.Close(websocket.StatusPolicyViolation, "device revoked")
					return
				}
				pingCtx, done := context.WithTimeout(ctx, 8*time.Second)
				err := c.Ping(pingCtx)
				done()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	send(Event{Type: "hello", Data: map[string]any{"version": config.Version, "device": d.ID, "capabilities": []string{"chat.attachments", "chat.notifications", "chat.notifications.get", "sessions.browse", "sessions.subscribe", "sessions.list", "operation.get", "operation.acknowledge", "chat.subscribe", "chat.delta", "chat.item", "chat.history", "chat.read", "notifications.subscribe", "chat.send", "chat.steer", "chat.queue", "chat.queue.cancel", "chat.stop", "chat.approve", "chat.catalog", "chat.settings", "chat.goal", "chat.compact", "sessions.create", "sessions.create.settings", "sessions.rename", "sessions.archive", "sessions.unarchive"}}})
	var listCancel context.CancelFunc
	defer func() {
		if listCancel != nil {
			listCancel()
		}
	}()
	var stopNotifications func()
	defer func() {
		if stopNotifications != nil {
			stopNotifications()
		}
	}()
	var unsubscribe func()
	defer func() {
		if unsubscribe != nil {
			unsubscribe()
		}
	}()
	var subCancel context.CancelFunc
	defer func() {
		if subCancel != nil {
			subCancel()
		}
	}()
	commands := readCommands(ctx, c, cancel)
	readSlots := make(chan struct{}, 4)
	for {
		var b []byte
		select {
		case <-ctx.Done():
			return
		case b = <-commands:
		}
		if !s.Store.Valid(d.ID) || !s.Store.IPAllowed(ip) {
			return
		}
		var q Command
		if json.Unmarshal(b, &q) != nil || q.ID == "" || len(q.ID) > 128 {
			send(Event{Type: "error", Error: "valid command id required"})
			continue
		}
		if q.Method == "chat.read" {
			dispatchRead(ctx, readSlots, q, func(call context.Context, command Command) (any, error) {
				return s.execute(call, d.ID, command)
			}, send)
			continue
		}
		if q.Method == "sessions.subscribe" {
			s.Bridge.StartSessionSync()
			if q.Limit == 0 {
				q.Limit = 100
			}
			if q.Limit < 1 || q.Limit > 5000 || q.Offset < 0 || len(q.Project) > 2048 || len(q.Text) > 1000 {
				send(Event{ID: q.ID, Type: "error", Error: "invalid session filter"})
				continue
			}
			if listCancel != nil {
				listCancel()
			}
			listCtx, stop := context.WithCancel(ctx)
			listCancel = stop
			send(Event{ID: q.ID, Type: "result", Data: map[string]bool{"subscribed": true}})
			go func(q Command) {
				timer := time.NewTicker(time.Second)
				defer timer.Stop()
				previous := ""
				var nextRefresh time.Time
				for {
					if delay := time.Until(nextRefresh); delay > 0 {
						wait := time.NewTimer(delay)
						select {
						case <-listCtx.Done():
							wait.Stop()
							return
						case <-wait.C:
						}
					}
					readChanges := s.Bridge.ReadChanges()
					page, err := s.Bridge.Browse(listCtx, q.Text, q.Archived, q.Project, q.Limit, q.Offset)
					nextRefresh = time.Now().Add(100 * time.Millisecond)
					if err == nil {
						raw, _ := json.Marshal(page)
						if string(raw) != previous {
							previous = string(raw)
							if !send(Event{Type: "sessions.page", Data: map[string]any{"subscription": q.ID, "page": page}}) {
								return
							}
						}
					}
					if err != nil && previous != "error:"+err.Error() {
						previous = "error:" + err.Error()
						if !send(Event{Type: "sessions.error", Data: map[string]any{"subscription": q.ID, "error": err.Error()}}) {
							return
						}
					}
					select {
					case <-listCtx.Done():
						return
					case <-timer.C:
					case <-readChanges:
					}
				}
			}(q)
			continue
		}
		if q.Method == "notifications.subscribe" {
			if len(q.Cursor) > 128 {
				send(Event{ID: q.ID, Type: "error", Error: "invalid notification cursor"})
				continue
			}
			if stopNotifications != nil {
				stopNotifications()
			}
			stopNotifications = s.notifications.subscribe(s, q.Cursor, q.ID, send)
			continue
		}
		if q.Method == "chat.subscribe" {
			latest.reset(q.ID)
			if subCancel != nil {
				subCancel()
			}
			if unsubscribe != nil {
				unsubscribe()
				unsubscribe = nil
			}
			subCtx, stop := context.WithCancel(ctx)
			callCtx, done := context.WithTimeout(ctx, 8*time.Second)
			states, remove, err := s.Bridge.Subscribe(callCtx, q.Thread)
			done()
			if err != nil {
				stop()
				send(Event{ID: q.ID, Type: "error", Error: err.Error()})
				continue
			}
			subCancel = stop
			unsubscribe = remove
			go s.Bridge.OpenSelectedChat(subCtx, q.Thread)
			send(Event{ID: q.ID, Type: "result", Data: map[string]bool{"subscribed": true}})
			go func(stream bool, subscription string) {
				for {
					select {
					case state := <-states:
						// Each state is a complete copy; project and send only the newest of a burst.
						for drained := false; !drained; {
							select {
							case next := <-states:
								state = next
							default:
								drained = true
							}
						}
						latest.put(streamDelivery{state: state, stream: stream, subscription: subscription})
					case <-subCtx.Done():
						return
					}
				}
			}(q.Stream, q.ID)
			continue
		}
		result, err := s.execute(ctx, d.ID, q)
		if err != nil {
			send(Event{ID: q.ID, Type: "error", Error: err.Error()})
		} else {
			send(Event{ID: q.ID, Type: "result", Data: result})
		}
	}
}

// Read control frames even while an ordered desktop command is still running.
func readCommands(ctx context.Context, c *websocket.Conn, cancel context.CancelFunc) <-chan []byte {
	commands := make(chan []byte, 32)
	go func() {
		defer cancel()
		for {
			kind, b, err := c.Read(ctx)
			if err != nil {
				return
			}
			if kind != websocket.MessageText {
				c.Close(websocket.StatusUnsupportedData, "text JSON required")
				return
			}
			select {
			case <-ctx.Done():
				return
			case commands <- b:
			default:
				c.Close(websocket.StatusPolicyViolation, "too many queued commands")
				return
			}
		}
	}()
	return commands
}

func (s *Server) execute(ctx context.Context, device string, q Command) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	switch q.Method {
	case "chat.notifications.get":
		if _, err := s.Bridge.History.Get(ctx, q.Thread); err != nil {
			return nil, err
		}
		return s.Bridge.NotificationPreference(q.Thread)
	case "sessions.browse":
		s.Bridge.StartSessionSync()
		limit := q.Limit
		if limit == 0 {
			limit = 100
		}
		if len(q.Text) > 1000 || len(q.Project) > 2048 {
			return nil, errors.New("invalid session filter")
		}
		return s.Bridge.Browse(ctx, q.Text, q.Archived, q.Project, limit, q.Offset)
	case "sessions.list":
		limit := q.Limit
		if limit == 0 {
			limit = 100
		}
		return s.Bridge.History.ListPage(ctx, q.Text, q.Archived, limit, q.Offset)
	case "chat.read":
		accepted, err := s.Bridge.MarkRead(ctx, q.Thread, q.ReadToken)
		if err != nil {
			if errors.Is(err, desktop.ErrReadSyncUnavailable) {
				return map[string]any{"accepted": false, "available": false, "reason": desktop.ReadSyncUnavailableReason, "thread": q.Thread, "readToken": q.ReadToken}, nil
			}
			return nil, err
		}
		return map[string]any{"accepted": accepted, "available": true, "thread": q.Thread, "readToken": q.ReadToken}, nil
	case "chat.item":
		return s.Bridge.Item(ctx, q.Thread, q.Turn, q.Item)
	case "chat.history":
		if len(q.Cursor) > 128 {
			return nil, errors.New("invalid history cursor")
		}
		if q.IndexOnly {
			if q.Cursor != "latest" {
				return nil, errors.New("index-only requires latest cursor")
			}
			return s.Bridge.HistoryIndex(ctx, q.Thread, q.Limit)
		}
		if q.Lazy {
			return s.Bridge.HistoryPageLean(ctx, q.Thread, q.Cursor, q.Offset, q.Limit)
		}
		return s.Bridge.HistoryPage(ctx, q.Thread, q.Cursor, q.Offset, q.Limit)
	case "chat.catalog":
		session, err := s.Bridge.History.Get(ctx, q.Thread)
		if err != nil {
			return nil, err
		}
		return s.Bridge.Catalog(session.CWD)
	case "operation.get":
		return s.resolveOperation(device, q.RequestID)
	case "operation.acknowledge":
		if q.Text != "inspected" {
			return nil, errors.New("explicit inspection confirmation is required")
		}
		return s.Store.Acknowledge(device, q.RequestID)
	}
	mutation := q.Method == "chat.notifications" || q.Method == "sessions.rename" || q.Method == "sessions.create" || q.Method == "sessions.archive" || q.Method == "sessions.unarchive" || q.Method == "chat.queue" || q.Method == "chat.queue.cancel" || q.Method == "chat.send" || q.Method == "chat.steer" || q.Method == "chat.stop" || q.Method == "chat.approve" || q.Method == "chat.settings" || q.Method == "chat.goal" || q.Method == "chat.compact"
	if !mutation {
		return nil, errors.New("method is not implemented or not allowlisted")
	}
	b, _ := json.Marshal(q)
	receipt, fresh, err := s.Store.Begin(device, q.ID, store.Hash(string(b)))
	if err != nil {
		return nil, err
	}
	if !fresh {
		return receipt, nil
	}
	var result any
	if q.Method == "chat.notifications" {
		if q.Enabled == nil {
			err = errors.New("explicit enabled flag required")
		} else if _, err = s.Bridge.History.Get(ctx, q.Thread); err == nil {
			var scope string
			if scope, err = s.Bridge.NotificationScope(); err == nil {
				result, err = s.Store.SetNotificationPreference(scope, q.Thread, *q.Enabled)
				s.Bridge.NotifyChanged(q.Thread)
			}
		}
	} else if q.Method == "chat.queue" {
		result, err = s.Bridge.QueueAttachments(ctx, device, q.Thread, q.ID, q.Text, q.Skills, q.Attachments)
	} else if q.Method == "chat.queue.cancel" {
		result, err = s.Bridge.CancelQueue(ctx, q.Thread, q.RequestID)
	} else if q.Method == "sessions.rename" {
		result, err = s.Bridge.Rename(ctx, q.Thread, q.Text)
	} else if q.Method == "sessions.create" {
		var created map[string]any
		created, err = s.Bridge.CreateChat(ctx, q.Project, q.Options)
		if err == nil && q.Enabled != nil {
			scope, prefErr := s.Bridge.NotificationScope()
			if prefErr == nil {
				_, prefErr = s.Store.SetNotificationPreference(scope, created["id"].(string), *q.Enabled)
			}
			created["notificationsApplied"] = prefErr == nil
			if prefErr == nil {
				s.Bridge.NotifyChanged(created["id"].(string))
			}
			if prefErr != nil {
				// Creation succeeded: never turn a preference failure into a create retry.
				created["message"] = "会话已创建，但通知设置未保存，请在会话中手动检查；不要重复创建。"
			}
		}
		result = created
	} else if strings.HasPrefix(q.Method, "sessions.") {
		result, err = s.Bridge.Lifecycle(ctx, q.Method, q.Thread, q.Project)
	} else {
		files := []attachments.File{}
		if len(q.Attachments) > 0 && q.Method != "chat.send" && q.Method != "chat.steer" {
			err = errors.New("attachments require send, steer or queue")
		} else {
			files, err = s.Bridge.Uploads.Resolve(device, q.Thread, q.Attachments)
		}
		if err == nil {
			result, err = s.Bridge.ControlActionFiles(ctx, q.Thread, q.ID, q.Method, q.Text, q.RequestID, q.Decision, q.Options, q.Skills, files)
		}
	}
	if err != nil {
		var rejected *desktop.Rejected
		if errors.As(err, &rejected) {
			return s.Store.Finish(device, q.ID, "rejected", map[string]string{"error": rejected.Message, "code": rejected.Code, "method": q.Method, "thread": q.Thread})
		}
		return s.Store.Finish(device, q.ID, "unknown", map[string]string{"error": err.Error()})
	}
	return s.Store.Finish(device, q.ID, "confirmed", result)
}
