package desktop

import (
	"context"
	"cxpalm/internal/config"
	"encoding/binary"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type frameSizeProbe struct {
	io.ReadWriteCloser
	header [4]byte
	have   int
	left   uint32
	max    atomic.Uint32
}

func (p *frameSizeProbe) Read(b []byte) (int, error) {
	n, err := p.ReadWriteCloser.Read(b)
	for data := b[:n]; len(data) > 0; {
		if p.left > 0 {
			take := min(int(p.left), len(data))
			p.left -= uint32(take)
			data = data[take:]
			continue
		}
		take := min(4-p.have, len(data))
		copy(p.header[p.have:], data[:take])
		p.have += take
		data = data[take:]
		if p.have == 4 {
			p.left = binary.LittleEndian.Uint32(p.header[:])
			p.have = 0
			if p.left > p.max.Load() {
				p.max.Store(p.left)
			}
		}
	}
	return n, err
}
func TestLiveIPCFrameSizeReadOnly(t *testing.T) {
	path := os.Getenv("CXPALM_FRAME_PROBE_CONFIG")
	if path == "" {
		t.Skip("opt-in frame length probe; no content logging")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := (History{Config: cfg}).List(ctx, "", false)
	if err != nil || len(rows) == 0 {
		t.Fatal("no history", err)
	}
	conn, err := dial(ctx, cfg.IPCPath)
	if err != nil {
		t.Fatal(err)
	}
	probe := &frameSizeProbe{ReadWriteCloser: conn}
	client, err := New(ctx, probe)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err = client.Follow(ctx, rows[0].ID, ""); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case _, ok := <-client.Events():
			if !ok {
				t.Fatalf("native connection closed: largest frame=%d limit=%d", probe.max.Load(), maxFrame)
				return
			}
		case <-ctx.Done():
			select {
			case <-client.Done():
				t.Fatal("native connection closed")
			default:
			}
			if probe.max.Load() == 0 {
				t.Fatal("no native frames received")
			}
			t.Logf("largest frame=%d limit=%d connectionStillOpen=true", probe.max.Load(), maxFrame)
			return
		}
	}
}
