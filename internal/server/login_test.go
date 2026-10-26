package server

import (
	"cxpalm/internal/store"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPasswordLoginCookieAndTokenBoundaries(t *testing.T) {
	s, ts, _ := fixture(t)
	_, err := s.Store.SetLogin(store.LoginInput{Username: "owner", Password: "fixture-password-12345", Expected: func() string { status, _ := s.Store.LoginSettings(); return status.Revision }()})
	if err != nil {
		t.Fatal(err)
	}
	post := func(path, origin, body string) *http.Response {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
		req.Host = "cx.example"
		req.Header.Set("Origin", origin)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	body := `{"username":"owner","password":"fixture-password-12345","remember":true}`
	for _, origin := range []string{"", "https://attacker.test"} {
		if post("/api/v1/browser/login", origin, body).StatusCode != 403 {
			t.Fatal("cross-origin login")
		}
	}
	if _, err := s.Store.SetIPPolicy(store.IPPolicy{Enabled: true, Entries: []string{"203.0.113.1"}}); err != nil {
		t.Fatal(err)
	}
	if post("/api/v1/login", "https://cx.example", body).StatusCode != 403 {
		t.Fatal("login bypasses IP whitelist")
	}
	if _, err := s.Store.SetIPPolicy(store.IPPolicy{}); err != nil {
		t.Fatal(err)
	}
	response := post("/api/v1/browser/login", "https://cx.example", body)
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatal("no session cookie")
	}
	c := cookies[0]
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Domain != "" || c.Path != "/api/v1/" || c.MaxAge != 30*24*3600 || c.Expires.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatal("insecure remembered cookie")
	}
	var result map[string]any
	json.NewDecoder(response.Body).Decode(&result)
	if result["token"] != nil {
		t.Fatal("browser response exposes bearer token")
	}
	response = post("/api/v1/browser/login", "https://cx.example", strings.Replace(body, `"remember":true`, `"remember":false`, 1))
	if response.StatusCode != 200 || response.Cookies()[0].MaxAge != 0 || !response.Cookies()[0].Expires.IsZero() {
		t.Fatal("temporary cookie is persistent")
	}
	response = post("/api/v1/login", "https://cx.example", body)
	var app struct {
		Token  string       `json:"token"`
		Device store.Device `json:"device"`
	}
	json.NewDecoder(response.Body).Decode(&app)
	if response.StatusCode != 200 || len(response.Cookies()) != 0 || len(app.Token) != 64 {
		t.Fatal("native login did not return isolated token")
	}
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/logout", strings.NewReader(`{}`))
	req.Host = "cx.example"
	req.Header.Set("Origin", "https://cx.example")
	req.Header.Set("Authorization", "Bearer "+app.Token)
	logout, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	logout.Body.Close()
	if logout.StatusCode != 200 || s.Store.Valid(app.Device.ID) {
		t.Fatal("native logout did not revoke session")
	}
	if post("/api/v1/login", "https://cx.example", `{"username":"owner","password":"wrong"}`).StatusCode != 401 {
		t.Fatal("incorrect password accepted")
	}
	if post("/api/v1/login", "https://cx.example", body+`{}`).StatusCode != 400 {
		t.Fatal("trailing JSON accepted")
	}
	for i := 0; i < 12; i++ {
		response = post("/api/v1/login", "https://cx.example", `{}`)
	}
	if response.StatusCode != 429 {
		t.Fatal("login attempts unbounded")
	}
}
