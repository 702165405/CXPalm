package desktop

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestNewChatSettingsValidation(t *testing.T) {
	catalog := Catalog{Models: []Model{{ID: "fixture", Efforts: []string{"high"}}}}
	for _, o := range []Options{{Model: "missing"}, {Effort: "high"}, {Model: "fixture", Effort: "ultra"}, {Permission: "full"}, {Mode: "plan"}, {Expected: "old"}, {GoalAction: "pause"}} {
		if err := validateNewChatOptions(o, catalog); err == nil {
			t.Fatalf("invalid settings accepted: %+v", o)
		}
	}
	for _, permission := range []string{"read-only", "workspace", "auto-review", "full"} {
		if err := validateNewChatOptions(Options{Model: "fixture", Effort: "high", Permission: permission, ConfirmFullAccess: permission == "full"}, catalog); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateNewChatOptions(Options{}, catalog); err != nil {
		t.Fatal(err)
	}
}

func TestNewChatInitializationUsesOriginalOwnerOnce(t *testing.T) {
	for _, outcome := range []string{"accepted", "rejected", "lost", "not-empty"} {
		t.Run(outcome, func(t *testing.T) {
			b, raw, _ := controlFixture(t)
			a, peer := net.Pipe()
			defer peer.Close()
			b.ipc = &IPC{conn: a, pending: map[string]chan Frame{}, closed: make(chan struct{}), events: make(chan Frame, 4), clientID: "fixture"}
			go b.ipc.read()
			defer b.Close()
			if outcome == "not-empty" {
				raw["turns"] = []any{map[string]any{"turnId": "already-started"}}
			}
			b.sessions["thread"] = &State{ID: "thread", Connected: true, Owner: "original-owner", Snapshot: mustJSON(raw)}
			observed := make(chan Frame, 2)
			go func() {
				for {
					f, err := readTestFrame(peer)
					if err != nil {
						return
					}
					observed <- f
					if outcome == "lost" {
						peer.Close()
						return
					}
					encoded, _ := json.Marshal(Frame{Type: "response", RequestID: f.RequestID, ResultType: "success", Result: mustJSON(map[string]any{"applied": outcome == "accepted"})})
					var header [4]byte
					binary.LittleEndian.PutUint32(header[:], uint32(len(encoded)))
					writeAll(peer, header[:])
					writeAll(peer, encoded)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := b.initializeCreatedChat(ctx, "thread", Options{Model: "fixture", Effort: "high", Permission: "auto-review"})
			if (err == nil) != (outcome == "accepted") {
				t.Fatalf("unexpected outcome: %v", err)
			}
			if outcome == "not-empty" {
				select {
				case <-observed:
					t.Fatal("nonempty chat mutated")
				default:
				}
				return
			}
			f := <-observed
			if f.Target != "original-owner" || f.Method != "thread-follower-update-thread-settings" || f.Version != 2 {
				t.Fatal("settings escaped original owner")
			}
			var p map[string]any
			json.Unmarshal(f.Params, &p)
			settings := p["threadSettings"].(map[string]any)
			if settings["model"] != "fixture" || settings["effort"] != "high" || settings["permissions"] != ":workspace" || settings["approvalsReviewer"] != "auto_review" {
				t.Fatal(settings)
			}
			select {
			case <-observed:
				t.Fatal("settings mutation replayed")
			case <-time.After(30 * time.Millisecond):
			}
		})
	}
}
