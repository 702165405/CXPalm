package desktop

import (
	"context"
	"cxpalm/internal/config"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoveryFiltersSourcesAcrossDirectories(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER,thread_source TEXT);`)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range []struct{ id, source, originator, cwd string }{{"desktop", "vscode", "Codex Desktop", root}, {"created-by-old-bridge", "vscode", "codex_mobile_bridge", root}, {"subagent", `{"subagent":{}}`, "Codex Desktop", root}, {"outside", "vscode", "Codex Desktop", os.TempDir()}} {
		_, e = db.Exec("INSERT INTO threads VALUES(?,?,?,0,?,?,0,'')", entry.id, "title", entry.cwd, entry.source, entry.originator)
		if e != nil {
			t.Fatal(e)
		}
	}
	db.Close()
	h := History{Config: config.Config{CodexHome: home}}
	rows, e := h.List(context.Background(), "", false)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 desktop chats across projects, got %v", rows)
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var metadata []map[string]any
	if err = json.Unmarshal(encoded, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, row := range metadata {
		if _, exists := row["project"]; exists {
			t.Fatal("retired label in session response")
		}
	}
	if _, e = h.Get(context.Background(), "outside"); e != nil {
		t.Fatal("desktop chat with unrelated working directory rejected", e)
	}
	if _, e = h.Get(context.Background(), "subagent"); e == nil {
		t.Fatal("subagent exposed")
	}
}

func TestDiscoverySearchRecencyArchiveAndPaging(t *testing.T) {
	home := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE threads(id TEXT,name TEXT,title TEXT,cwd TEXT,updated_at INTEGER,recency_at INTEGER,recency_at_ms INTEGER,source TEXT,originator TEXT,archived INTEGER,thread_source TEXT);`)
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(home, "no-longer-exists")
	// Put the target beyond the old 500-row window and outside every configured root.
	for i := 0; i < 501; i++ {
		_, err = db.Exec(`INSERT INTO threads VALUES(?,NULL,'filler',?,1000,NULL,NULL,'vscode','Codex Desktop',0,'')`, fmt.Sprintf("filler-%03d", i), home)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`INSERT INTO threads VALUES
 ('old','Renamed','Original title',?,1,NULL,NULL,'vscode','Codex Desktop',0,''),
 ('recent-a',NULL,'Native clock',?,2,2000,NULL,'vscode','codex_work_desktop',0,''),
 ('recent-b',NULL,'Native clock',?,3,NULL,2000999,'vscode','Codex Desktop',0,''),
 ('recent-c',NULL,'Native clock',?,4,NULL,2000999,'vscode','Codex Desktop',0,''),
 ('archive',NULL,'Archived',?,5,NULL,NULL,'vscode','Codex Desktop',1,''),
 ('cli',NULL,'CLI',?,9999,NULL,NULL,'cli','codex_cli_rs',0,''),
 ('legacy',NULL,'Legacy',?,6,NULL,NULL,'vscode',NULL,0,''),
 ('thread-agent',NULL,'Agent',?,9999,NULL,NULL,'vscode','Codex Desktop',0,'subagent'),
 ('json-agent',NULL,'Agent',?,9999,NULL,NULL,'{"subagent":{}}','Codex Desktop',0,''),
 ('source-text',NULL,'Source',?,7,NULL,NULL,'not-a-subagent-source','Codex Desktop',0,'')`, missing, home, home, home, home, home, home, home, home, home)
	if err != nil {
		t.Fatal(err)
	}
	h := History{Config: config.Config{CodexHome: home}}
	ctx := context.Background()
	for _, tc := range []struct {
		name, query   string
		archived      bool
		limit, offset int
		want          []string
	}{
		{"recency-ms-and-id", "Native clock", false, 2, 0, []string{"recent-c", "recent-b"}},
		{"offset", "Native clock", false, 2, 2, []string{"recent-a"}},
		{"name", "RENAMED", false, 100, 0, []string{"old"}},
		{"title-before-limit", "Original", false, 1, 0, []string{"old"}},
		{"missing-cwd", "no-longer-exists", false, 100, 0, []string{"old"}},
		{"archive", "", true, 100, 0, []string{"archive"}},
		{"legacy", "Legacy", false, 100, 0, []string{"legacy"}},
		{"source-text", "Source", false, 100, 0, []string{"source-text"}},
		{"cli", "CLI", false, 100, 0, []string{}},
		{"agents", "Agent", false, 100, 0, []string{}},
		{"sql-parameter", "' OR 1=1 --", false, 100, 0, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, e := h.ListPage(ctx, tc.query, tc.archived, tc.limit, tc.offset)
			if e != nil {
				t.Fatal(e)
			}
			got := []string{}
			for _, r := range rows {
				got = append(got, r.ID)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	rows, err := h.List(ctx, "", false)
	if err != nil || len(rows) != 100 {
		t.Fatalf("default page: %d, %v", len(rows), err)
	}
	rows, err = h.ListPage(ctx, "Native clock", false, 1, 0)
	if err != nil || rows[0].Updated != 2000 {
		t.Fatalf("epoch seconds compatibility: %v, %v", rows, err)
	}
	for _, id := range []string{"old", "archive"} {
		if _, err = h.Get(ctx, id); err != nil {
			t.Fatalf("lookup %s: %v", id, err)
		}
	}
	for _, id := range []string{"cli", "thread-agent", "json-agent"} {
		if _, err = h.Get(ctx, id); err == nil {
			t.Fatalf("non-desktop root exposed: %s", id)
		}
	}
	for _, args := range [][2]int{{0, 0}, {501, 0}, {1, -1}} {
		if _, err = h.ListPage(ctx, "", false, args[0], args[1]); err == nil {
			t.Fatalf("invalid pagination accepted: %v", args)
		}
	}
}

func TestDiscoveryWithoutOriginatorColumn(t *testing.T) {
	home := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,archived INTEGER);
 INSERT INTO threads VALUES('legacy','Older desktop schema','missing-dir',1,'vscode',0);`)
	if err != nil {
		t.Fatal(err)
	}
	h := History{Config: config.Config{CodexHome: home}}
	rows, err := h.List(context.Background(), "", false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("legacy schema: %v, %v", rows, err)
	}
}

func TestListOmitsMetadataOnlyPlaceholderButExactLookupStillCreatesSafely(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	db, e := sql.Open("sqlite", filepath.Join(b.config.CodexHome, "state_5.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(`ALTER TABLE threads ADD COLUMN first_user_message TEXT; ALTER TABLE threads ADD COLUMN name TEXT; INSERT INTO threads(id,title,name,cwd,updated_at,source,originator,archived,first_user_message) VALUES('placeholder','','New chat','.',1,'vscode','Codex Desktop',0,''),('message-only','','Named','.',2,'vscode','Codex Desktop',0,'first message')`); e != nil {
		t.Fatal(e)
	}
	rows, e := b.History.List(context.Background(), "", false)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 2 {
		t.Fatal("metadata-only placeholder leaked into official list", rows)
	}
	for _, r := range rows {
		if r.ID == "placeholder" {
			t.Fatal("empty placeholder listed")
		}
	}
	if _, e = b.History.Get(context.Background(), "placeholder"); e != nil {
		t.Fatal("empty thread creation/selection was broken", e)
	}
}
