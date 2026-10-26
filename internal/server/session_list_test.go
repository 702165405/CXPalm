package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionListPushesArchiveChangesAndCorrelatesScope(t *testing.T) {
	s, ts, token := fixture(t)
	db, e := sql.Open("sqlite", filepath.Join(s.Config.CodexHome, "state_5.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=2000; CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER);INSERT INTO threads VALUES('one','Fixture','/project',1,'vscode','Codex Desktop',0)`)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	c, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: &http.Client{Transport: roundTripper{transport}}, HTTPHeader: http.Header{"Origin": {"https://cx.example"}, "Authorization": {"Bearer " + token}}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	if _, _, e = c.Read(ctx); e != nil {
		t.Fatal(e)
	}
	write := func(q Command) {
		raw, _ := json.Marshal(q)
		if e = c.Write(ctx, websocket.MessageText, raw); e != nil {
			t.Fatal(e)
		}
	}
	page := func(id string) []map[string]any {
		for {
			_, raw, e := c.Read(ctx)
			if e != nil {
				t.Fatal(e)
			}
			var event struct {
				Type string
				Data struct {
					Subscription string
					Page         struct{ Items []map[string]any }
				}
			}
			if json.Unmarshal(raw, &event) == nil && event.Type == "sessions.page" && event.Data.Subscription == id {
				return event.Data.Page.Items
			}
		}
	}
	write(Command{ID: "active", Method: "sessions.subscribe", Limit: 100})
	if rows := page("active"); len(rows) != 1 || rows[0]["status"] != "unknown" {
		t.Fatal(rows)
	}
	if _, e = db.Exec(`UPDATE threads SET archived=1 WHERE id='one'`); e != nil {
		t.Fatal(e)
	}
	if rows := page("active"); len(rows) != 0 {
		t.Fatal("archived row remained in active list", rows)
	}
	write(Command{ID: "archived", Method: "sessions.subscribe", Limit: 100, Archived: true})
	if rows := page("archived"); len(rows) != 1 || rows[0]["archived"] != true {
		t.Fatal(rows)
	}
}
