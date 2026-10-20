package desktop

import (
	"context"
	"cxpalm/internal/attachments"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAttachmentSteerUsesOriginalOwnerAndDurableDisplayReceipt(t *testing.T) {
	b, device, peer := queueFixture(t)
	b.Uploads = &attachments.Manager{Root: filepath.Join(t.TempDir(), "uploads")}
	f, err := b.Uploads.Save(device, "thread", "notes.txt", strings.NewReader("attachment contents"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := b.Uploads.Resolve(device, "thread", []string{f.ID})
	if err != nil {
		t.Fatal(err)
	}
	raw := map[string]any{"threadRuntimeStatus": map[string]any{"type": "active"}, "turns": []any{map[string]any{"turnId": "turn-a", "status": "inProgress", "items": []any{}}}}
	b.sessions["thread"].Snapshot = mustJSON(raw)
	observed := make(chan Frame, 1)
	go func() {
		frame, err := readTestFrame(peer)
		if err != nil {
			return
		}
		observed <- frame
		encoded, _ := json.Marshal(Frame{Type: "response", RequestID: frame.RequestID, ResultType: "success", Result: json.RawMessage(`{"result":{"turnId":"turn-a"}}`)})
		var header [4]byte
		binary.LittleEndian.PutUint32(header[:], uint32(len(encoded)))
		writeAll(peer, header[:])
		writeAll(peer, encoded)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = b.ControlActionFiles(ctx, "thread", "steer-id", "chat.steer", "use these notes", "", nil, Options{}, nil, files); err != nil {
		t.Fatal(err)
	}
	frame := <-observed
	if frame.Target != "original-owner" || frame.Method != "thread-follower-steer-turn" || !strings.Contains(string(frame.Params), files[0].Name) {
		t.Fatal("attachment escaped native owner", frame.Method)
	}
	messages, err := b.ReadStore.SteerMessages("thread")
	if err != nil || len(messages) != 1 || messages[0].Turn != "turn-a" {
		t.Fatal(messages, err)
	}
	if !strings.Contains(messages[0].Text, "use these notes") || !strings.Contains(messages[0].Text, files[0].Name) {
		t.Fatal("guidance display lost text or attachment")
	}
	if _, err = b.ReadStore.EnqueueFiles(device, "queue-id", "thread", "", nil, []string{f.ID}); err != nil {
		t.Fatal(err)
	}
	rows, err := b.ReadStore.Queue("thread")
	if err != nil || len(rows) != 1 || len(rows[0].Attachments) != 1 || rows[0].Attachments[0] != f.ID {
		t.Fatal("queued attachment reference lost", rows, err)
	}
}
