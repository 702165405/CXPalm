package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestHeartbeatWhileCommandDispatchIsBlocked(t *testing.T) {
	for _, pump := range []bool{false, true} {
		t.Run(fmt.Sprint(pump), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ready := make(chan struct{})
			release := make(chan struct{})
			result := make(chan error, 1)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				if err != nil {
					result <- err
					return
				}
				defer c.CloseNow()
				call, stop := context.WithCancel(ctx)
				defer stop()
				var commands <-chan []byte
				if pump {
					commands = readCommands(call, c, stop)
				}
				close(ready)
				// The dispatcher is deliberately not reading while its desktop operation waits.
				select {
				case <-release:
				case <-call.Done():
					result <- call.Err()
					return
				}
				if !pump {
					result <- nil
					return
				}
				for i := 0; i < 20; i++ {
					select {
					case b := <-commands:
						var q Command
						if json.Unmarshal(b, &q) != nil || q.ID != fmt.Sprint(i) {
							result <- fmt.Errorf("command order: %s", b)
							return
						}
					case <-call.Done():
						result <- call.Err()
						return
					}
				}
				result <- nil
			}))
			defer ts.Close()
			c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer c.CloseNow()
			go func() { _, _, _ = c.Read(ctx) }()
			<-ready
			// This ping needs the server reader to process a control frame, not the dispatcher.
			ping, done := context.WithTimeout(ctx, 300*time.Millisecond)
			err = c.Ping(ping)
			done()
			if pump && err != nil {
				t.Fatal("heartbeat falsely failed during a desktop command:", err)
			}
			if !pump && err == nil {
				t.Fatal("baseline unexpectedly read a pong without a reader")
			}
			if pump {
				for i := 0; i < 20; i++ {
					if err := c.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf(`{"id":"%d","method":"chat.history"}`, i))); err != nil {
						t.Fatal(err)
					}
				}
				ping, done = context.WithTimeout(ctx, time.Second)
				err = c.Ping(ping)
				done()
				if err != nil {
					t.Fatal("queued commands blocked control frames:", err)
				}
			}
			close(release)
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCommandQueueIsBounded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		call, stop := context.WithCancel(ctx)
		defer stop()
		readCommands(call, c, stop)
		<-call.Done()
	}))
	defer ts.Close()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	for i := 0; i < 33; i++ {
		if err = c.Write(ctx, websocket.MessageText, []byte(`{"id":"overflow"}`)); err != nil {
			break
		}
	}
	_, _, err = c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("unbounded or unexplained overflow: %v", err)
	}
}

func TestQuietWebSocketSendsApplicationHeartbeat(t *testing.T) {
	_, ts, token := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 13*time.Second)
	defer cancel()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: &http.Client{Transport: roundTripper{transport}}, HTTPHeader: http.Header{"Origin": {"https://cx.example"}, "Authorization": {"Bearer " + token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	for _, want := range []string{"hello", "connection.heartbeat"} {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatal("quiet connected socket has no application liveness", err)
		}
		var e Event
		if json.Unmarshal(data, &e) != nil || e.Type != want {
			t.Fatalf("want %s, got %s", want, data)
		}
	}
}
