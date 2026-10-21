package desktop

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

func TestApprovalDecisionsStayWithinOriginalRequest(t *testing.T) {
	for _, tc := range []struct {
		method, params, choice string
		allowed                bool
	}{
		{"item/permissions/requestApproval", `{"threadId":"thread","permissions":{"network":{"enabled":true},"fileSystem":{"read":["/fixture"]}}}`, "accept", true},
		{"item/permissions/requestApproval", `{"threadId":"thread","permissions":{"future":true}}`, "accept", false},
		{"item/permissions/requestApproval", `{"threadId":"thread","permissions":{"future":true}}`, "decline", true},
		{"item/permissions/requestApproval", `{"threadId":"other","permissions":{}}`, "accept", false},
		{"item/permissions/requestApproval", `{"threadId":"thread","permissions":{}}`, "cancel", false},
		{"item/permissions/requestApproval", `{}`, "accept", false},
		{"item/commandExecution/requestApproval", `{"availableDecisions":["decline"]}`, "accept", false},
		{"item/commandExecution/requestApproval", `{"availableDecisions":[]}`, "accept", false},
		{"item/commandExecution/requestApproval", `{"availableDecisions":[{"acceptForSession":{}}]}`, "accept", false},
		{"item/commandExecution/requestApproval", `{"availableDecisions":["cancel"]}`, "cancel", true},
		{"item/fileChange/requestApproval", `{}`, "accept", true},
		{"item/fileChange/requestApproval", `{}`, "acceptForSession", false},
		{"unknown", `{}`, "accept", false},
	} {
		t.Run(tc.method+tc.params+tc.choice, func(t *testing.T) {
			var params map[string]any
			json.Unmarshal([]byte(tc.params), &params)
			raw := map[string]any{"requests": []any{map[string]any{"id": float64(12), "method": tc.method, "params": params}}}
			choice, _ := json.Marshal(tc.choice)
			_, payload, err := approvalAction(raw, "thread", "12", choice)
			if (err == nil) != tc.allowed {
				t.Fatalf("decision allowed=%v: %v", tc.allowed, err)
			}
			if err == nil && tc.method == "item/permissions/requestApproval" {
				response := payload["response"].(map[string]any)
				if response["scope"] != "turn" {
					t.Fatal("grant escaped turn scope")
				}
				grants, _ := json.Marshal(response["permissions"])
				requested, _ := json.Marshal(params["permissions"])
				if tc.choice == "accept" && string(grants) != string(requested) {
					t.Fatal("grant differs from request")
				}
				if tc.choice == "decline" && string(grants) != "{}" {
					t.Fatal("denial granted permissions")
				}
			}
			if _, _, err := approvalAction(raw, "thread", "expired", choice); err == nil {
				t.Fatal("expired approval accepted")
			}
		})
	}
}

func TestPermissionApprovalUsesOriginalOwnerIPC(t *testing.T) {
	for _, choice := range []string{"accept", "decline"} {
		t.Run(choice, func(t *testing.T) {
			b, raw, _ := controlFixture(t)
			defer b.Close()
			var requests []any
			json.Unmarshal([]byte(`[{"id":42,"method":"item/permissions/requestApproval","params":{"threadId":"thread","permissions":{"network":{"enabled":true}}}}]`), &requests)
			raw["requests"] = requests
			a, peer := net.Pipe()
			defer peer.Close()
			b.ipc = &IPC{conn: a, pending: map[string]chan Frame{}, closed: make(chan struct{}), events: make(chan Frame, 4), clientID: "fixture"}
			go b.ipc.read()
			encoded, _ := json.Marshal(raw)
			b.sessions["thread"] = &State{ID: "thread", Owner: "original-owner", Connected: true, Snapshot: encoded}
			observed := make(chan Frame, 1)
			go func() {
				f, err := readTestFrame(peer)
				if err != nil {
					return
				}
				observed <- f
				data, _ := json.Marshal(Frame{Type: "response", RequestID: f.RequestID, ResultType: "success", Result: json.RawMessage(`{"ok":true}`)})
				var header [4]byte
				binary.LittleEndian.PutUint32(header[:], uint32(len(data)))
				writeAll(peer, header[:])
				writeAll(peer, data)
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			decision, _ := json.Marshal(choice)
			if _, err := b.Action(ctx, "thread", "expired-operation", "chat.approve", "", "expired", decision); err == nil {
				t.Fatal("expired approval dispatched")
			} else {
				var rejected *Rejected
				if !errors.As(err, &rejected) {
					t.Fatal("known unexecuted approval would remain uncertain")
				}
			}
			if _, err := b.Action(ctx, "thread", "operation", "chat.approve", "", "42", decision); err != nil {
				t.Fatal(err)
			}
			f := <-observed
			if f.Target != "original-owner" || f.Method != "thread-follower-permissions-request-approval-response" || f.Version != 1 {
				t.Fatal("wrong approval owner/protocol")
			}
			var p map[string]any
			json.Unmarshal(f.Params, &p)
			if p["requestId"] != float64(42) || p["response"].(map[string]any)["scope"] != "turn" {
				t.Fatal("request identity or grant scope changed")
			}
		})
	}
}
