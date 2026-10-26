package server

import (
	"context"
	"cxpalm/internal/store"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIPGateAndExplicitProxyTrust(t *testing.T) {
	s, _, _ := fixture(t)
	s.Config.TrustedProxyIP = true
	if _, err := s.Store.SetIPPolicy(store.IPPolicy{Enabled: true, Entries: []string{"203.0.113.0/24"}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		remote, header, xff string
		want                int
	}{{"127.0.0.1:1000", "203.0.113.5", "", 401}, {"127.0.0.1:1000", "198.51.100.1", "", 403}, {"127.0.0.1:1000", "", "203.0.113.5", 403}, {"127.0.0.1:1000", "203.0.113.5,198.51.100.1", "", 403}, {"198.51.100.1:1000", "203.0.113.5", "", 403}} {
		r := httptest.NewRequest("POST", "https://cx.example/api/v1/login", strings.NewReader("{}"))
		r.Host = "cx.example"
		r.RemoteAddr = tc.remote
		r.Header.Set("Origin", "https://cx.example")
		if tc.header != "" {
			r.Header.Set("X-CXPalm-Client-IP", tc.header)
		}
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%+v: %d", tc, w.Code)
		}
	}
	s.Config.TrustedProxyIP = false
	r := httptest.NewRequest("POST", "https://cx.example/api/v1/login", strings.NewReader("{}"))
	r.Host = "cx.example"
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Origin", "https://cx.example")
	r.Header.Set("X-CXPalm-Client-IP", "203.0.113.1")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("unconfigured header trusted")
	}
}
func TestIPChangeClosesAlreadyConnectedWebSocket(t *testing.T) {
	s, ts, token := fixture(t)
	s.Config.TrustedProxyIP = true
	if _, err := s.Store.SetIPPolicy(store.IPPolicy{Enabled: true, Entries: []string{"203.0.113.5"}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: &http.Client{Transport: roundTripper{transport}}, HTTPHeader: http.Header{"Origin": {"https://cx.example"}, "Authorization": {"Bearer " + token}, "X-CXPalm-Client-IP": {"203.0.113.5"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if _, _, err = c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Store.SetIPPolicy(store.IPPolicy{Enabled: true, Entries: []string{"198.51.100.1"}}); err != nil {
		t.Fatal(err)
	}
	if err = c.Write(ctx, websocket.MessageText, []byte(`{"id":"after-policy","method":"operation.get"}`)); err == nil {
		if _, _, err = c.Read(ctx); err == nil {
			t.Fatal("blocked peer still receives replies")
		}
	}
}
