// Package agent owns only the listener and connections it starts.
package agent

import (
	"context"
	"cxpalm/internal/config"
	"cxpalm/internal/server"
	"cxpalm/internal/store"
	"cxpalm/internal/tunnel"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

type Runtime struct {
	tunnelDone chan struct{}
	Done       chan struct{}
	cancel     context.CancelFunc
	http       *http.Server
	bridge     *server.Server
	once       sync.Once
	mu         sync.Mutex
	err        error
}

func Start(parent context.Context, c config.Config, db *store.Store) (*Runtime, error) {
	listener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return nil, errors.New("Agent port is occupied; the existing process was not changed")
	}
	// Recover uncertain receipts only after securing our own listener.
	if err = db.Recover(); err != nil {
		listener.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	bridge := server.New(c, db)
	r := &Runtime{Done: make(chan struct{}), cancel: cancel, bridge: bridge}
	r.http = &http.Server{Handler: bridge.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	r.tunnelDone = make(chan struct{})
	if c.SSHHost != "" {
		go func() { defer close(r.tunnelDone); tunnel.Run(ctx, c) }()
	} else {
		close(r.tunnelDone)
	}
	go func() {
		err := r.http.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		r.mu.Lock()
		r.err = err
		r.mu.Unlock()
		close(r.Done)
	}()
	return r, nil
}
func (r *Runtime) Err() error { r.mu.Lock(); defer r.mu.Unlock(); return r.err }
func (r *Runtime) Stop() {
	r.once.Do(func() {
		r.cancel()
		r.bridge.Bridge.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if r.http.Shutdown(ctx) != nil {
			_ = r.http.Close()
		}
		<-r.Done
		<-r.tunnelDone
	})
}
