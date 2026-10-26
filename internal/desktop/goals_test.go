package desktop

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeGoalProjectionAndStaleCommands(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	path := filepath.Join(b.config.CodexHome, "goals_1.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE thread_goals(thread_id TEXT PRIMARY KEY,goal_id TEXT,objective TEXT,status TEXT,token_budget INTEGER,tokens_used INTEGER,time_used_seconds INTEGER,created_at_ms INTEGER,updated_at_ms INTEGER);
 INSERT INTO thread_goals VALUES('thread','goal-one','fixture objective','active',NULL,5,1,1000,2000);`)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	goal, revision, err := b.nativeGoal("thread")
	if err != nil || goal["objective"] != "fixture objective" || revision == "" {
		t.Fatal(goal, revision, err)
	}
	before, _ := os.ReadFile(path)
	_, err = b.prepareGoal(context.Background(), "thread", "", Options{GoalAction: "clear", ExpectedGoal: "stale"}, map[string]any{})
	if err == nil {
		t.Fatal("stale goal mutation accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("read-only goal path wrote official storage")
	}
	if _, err = b.prepareGoal(context.Background(), "thread", "new", Options{ExpectedGoal: revision}, map[string]any{}); err == nil {
		t.Fatal("existing goal replaced")
	}
	out := b.PhoneState(State{ID: "thread", Snapshot: json.RawMessage(`{"turns":[]}`)})
	var raw map[string]any
	json.Unmarshal(out.Snapshot, &raw)
	if raw["goalRevision"] != revision || raw["goalAvailable"] != true || raw["threadGoal"] == nil {
		t.Fatal("native goal projection missing")
	}
	// Accounting updates are not identity changes; controls remain usable mid-turn.
	db.Exec(`UPDATE thread_goals SET tokens_used=9,updated_at_ms=3000 WHERE thread_id='thread'`)
	_, next, _ := b.nativeGoal("thread")
	if next != revision {
		t.Fatal("accounting changed goal identity")
	}
	db.Exec(`UPDATE thread_goals SET status='paused' WHERE thread_id='thread'`)
	_, next, _ = b.nativeGoal("thread")
	if next == revision {
		t.Fatal("lifecycle change did not invalidate controls")
	}
}

// The child is this test binary, not an official runtime or a real conversation.
func init() {
	path := os.Getenv("CXPALM_GOAL_FIXTURE_DB")
	if path == "" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var q struct {
			ID     any            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &q) != nil {
			os.Exit(2)
		}
		if q.Method == "initialized" {
			continue
		}
		result := map[string]any{}
		switch q.Method {
		case "initialize":
		case "thread/goal/set":
			db, e := sql.Open("sqlite", path)
			if e != nil {
				os.Exit(3)
			}
			_, e = db.Exec(`INSERT OR REPLACE INTO thread_goals VALUES(?,?,?,? ,NULL,0,0,1000,1000)`, q.Params["threadId"], "fixture-goal", q.Params["objective"], q.Params["status"])
			db.Close()
			if e != nil {
				os.Exit(4)
			}
			result["goal"] = map[string]any{"objective": q.Params["objective"], "status": "active"}
		default:
			os.Exit(5) // No executor methods can pass the metadata boundary.
		}
		encoder.Encode(map[string]any{"id": q.ID, "result": result})
	}
	os.Exit(0)
}
func TestGoalCreationUsesNativeMetadataAndOnlyOriginalOwnerForActivation(t *testing.T) {
	b, raw, _ := controlFixture(t)
	defer b.Close()
	path := filepath.Join(b.config.CodexHome, "goals_1.sqlite")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`CREATE TABLE thread_goals(thread_id TEXT PRIMARY KEY,goal_id TEXT,objective TEXT,status TEXT,token_budget INTEGER,tokens_used INTEGER,time_used_seconds INTEGER,created_at_ms INTEGER,updated_at_ms INTEGER)`)
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("CXPALM_GOAL_FIXTURE_DB", path)
	executable, _ := os.Executable()
	b.config.CodexBin = executable
	a, peer := net.Pipe()
	defer peer.Close()
	ipc := &IPC{conn: a, pending: map[string]chan Frame{}, closed: make(chan struct{}), events: make(chan Frame, 4), clientID: "fixture"}
	go ipc.read()
	b.ipc = ipc
	data, _ := json.Marshal(raw)
	b.sessions["thread"] = &State{ID: "thread", Owner: "original-owner", Connected: true, Snapshot: data}
	observed := make(chan Frame, 1)
	go func() {
		f, err := readTestFrame(peer)
		if err != nil {
			return
		}
		observed <- f
		encoded, _ := json.Marshal(Frame{Type: "response", RequestID: f.RequestID, ResultType: "success", Result: json.RawMessage(`{"applied":true}`)})
		var h [4]byte
		binary.LittleEndian.PutUint32(h[:], uint32(len(encoded)))
		writeAll(peer, h[:])
		writeAll(peer, encoded)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, e = b.ControlAction(ctx, "thread", "goal-operation", "chat.goal", "isolated objective", "", nil, Options{ExpectedGoal: "none"}, nil); e != nil {
		t.Fatal(e)
	}
	f := <-observed
	if f.Method != "thread-follower-start-turn" || f.Target != "original-owner" || !strings.Contains(string(f.Params), `"clientUserMessageId":"goal-operation"`) {
		t.Fatal("goal activation escaped original owner")
	}
	goal, _, e := b.nativeGoal("thread")
	if e != nil || goal["objective"] != "isolated objective" || goal["status"] != "active" {
		t.Fatal("goal was only decorative", e)
	}
}
