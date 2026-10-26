package server

import (
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Shared per-IP and global budgets bound password hashing and login attempts.
func (s *Server) allowLoginAttempt(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.window) > time.Minute {
		s.window = time.Now()
		s.attempts = map[string]int{}
	}
	if s.attempts["total"] >= 60 || s.attempts["ip:"+ip] >= 10 {
		return false
	}
	s.attempts["total"]++
	s.attempts["ip:"+ip]++
	return true
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.allowLoginAttempt(s.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "login rate limited", 429)
		return
	}
	// Bound slow request bodies without applying a read timeout to WebSocket chats.
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer controller.SetReadDeadline(time.Time{})
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Remember bool   `json:"remember"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&input) != nil || decoder.Decode(&extra) != io.EOF || len(input.Username) > 80 || len(input.Password) > 256 {
		http.Error(w, "invalid login request", 400)
		return
	}
	select {
	case s.loginSlots <- struct{}{}:
		defer func() { <-s.loginSlots }()
	default:
		http.Error(w, "login busy; try later", 429)
		return
	}
	browser := strings.HasPrefix(r.URL.Path, "/api/v1/browser/")
	if input.Name == "" {
		if browser {
			input.Name = "Web browser"
		} else {
			input.Name = "Android App"
		}
	}
	device, token, err := s.Store.Login(input.Username, input.Password, input.Name, s.clientIP(r), input.Remember)
	input.Password = ""
	if err != nil {
		if errors.Is(err, store.ErrUnauthorized) {
			http.Error(w, "account or password rejected", 401)
		} else {
			http.Error(w, "login unavailable", 503)
		}
		return
	}
	if browser {
		cookie := browserSessionCookie(token, time.Time{})
		if input.Remember {
			cookie.Expires = time.Unix(device.Expires, 0)
			cookie.MaxAge = int(store.RememberLogin / time.Second)
		}
		http.SetCookie(w, cookie)
		output(w, 200, map[string]any{"device": device, "version": config.Version})
		return
	}
	output(w, 200, map[string]any{"device": device, "token": token, "version": config.Version})
}
