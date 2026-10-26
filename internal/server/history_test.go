package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoryReadOverAuthenticatedWebsocket(t *testing.T) {
	s, ts, token := fixture(t)
	path := filepath.Join(s.Config.CodexHome, "rollout.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 35; i++ {
		fmt.Fprintf(f, `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"text":"question %d"}]}}`+"\n", i)
		fmt.Fprintf(f, `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"text":"answer %d"}]}}`+"\n", i)
	}
	f.Close()
	db, err := sql.Open("sqlite", filepath.Join(s.Config.CodexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE threads(id TEXT,cwd TEXT,source TEXT,archived INTEGER,updated_at INTEGER,rollout_path TEXT);INSERT INTO threads VALUES('chat','.','vscode',1,1,?)`, path)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/v1/ws", &websocket.DialOptions{HTTPClient: &http.Client{Transport: roundTripper{transport}}, HTTPHeader: http.Header{"Origin": {"https://cx.example"}, "Authorization": {"Bearer " + token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if _, _, err = c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	for i, q := range []Command{{ID: "first", Method: "chat.history", Thread: "chat", Offset: 0, Limit: 20}, {ID: "last", Method: "chat.history", Thread: "chat", Cursor: "latest", Limit: 20}, {ID: "invalid", Method: "chat.history", Thread: "chat", Offset: -1}} {
		data, _ := json.Marshal(q)
		if err = c.Write(ctx, websocket.MessageText, data); err != nil {
			t.Fatal(err)
		}
		_, raw, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			ID   string
			Type string
			Data struct {
				Offset, Total    int
				Turns, Questions []any
			}
		}
		if err = json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		if result.ID != q.ID {
			t.Fatal("uncorrelated history response")
		}
		if i == 2 {
			if result.Type != "error" {
				t.Fatal("invalid page accepted")
			}
			continue
		}
		if result.Type != "result" || result.Data.Total != 35 || len(result.Data.Questions) != 35 || len(result.Data.Turns) != 20 {
			t.Fatal(string(raw))
		}
		if result.Data.Offset != []int{0, 15}[i] {
			t.Fatal("wrong history window")
		}
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("history read mutated official rollout")
	}
}
