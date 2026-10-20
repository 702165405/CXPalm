package server

import (
	"cxpalm/internal/config"
	"cxpalm/internal/store"
	"net/http"
	"strings"
	"time"
)

const browserCookie = "__Secure-cxpalm-browser"

// Browsers cannot set WebSocket Authorization headers. The existing device token
// stays in a host-only HttpOnly cookie; the same exact-Origin gate protects it.
func (s *Server) authorize(r *http.Request) (store.Device, error) {
	if authorization := r.Header.Get("Authorization"); authorization != "" {
		if !strings.HasPrefix(authorization, "Bearer ") {
			return store.Device{}, store.ErrUnauthorized
		}
		return s.Store.Authorize(strings.TrimPrefix(authorization, "Bearer "))
	}
	var token string
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name == browserCookie {
			token = cookie.Value
			count++
		}
	}
	if count != 1 || len(token) != 64 {
		return store.Device{}, store.ErrUnauthorized
	}
	return s.Store.Authorize(token)
}

func browserSessionCookie(token string, expires time.Time) *http.Cookie {
	return &http.Cookie{Name: browserCookie, Value: token, Path: "/api/v1/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, Expires: expires}
}

func (s *Server) browserSession(w http.ResponseWriter, r *http.Request) {
	d, err := s.authorize(r)
	if err != nil {
		http.Error(w, "device authorization required", http.StatusUnauthorized)
		return
	}
	output(w, http.StatusOK, map[string]any{"device": d, "version": config.Version})
}

func (s *Server) browserLogout(w http.ResponseWriter, r *http.Request) {
	d, err := s.authorize(r)
	if err == nil {
		err = s.Store.RevokeFrom(d.ID, s.clientIP(r))
		if err != nil {
			http.Error(w, "could not revoke device", http.StatusInternalServerError)
			return
		}
	}
	cookie := browserSessionCookie("", time.Unix(1, 0))
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
	output(w, http.StatusOK, map[string]bool{"revoked": true})
}
