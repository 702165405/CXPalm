package desktop

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"cxpalm/internal/store"
)

// Official long-chat snapshots can exceed 16 MiB; retain a bounded native frame limit.
const maxFrame = 64 << 20

var methods = map[string]int{
	"initialize": 0, "thread-owner-discovery": 1, "thread-stream-following-changed": 1,
	"thread-follower-start-turn": 2, "thread-follower-steer-turn": 1,
	"thread-follower-interrupt-turn": 4, "thread-follower-load-complete-history": 1,
	"thread-follower-command-approval-decision": 1, "thread-follower-file-approval-decision": 1,
	"thread-follower-permissions-request-approval-response": 1,
	"thread-follower-submit-user-input":                     1,
	"thread-follower-update-thread-settings":                2,
	"thread-follower-compact-thread":                        1,
}

type Frame struct {
	Type       string          `json:"type"`
	RequestID  string          `json:"requestId,omitempty"`
	Source     string          `json:"sourceClientId,omitempty"`
	Target     string          `json:"targetClientId,omitempty"`
	Targets    []string        `json:"targetClientIds,omitempty"`
	Method     string          `json:"method,omitempty"`
	Version    int             `json:"version"`
	TimeoutMS  int             `json:"timeoutMs,omitempty"`
	Params     json.RawMessage `json:"params,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	ResultType string          `json:"resultType,omitempty"`
	Error      json.RawMessage `json:"error,omitempty"`
}
type IPC struct {
	conn     io.ReadWriteCloser
	mu       sync.Mutex
	write    sync.Mutex
	pending  map[string]chan Frame
	clientID string
	closed   chan struct{}
	once     sync.Once
	events   chan Frame
}

func Connect(ctx context.Context, path string) (*IPC, error) {
	c, e := dial(ctx, path)
	if e != nil {
		return nil, e
	}
	return New(ctx, c)
}
func New(ctx context.Context, conn io.ReadWriteCloser) (*IPC, error) {
	c := &IPC{conn: conn, pending: map[string]chan Frame{}, closed: make(chan struct{}), events: make(chan Frame, 256)}
	go c.read()
	r, e := c.Request(ctx, "initialize", map[string]any{"clientType": "cxpalm"}, "")
	if e != nil {
		c.Close()
		return nil, e
	}
	var v struct {
		ClientID string `json:"clientId"`
	}
	if e = json.Unmarshal(r.Result, &v); e != nil || v.ClientID == "" {
		c.Close()
		return nil, errors.New("desktop initialize did not return client identity")
	}
	c.mu.Lock()
	c.clientID = v.ClientID
	c.mu.Unlock()
	return c, nil
}
func (c *IPC) Events() <-chan Frame  { return c.events }
func (c *IPC) Done() <-chan struct{} { return c.closed }
func (c *IPC) Close()                { c.once.Do(func() { close(c.closed); c.conn.Close() }) }
func (c *IPC) read() {
	defer c.Close()
	defer close(c.events)
	for {
		var h [4]byte
		if _, e := io.ReadFull(c.conn, h[:]); e != nil {
			return
		}
		n := binary.LittleEndian.Uint32(h[:])
		if n == 0 || n > maxFrame {
			return
		}
		b := make([]byte, n)
		if _, e := io.ReadFull(c.conn, b); e != nil {
			return
		}
		var f Frame
		if json.Unmarshal(b, &f) != nil {
			return
		}
		if f.Type == "response" {
			c.mu.Lock()
			ch := c.pending[f.RequestID]
			c.mu.Unlock()
			if ch != nil {
				select {
				case ch <- f:
				default:
				}
			}
			continue
		}
		select {
		case c.events <- f:
		case <-c.closed:
			return
		default:
			return
		}
	}
}
func (c *IPC) send(ctx context.Context, f Frame) error {
	c.mu.Lock()
	f.Source = c.clientID
	c.mu.Unlock()
	b, e := json.Marshal(f)
	if e != nil {
		return e
	}
	if len(b) > maxFrame {
		return errors.New("IPC frame exceeds limit")
	}
	c.write.Lock()
	defer c.write.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		return errors.New("desktop disconnected")
	default:
	}
	// A blocked native pipe write must also be interrupted on request timeout.
	stop := context.AfterFunc(ctx, c.Close)
	defer stop()
	var h [4]byte
	binary.LittleEndian.PutUint32(h[:], uint32(len(b)))
	if e = writeAll(c.conn, h[:]); e == nil {
		e = writeAll(c.conn, b)
	}
	return e
}
func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, e := w.Write(b)
		if e != nil {
			return e
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func (c *IPC) Request(ctx context.Context, method string, params any, owner string) (Frame, error) {
	version, ok := methods[method]
	if !ok || method == "thread-stream-following-changed" {
		return Frame{}, errors.New("desktop method is not allowlisted")
	}
	b, e := json.Marshal(params)
	if e != nil {
		return Frame{}, e
	}
	id := store.Secret()[:32]
	ch := make(chan Frame, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	timeout := 15000
	if deadline, ok := ctx.Deadline(); ok {
		timeout = int(time.Until(deadline).Milliseconds())
		if timeout < 1 {
			timeout = 1
		}
	}
	if e = c.send(ctx, Frame{Type: "request", RequestID: id, Method: method, Version: version, TimeoutMS: timeout, Params: b, Target: owner}); e != nil {
		return Frame{}, e
	}
	select {
	case r := <-ch:
		if r.ResultType != "success" {
			return r, fmt.Errorf("desktop rejected %s", method)
		}
		return r, nil
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	case <-c.closed:
		return Frame{}, errors.New("desktop disconnected; operation outcome may be unknown")
	}
}
func (c *IPC) Follow(ctx context.Context, id, owner string) error {
	b, _ := json.Marshal(map[string]any{"hostId": "local", "conversationId": id, "following": true})
	f := Frame{Type: "broadcast", Method: "thread-stream-following-changed", Version: 1, Params: b}
	if owner != "" {
		f.Targets = []string{owner}
	}
	return c.send(ctx, f)
}
