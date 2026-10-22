package desktop

import (
	"context"
	"cxpalm/internal/config"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectMembershipBeforePaginationAndLiveSummary(t *testing.T) {
	home := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER); INSERT INTO threads VALUES ('one','First','/same',3,'vscode','Codex Desktop',0),('two','Second','/same',2,'vscode','Codex Desktop',0),('old','Old','/gone',1,'vscode','Codex Desktop',1)`); e != nil {
		t.Fatal(e)
	}
	metadata := `{"local-projects":{"p":{"id":"p","name":"Official project","rootPaths":["/same"]}},"thread-project-assignments":{"one":{"projectKind":"local","projectId":"p"}},"projectless-thread-ids":["two"]}`
	if e = os.WriteFile(filepath.Join(home, ".codex-global-state.json"), []byte(metadata), 0600); e != nil {
		t.Fatal(e)
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	raw := json.RawMessage(`{"hasUnreadTurn":true,"threadRuntimeStatus":{"type":"idle"},"turns":[{"status":"completed","agentMessageCompletedAtMsById":{"a":1790000000123}}]}`)
	b.sessions["one"] = &State{ID: "one", Connected: true, Owner: "official", Snapshot: raw}
	page, e := b.Browse(context.Background(), "", false, "p", 1, 0)
	if e != nil || len(page.Items) != 1 || page.Items[0].ID != "one" || page.HasMore {
		t.Fatalf("membership: %+v %v", page, e)
	}
	s := page.Items[0]
	if s.Status != "completed" || s.Unread == nil || !*s.Unread || s.LastReplyAt != 1790000000 || !s.Live {
		t.Fatalf("live summary: %+v", s)
	}
	if s.ProjectName != "Official project" {
		t.Fatal(s.ProjectName)
	}
	page, e = b.Browse(context.Background(), "", false, "none", 100, 0)
	if e != nil || len(page.Items) != 1 || page.Items[0].ID != "two" {
		t.Fatalf("explicit projectless ignored: %+v %v", page, e)
	}
	if page.Items[0].Unread != nil || page.Items[0].Status != "unknown" {
		t.Fatal("unknown was presented as read/completed")
	}
	page, e = b.Browse(context.Background(), "", true, "", 100, 0)
	if e != nil || len(page.Items) != 1 || !page.Items[0].Archived {
		t.Fatalf("archive: %+v %v", page, e)
	}
	page, e = b.Browse(context.Background(), "", false, "", 1, 1)
	if e != nil || len(page.Items) != 1 || page.Items[0].ID != "two" || page.HasMore {
		t.Fatalf("pagination: %+v %v", page, e)
	}
	b.sessions["one"].Connected = false
	page, _ = b.Browse(context.Background(), "", false, "p", 100, 0)
	if page.Items[0].Status != "unknown" || page.Items[0].Unread != nil {
		t.Fatal("stale snapshot pretends to be live")
	}
}

func TestSummaryStatusAndUnreadProjection(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`{"threadRuntimeStatus":{"type":"active"}}`, "running"},
		{`{"threadRuntimeStatus":{"type":"idle"},"turns":[{"status":"failed"}]}`, "failed"},
		{`{"threadRuntimeStatus":{"type":"active"},"requests":[{"id":"approval"}]}`, "waiting"},
		{`{"threadRuntimeStatus":{"type":"mystery"}}`, "unknown"},
	} {
		row := Session{Status: "unknown"}
		enrichSession(&row, State{Connected: true, Snapshot: json.RawMessage(tc.raw)})
		if row.Status != tc.want {
			t.Fatalf("%s -> %s", tc.raw, row.Status)
		}
	}
	s := PhoneState(State{ID: "id", Snapshot: json.RawMessage(`{"hasUnreadTurn":false}`)})
	var raw map[string]any
	_ = json.Unmarshal(s.Snapshot, &raw)
	if raw["hasUnreadTurn"] != false {
		t.Fatal("official read flag dropped")
	}
}

func TestMetadataBoundaryRejectsExecutionAndArbitraryRoots(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, method := range []string{"thread/resume", "turn/start", "turn/steer", "account/read"} {
		if _, e := metadataCall(ctx, config.Config{}, method, nil); e == nil {
			t.Fatal("unallowlisted metadata request", method)
		}
	}
	b := NewBridge(config.Config{CodexHome: t.TempDir()})
	defer b.Close()
	for _, project := range []string{"../../private", "cwd:" + t.TempDir()} {
		if _, e := b.Lifecycle(ctx, "sessions.create", "", project); e == nil {
			t.Fatal("unknown project accepted")
		}
	}
	if _, e := b.Lifecycle(ctx, "sessions.archive", "not-a-thread", ""); e == nil {
		t.Fatal("invalid thread accepted")
	}
}

// Explicit opt-in runs only on a brand-new, isolated CODEX_HOME without tokens.
func TestIsolatedOfficialMetadataLifecycle(t *testing.T) {
	bin := os.Getenv("CXPALM_ISOLATED_METADATA_CLI")
	if bin == "" {
		t.Skip("official CLI isolated integration not requested")
	}
	home := t.TempDir()
	cwd := t.TempDir()
	c := config.Config{CodexHome: home, CodexBin: bin}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, e := metadataCall(ctx, c, "thread/start", map[string]any{"cwd": cwd, "ephemeral": false})
	if e != nil {
		t.Fatal(e)
	}
	var data struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if e = json.Unmarshal(raw, &data); e != nil || !threadID.MatchString(data.Thread.ID) {
		t.Fatal("invalid new thread result", e)
	}
	if _, e = metadataCall(ctx, c, "thread/read", map[string]any{"threadId": data.Thread.ID, "includeTurns": true}); e != nil {
		t.Fatalf("new empty chat not readable after creator exit: %v", e)
	}
	h := History{Config: c}
	if _, e = h.Get(ctx, data.Thread.ID); e != nil {
		t.Fatalf("empty new chat is not persisted: %v", e)
	}
	bridge := NewBridge(c)
	defer bridge.Close()
	result, err := bridge.Rename(ctx, data.Thread.ID, "Renamed isolated conversation")
	if err != nil {
		t.Fatal(err)
	}
	if result.(map[string]any)["title"] != "Renamed isolated conversation" {
		t.Fatal(result)
	}
	for _, method := range []string{"thread/archive", "thread/unarchive"} {
		if _, e = metadataCall(ctx, c, method, map[string]any{"threadId": data.Thread.ID}); e != nil {
			t.Fatal(e)
		}
		row, e := h.Get(ctx, data.Thread.ID)
		if e != nil || row.Archived != (method == "thread/archive") {
			t.Fatalf("%s not confirmed: %+v %v", method, row, e)
		}
	}
}

func TestReplyDateIsRootedAndCacheInvalidates(t *testing.T) {
	home := t.TempDir()
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	path := filepath.Join(home, "rollout.jsonl")
	first := `{"timestamp":"2026-10-11T04:05:06Z","type":"response_item","payload":{"type":"message","role":"assistant"}}` + "\n"
	if e := os.WriteFile(path, []byte(first), 0600); e != nil {
		t.Fatal(e)
	}
	row := Session{RolloutPath: path}
	b.replyDate(&row)
	expected, _ := time.Parse(time.RFC3339, "2026-10-11T04:05:06Z")
	if row.LastReplyAt != expected.Unix() {
		t.Fatal(row.LastReplyAt)
	}
	first += `{"timestamp":"2026-10-12T04:05:06Z","type":"response_item","payload":{"type":"message","role":"assistant"}}` + "\n"
	if e := os.WriteFile(path, []byte(first), 0600); e != nil {
		t.Fatal(e)
	}
	b.replyDate(&row)
	if row.LastReplyAt <= expected.Unix() {
		t.Fatal("stale cached reply date")
	}
	outside := filepath.Join(t.TempDir(), "outside.jsonl")
	_ = os.WriteFile(outside, []byte(first), 0600)
	row = Session{RolloutPath: outside}
	b.replyDate(&row)
	if row.LastReplyAt != 0 {
		t.Fatal("outside history root read")
	}
	raw, _ := json.Marshal(row)
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	if _, ok := fields["RolloutPath"]; ok {
		t.Fatal("private rollout path exposed")
	}
}

func TestArchiveRejectsActiveAndArchivedCannotContinue(t *testing.T) {
	home := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	id := "11111111-1111-4111-8111-111111111111"
	_, e = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER);INSERT INTO threads VALUES(?,'Active','/root',1,'vscode','Codex Desktop',0)`, id)
	if e != nil {
		t.Fatal(e)
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	b.sessions[id] = &State{ID: id, Connected: true, Owner: "desktop", Snapshot: json.RawMessage(`{"threadRuntimeStatus":{"type":"active"}}`)}
	if _, e = b.Lifecycle(context.Background(), "sessions.archive", id, ""); e == nil {
		t.Fatal("active chat archived")
	}
	b.sessions[id].Connected = false
	if _, e = b.Lifecycle(context.Background(), "sessions.archive", id, ""); e == nil {
		t.Fatal("disconnected chat archived")
	}
	if _, e = db.Exec(`UPDATE threads SET archived=1 WHERE id=?`, id); e != nil {
		t.Fatal(e)
	}
	b.sessions[id].Connected = true
	if _, e = b.ControlAction(context.Background(), id, "op", "chat.send", "must not send", "", nil, Options{}, nil); e == nil {
		t.Fatal("archived chat resumed")
	}
}

func TestLiveSessionCatalogueReadOnly(t *testing.T) {
	path := os.Getenv("CXPALM_LIVE_CONFIG")
	if path == "" {
		t.Skip("opt-in read-only local metadata check")
	}
	c, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	b := NewBridge(c)
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, e := b.Browse(ctx, "", false, "", 5000, 0)
	if e != nil {
		t.Fatal(e)
	}
	archived, e := b.Browse(ctx, "", true, "", 5000, 0)
	if e != nil {
		t.Fatal(e)
	}
	dates := 0
	for _, row := range append(page.Items, archived.Items...) {
		if row.LastReplyAt > 0 {
			dates++
		}
	}
	if len(page.Items) > 0 {
		states, remove, e := b.Subscribe(ctx, page.Items[0].ID)
		if e != nil {
			t.Fatal(e)
		}
		defer remove()
		ready := false
		for !ready {
			select {
			case state := <-states:
				ready = state.Connected
			case <-ctx.Done():
				t.Fatal("passive owner snapshot timeout")
			}
		}
		live, e := b.Browse(ctx, "", false, "", 1, 0)
		if e != nil || len(live.Items) != 1 || !live.Items[0].Live {
			t.Fatal("no live summary", e)
		}
	}
	t.Logf("read-only catalogue: active=%d archived=%d projects=%d actual-reply-dates=%d; no writes", len(page.Items), len(archived.Items), len(page.Projects), dates)
}

func TestProjectInheritanceMatchesDesktopRootsWithoutInventingDirectoryProjects(t *testing.T) {
	var m projectMetadata
	if err := json.Unmarshal([]byte(`{"local-projects":{"parent":{"name":"Parent","rootPaths":["D:\\work"]},"child":{"name":"Child","rootPaths":["D:\\work\\child"]},"unix":{"name":"Unix","rootPaths":["/Work"]},"unc":{"name":"Share","rootPaths":["\\\\server\\share"]}},"thread-project-assignments":{"explicit":{"projectKind":"local","projectId":"parent"}},"projectless-thread-ids":["none"]}`), &m); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, cwd, want string }{
		{"new", `\\?\D:\WORK\child\src`, "child"},
		{"exact", `D:\work`, "parent"},
		{"explicit", `D:\work\child`, "parent"},
		{"none", `D:\work\child`, "none"},
		{"boundary", `D:\workspace`, "none"},
		{"outside", `D:\elsewhere`, "none"},
		{"unix", "/Work/src", "unix"},
		{"unix-case", "/work/src", "none"},
		{"share", `\\?\UNC\SERVER\SHARE\src`, "unc"},
		{"empty", "", "none"},
	} {
		row := Session{ID: tc.id, CWD: tc.cwd}
		m.assign(&row)
		if row.ProjectID != tc.want {
			t.Fatalf("%s: project=%q want=%q", tc.id, row.ProjectID, tc.want)
		}
	}
}

func TestRootInheritedChatsAppearInProjectFiltersBeforePagination(t *testing.T) {
	home := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.Exec(`CREATE TABLE threads(id TEXT,title TEXT,cwd TEXT,updated_at INTEGER,source TEXT,originator TEXT,archived INTEGER);
    INSERT INTO threads VALUES('one','One','D:\work',3,'vscode','Codex Desktop',0),('two','Two','\\?\D:\work',2,'vscode','Codex Desktop',0),('archived','Archived','D:\work',1,'vscode','Codex Desktop',1)`)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(home, ".codex-global-state.json"), []byte(`{"local-projects":{"p":{"name":"Work","rootPaths":["D:\\work"]}}}`), 0600); e != nil {
		t.Fatal(e)
	}
	b := NewBridge(config.Config{CodexHome: home})
	defer b.Close()
	page, e := b.Browse(context.Background(), "", false, "p", 1, 0)
	if e != nil || len(page.Items) != 1 || page.Items[0].ID != "one" || !page.HasMore {
		t.Fatalf("first page: %+v %v", page, e)
	}
	page, e = b.Browse(context.Background(), "", false, "p", 1, 1)
	if e != nil || len(page.Items) != 1 || page.Items[0].ID != "two" || page.HasMore {
		t.Fatalf("second page: %+v %v", page, e)
	}
	page, e = b.Browse(context.Background(), "", true, "p", 100, 0)
	if e != nil || len(page.Items) != 1 || page.Items[0].ID != "archived" {
		t.Fatalf("archive: %+v %v", page, e)
	}
}
