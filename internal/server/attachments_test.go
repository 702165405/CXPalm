package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"cxpalm/internal/attachments"
)

func TestAttachmentHTTPRequiresOriginAuthorizationAndAllowedChat(t *testing.T) {
	s, ts, token := fixture(t)
	db, err := sql.Open("sqlite", filepath.Join(s.Config.CodexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,archived INTEGER);INSERT INTO threads VALUES('chat','fixture','.',1,'vscode',0)`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	upload := func(origin, bearer, thread string) *http.Response {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("file", "test.txt")
		part.Write([]byte("fixture attachment"))
		form.Close()
		r, _ := http.NewRequest("POST", ts.URL+"/api/v1/attachments?thread="+thread, &body)
		r.Host = "cx.example"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", form.FormDataContentType())
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	if upload("https://evil.example", token, "chat").StatusCode != 403 {
		t.Fatal("cross-origin upload")
	}
	if upload("https://cx.example", "", "chat").StatusCode != 401 {
		t.Fatal("anonymous upload")
	}
	if upload("https://cx.example", token, "missing").StatusCode != 400 {
		t.Fatal("unknown chat upload")
	}
	response := upload("https://cx.example", token, "chat")
	if response.StatusCode != 201 {
		t.Fatal(response.Status)
	}
	var file attachments.File
	if err = json.NewDecoder(response.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if file.ID == "" || file.Path != "" || file.Device != "" || file.Thread != "" {
		t.Fatal("metadata leaked")
	}
	device, _ := s.Store.Authorize(token)
	if _, err = s.Bridge.Uploads.Resolve(device.ID, "chat", []string{file.ID}); err != nil {
		t.Fatal(err)
	}
	s.Store.Revoke(device.ID)
	if upload("https://cx.example", token, "chat").StatusCode != 401 {
		t.Fatal("revoked upload")
	}
	if strings.Contains(response.Header.Get("Cache-Control"), "public") {
		t.Fatal("upload cached")
	}
}
