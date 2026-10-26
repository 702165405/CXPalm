package server

import (
	"context"
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveDesktopThroughWebSocketReadOnly(t *testing.T) {
	path := os.Getenv("CXPALM_LIVE_CONFIG")
	if path == "" {
		t.Skip("opt-in real desktop read-only WSS path")
	}
	c, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	c.DataDir = t.TempDir()
	db, e := store.Open(c.DataDir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s := New(c, db)
	defer s.Bridge.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	sessions, e := s.Bridge.History.List(ctx, "", false)
	if e != nil || len(sessions) == 0 {
		t.Fatal("allowed desktop chat required", e)
	}
	_, token, e := loginFixture(t, db, "isolated-read-only-test")
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	origin, _ := url.Parse(c.Origin)
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: hostTransport{transport, origin.Host}}
	socket, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{c.Origin}, "Authorization": []string{"Bearer " + token}}})
	if e != nil {
		t.Fatal(e)
	}
	defer socket.CloseNow()
	socket.SetReadLimit(2 << 20)
	_, _, e = socket.Read(ctx)
	if e != nil {
		t.Fatal(e)
	}
	command, _ := json.Marshal(Command{ID: "read-only-subscription", Method: "chat.subscribe", Thread: sessions[0].ID})
	if e = socket.Write(ctx, websocket.MessageText, command); e != nil {
		t.Fatal(e)
	}
	for {
		_, raw, e := socket.Read(ctx)
		if e != nil {
			t.Fatal(e)
		}
		var event struct {
			Type  string `json:"type"`
			Error string `json:"error"`
			Data  struct {
				Connected bool `json:"connected"`
				Snapshot  struct {
					Turns []json.RawMessage `json:"turns"`
				} `json:"snapshot"`
			} `json:"data"`
		}
		if e = json.Unmarshal(raw, &event); e != nil {
			t.Fatal(e)
		}
		if event.Type == "error" {
			t.Fatal(event.Error)
		}
		if event.Type == "chat.state" && event.Data.Connected {
			if len(event.Data.Snapshot.Turns) == 0 {
				t.Fatal("no phone-visible turns")
			}
			t.Logf("real desktop -> Go bridge -> authenticated WebSocket: %d bytes, %d turns", len(raw), len(event.Data.Snapshot.Turns))
			return
		}
	}
}

type hostTransport struct {
	base http.RoundTripper
	host string
}

func (r hostTransport) RoundTrip(q *http.Request) (*http.Response, error) {
	q = q.Clone(q.Context())
	q.Host = r.host
	return r.base.RoundTrip(q)
}
