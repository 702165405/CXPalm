package server

import (
	"context"
	"net/http"
	"time"

	"cxpalm/internal/attachments"
)

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	d, err := s.authorize(r)
	if err != nil {
		http.Error(w, "device authorization required", 401)
		return
	}
	thread := r.URL.Query().Get("thread")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	row, err := s.Bridge.History.Get(ctx, thread)
	if err != nil || row.Archived {
		http.Error(w, "chat unavailable or archived", 400)
		return
	}
	select {
	case s.uploadSlots <- struct{}{}:
		defer func() { <-s.uploadSlots }()
	default:
		http.Error(w, "upload busy", 429)
		return
	}
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(60 * time.Second))
	r.Body = http.MaxBytesReader(w, r.Body, attachments.MaxSize+(64<<10))
	multipart, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "multipart file required", 400)
		return
	}
	part, err := multipart.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		http.Error(w, "one file required", 400)
		return
	}
	// Revocation and IP checks apply before accepting any bytes, as on WebSocket commands.
	if !s.Store.Valid(d.ID) || !s.Store.IPAllowed(s.clientIP(r)) {
		http.Error(w, "authorization rejected", 401)
		return
	}
	f, err := s.Bridge.Uploads.Save(d.ID, thread, part.FileName(), part)
	if err != nil {
		http.Error(w, "upload failed: check file size (20 MiB maximum), connection and storage quota", 400)
		return
	}
	if !s.Store.Valid(d.ID) || !s.Store.IPAllowed(s.clientIP(r)) {
		http.Error(w, "authorization rejected", 401)
		return
	}
	output(w, 201, f)
}
