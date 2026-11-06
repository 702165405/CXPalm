package desktop

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type failingIPCStream struct {
	writes int
	failAt int
	closed bool
}

func (stream *failingIPCStream) Read([]byte) (int, error) { return 0, io.EOF }
func (stream *failingIPCStream) Write(data []byte) (int, error) {
	stream.writes++
	if stream.writes == stream.failAt {
		return 1, io.ErrUnexpectedEOF
	}
	return len(data), nil
}
func (stream *failingIPCStream) Close() error { stream.closed = true; return nil }

func TestIPCPartialWriteClosesTransportWithoutReplay(t *testing.T) {
	for _, failure := range []struct {
		name string
		at   int
	}{{"header", 1}, {"payload", 2}} {
		t.Run(failure.name, func(t *testing.T) {
			stream := &failingIPCStream{failAt: failure.at}
			client := &IPC{conn: stream, pending: map[string]chan Frame{}, closed: make(chan struct{}), events: make(chan Frame, 1)}
			defer client.Close()
			_, err := client.Request(context.Background(), "thread-follower-interrupt-turn", map[string]any{"conversationId": "fixture"}, "owner")
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("write failure not returned: %v", err)
			}
			select {
			case <-client.Done():
			default:
				t.Fatal("partial frame left the transport reusable")
			}
			if !stream.closed || len(client.pending) != 0 {
				t.Fatal("failed transport or request was not released")
			}
			if err = client.Follow(context.Background(), "fixture", "owner"); err == nil {
				t.Fatal("a later request reused the damaged stream")
			}
			if stream.writes != failure.at {
				t.Fatal("failed request replayed or another frame appended")
			}
		})
	}
}

func readTestFrame(c net.Conn) (Frame, error) {
	var h [4]byte
	if _, e := io.ReadFull(c, h[:]); e != nil {
		return Frame{}, e
	}
	b := make([]byte, binary.LittleEndian.Uint32(h[:]))
	_, e := io.ReadFull(c, b)
	if e != nil {
		return Frame{}, e
	}
	var f Frame
	e = json.Unmarshal(b, &f)
	return f, e
}
func TestIPCHandshakeAndAllowlist(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	go func() {
		f, e := readTestFrame(b)
		if e != nil {
			return
		}
		r, _ := json.Marshal(Frame{Type: "response", RequestID: f.RequestID, ResultType: "success", Result: json.RawMessage(`{"clientId":"fixture"}`)})
		var h [4]byte
		binary.LittleEndian.PutUint32(h[:], uint32(len(r)))
		writeAll(b, h[:])
		writeAll(b, r)
		io.Copy(io.Discard, b)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, e := New(ctx, a)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if _, e = c.Request(ctx, "turn/start", map[string]any{}, ""); e == nil {
		t.Fatal("arbitrary executor RPC was accepted")
	}
}
func TestOversizeIPCFrameCloses(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	go func() {
		readTestFrame(b)
		var h [4]byte
		binary.LittleEndian.PutUint32(h[:], maxFrame+1)
		b.Write(h[:])
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if c, e := New(ctx, a); e == nil {
		c.Close()
		t.Fatal("oversized frame accepted")
	}
}
func TestPatchPaths(t *testing.T) {
	b, e := normalizePatches(json.RawMessage(`[{"op":"replace","path":["a/b",0],"value":"x"}]`))
	if e != nil {
		t.Fatal(e)
	}
	var patches []map[string]any
	if e = json.Unmarshal(b, &patches); e != nil {
		t.Fatal(e)
	}
	if patches[0]["path"] != "/a~1b/0" {
		t.Fatal(string(b))
	}
	if _, e = normalizePatches(json.RawMessage(`[{"op":"move","path":"/x"}]`)); e == nil {
		t.Fatal("unsupported patch accepted")
	}
}
func TestCompletedWriteSurvivesCallerCancellation(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	c := &IPC{conn: a, pending: map[string]chan Frame{}, closed: make(chan struct{}), events: make(chan Frame, 1)}
	defer c.Close()
	go io.Copy(io.Discard, b)
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		e := c.Follow(ctx, "thread", "")
		cancel()
		if e != nil {
			t.Fatal(e)
		}
		time.Sleep(time.Millisecond)
		select {
		case <-c.Done():
			t.Fatal("completed send closed transport when caller canceled its finished request")
		default:
		}
	}
}

func TestLargeOfficialSnapshotFrame(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	payload := strings.Repeat("x", 20<<20)
	go func() {
		f, err := readTestFrame(b)
		if err != nil {
			return
		}
		response, _ := json.Marshal(Frame{Type: "response", RequestID: f.RequestID, ResultType: "success", Result: json.RawMessage(`{"clientId":"fixture"}`)})
		event, _ := json.Marshal(Frame{Type: "event", Params: json.RawMessage(`{"snapshot":"` + payload + `"}`)})
		for _, data := range [][]byte{response, event} {
			var h [4]byte
			binary.LittleEndian.PutUint32(h[:], uint32(len(data)))
			if writeAll(b, h[:]) != nil || writeAll(b, data) != nil {
				return
			}
		}
		io.Copy(io.Discard, b)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := New(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	select {
	case frame, ok := <-c.Events():
		if !ok || len(frame.Params) <= 16<<20 {
			t.Fatal("large official snapshot was rejected")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-c.Done():
		t.Fatal("snapshot closed IPC")
	default:
	}
}
