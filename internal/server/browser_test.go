package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestBrowserCookieLoginAndRevocation(t *testing.T) {
	s, ts, _ := fixture(t)
	post := func(path, origin, body string, cookie *http.Cookie) *http.Response {
		t.Helper()
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
		req.Host = "cx.example"
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	body := `{"username":"fixture","password":"fixture-test-password-012345","remember":true}`
	for _, origin := range []string{"", "https://evil.example"} {
		if response := post("/api/v1/browser/login", origin, body, nil); response.StatusCode != 403 {
			t.Fatal("cross-origin login accepted")
		}
	}
	response := post("/api/v1/browser/login", "https://cx.example", body, nil)
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	var payload struct {
		Device struct {
			ID string `json:"id"`
		} `json:"device"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Token != "" || payload.Device.ID == "" {
		t.Fatal("browser response leaks bearer or omits identity")
	}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatal("cookie missing")
	}
	cookie := cookies[0]
	if cookie.Name != browserCookie || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.Path != "/api/v1/" || cookie.Expires.Before(time.Now()) {
		t.Fatal("unsafe browser cookie")
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("auth response may be cached")
	}
	if response := post("/api/v1/browser/session", "https://cx.example", `{}`, nil); response.StatusCode != 401 {
		t.Fatal("anonymous session accepted")
	}
	for _, origin := range []string{"", "https://evil.example"} {
		if response := post("/api/v1/browser/logout", origin, `{}`, cookie); response.StatusCode != 403 {
			t.Fatal("cross-origin logout accepted")
		}
	}
	if response := post("/api/v1/browser/session", "https://cx.example", `{}`, cookie); response.StatusCode != 200 {
		t.Fatal("cookie session rejected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: roundTripper{transport}}
	headers := http.Header{"Origin": {"https://cx.example"}, "Cookie": {cookie.Name + "=" + cookie.Value}}
	for _, h := range []http.Header{
		{"Origin": {"https://evil.example"}, "Cookie": headers["Cookie"]},
		{"Cookie": headers["Cookie"]},
		{"Origin": {"https://cx.example"}},
	} {
		c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: h})
		if err == nil {
			c.CloseNow()
			t.Fatal("unauthorized/cross-origin cookie WebSocket accepted")
		}
	}
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: client, HTTPHeader: headers})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if _, _, err = c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	s.Store.Begin(payload.Device.ID, "fixture-read", "fixture")
	s.Store.Finish(payload.Device.ID, "fixture-read", "confirmed", nil)
	for _, method := range []string{"operation.get", "files.read", "terminal.open", "git.status", "side.create"} {
		command, _ := json.Marshal(Command{ID: method, Method: method, RequestID: "fixture-read"})
		if err = c.Write(ctx, websocket.MessageText, command); err != nil {
			t.Fatal(err)
		}
		_, raw, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event Event
		if err = json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		if method == "operation.get" && event.Type != "result" || method != "operation.get" && event.Type != "error" {
			t.Fatal("cookie changed command allowlist")
		}
	}
	response = post("/api/v1/browser/logout", "https://cx.example", `{}`, cookie)
	if response.StatusCode != 200 || s.Store.Valid(payload.Device.ID) {
		t.Fatal("logout did not revoke device")
	}
	if len(response.Cookies()) != 1 || response.Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not expire cookie")
	}
	if response := post("/api/v1/browser/session", "https://cx.example", `{}`, cookie); response.StatusCode != 401 {
		t.Fatal("revoked cookie accepted")
	}
	if err = c.Write(ctx, websocket.MessageText, []byte(`{"id":"after-revoke","method":"projects.list"}`)); err == nil {
		if _, _, err = c.Read(ctx); err == nil {
			t.Fatal("revoked browser retained command access")
		}
	}
}

func TestBrowserRejectsAmbiguousCredentialsAndTokenURLs(t *testing.T) {
	_, ts, token := fixture(t)
	for _, setup := range []func(*http.Request){
		func(r *http.Request) { r.URL.RawQuery = "token=" + token },
		func(r *http.Request) { r.Header.Set("Sec-WebSocket-Protocol", "bearer,"+token) },
		func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: browserCookie, Value: token})
			r.AddCookie(&http.Cookie{Name: browserCookie, Value: token})
		},
		func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: browserCookie, Value: token})
			r.Header.Set("Authorization", "Basic invalid")
		},
	} {
		req, _ := http.NewRequest("POST", ts.URL+"/api/v1/browser/session", nil)
		req.Host = "cx.example"
		req.Header.Set("Origin", "https://cx.example")
		setup(req)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != 401 {
			t.Fatal("ambiguous or URL credentials accepted")
		}
	}

	for i := 0; i < 11; i++ {
		req, _ := http.NewRequest("POST", ts.URL+"/api/v1/browser/login", strings.NewReader(`{}`))
		req.Host = "cx.example"
		req.Header.Set("Origin", "https://cx.example")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if i == 10 && response.StatusCode != 429 {
			t.Fatal("browser login bypasses rate limit")
		}
	}
}
