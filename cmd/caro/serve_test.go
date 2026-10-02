package main

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// The serve lifecycle tests pin what is reachable without process control
// (Windows cannot raise SIGINT or SIGTERM at the test process): the static
// server configuration through the newServeServer helper, the signal set
// through serveSignals, and the drain/close contract through closeServeStack
// and a real port-in-use boot. The signal-driven arms of runServe are
// code-verified only.

// TestServeServerLifecycleKnobs freezes the timeout configuration: a
// positive ReadTimeout and IdleTimeout bound slow or idle clients, while
// WriteTimeout stays 0 by decision because any finite value would cut
// healthy SSE streams mid-series (it deadlines the whole response, not
// idle periods).
func TestServeServerLifecycleKnobs(t *testing.T) {
	srv := newServeServer(":0", http.NotFoundHandler())
	if srv.ReadTimeout <= 0 {
		t.Errorf("ReadTimeout = %v, want a positive bound against slow clients", srv.ReadTimeout)
	}
	if srv.IdleTimeout <= 0 {
		t.Errorf("IdleTimeout = %v, want a positive bound against idle keep-alives", srv.IdleTimeout)
	}
	if srv.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0: finite values kill healthy SSE streams", srv.WriteTimeout)
	}
}

// TestServeSignalsAreInterruptAndSIGTERM freezes the stop set: SIGTERM
// rides along Ctrl+C so an orchestrator's termination drains too.
func TestServeSignalsAreInterruptAndSIGTERM(t *testing.T) {
	interrupt, sigterm := false, false
	for _, sig := range serveSignals {
		interrupt = interrupt || sig == os.Interrupt
		sigterm = sigterm || sig == syscall.SIGTERM
	}
	if !interrupt || !sigterm {
		t.Errorf("serveSignals = %v, want os.Interrupt and syscall.SIGTERM", serveSignals)
	}
}

// TestServePortInUseExitsOneClosesStore boots the real command against an
// occupied port: the listen failure must exit 1 with the whole stack
// closed behind it, the store included (its checkpointing close leaves no
// populated WAL sidecar).
func TestServePortInUseExitsOneClosesStore(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer func() { _ = ln.Close() }()

	db := filepath.Join(t.TempDir(), "caro.db")
	if code := runServe([]string{"-db", db, "-addr", ln.Addr().String()}); code != 1 {
		t.Fatalf("serve exit = %d, want 1 on the occupied port", code)
	}
	if fi, err := os.Stat(db + "-wal"); err == nil && fi.Size() > 0 {
		t.Fatalf("wal sidecar holds %d bytes after serve exit, want a closed store", fi.Size())
	}
}

// TestServeCloseStackDrainsThenBackstopsAndClosesEverything pins the
// shutdown contract of closeServeStack: a lingering SSE stream holds the
// graceful drain for the full window (a bare Close would return at once),
// the backstop cuts it, and every stack component answers closed after.
func TestServeCloseStackDrainsThenBackstopsAndClosesEverything(t *testing.T) {
	st, err := server.Open(filepath.Join(t.TempDir(), "caro.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rooms := server.NewRoomManager(hub, st, wq)
	srv := newServeServer("", server.NewHTTPAPI(st, rooms))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()

	// One lingering spectator stream: the events route is public, so the
	// room needs no account (owner 1, an open pvp room).
	room, err := rooms.Create(1, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+ln.Addr().String()+"/api/rooms/"+room.ID()+"/events", nil)
	if err != nil {
		t.Fatalf("new sse request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open sse: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	const drain = 250 * time.Millisecond
	start := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		closeServeStack(srv, rooms, wq, hub, st, drain)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("closeServeStack never returned: the drain is unbounded")
	}
	if el := time.Since(start); el < drain {
		t.Errorf("close elapsed %v, want at least the drain window %v the stream holds", el, drain)
	}

	// The backstop actually cut the lingering stream: the read returns
	// instead of parking until the request deadline.
	readDone := make(chan error, 1)
	go func() {
		_, rerr := bufio.NewReader(resp.Body).Read(make([]byte, 1))
		readDone <- rerr
	}()
	select {
	case <-readDone:
	case <-time.After(5 * time.Second):
		t.Fatal("sse stream still open after the backstop close")
	}

	if err := <-serveDone; err != nil && err != http.ErrServerClosed {
		t.Errorf("srv.Serve = %v, want ErrServerClosed", err)
	}
	if _, err := hub.Subscribe("x"); !errors.Is(err, server.ErrHubClosed) {
		t.Errorf("subscribe after close = %v, want ErrHubClosed", err)
	}
	if err := wq.Send(func(context.Context) error { return nil }); !errors.Is(err, server.ErrQueueClosed) {
		t.Errorf("send after close = %v, want ErrQueueClosed", err)
	}
	if _, err := rooms.Get(room.ID()); !errors.Is(err, server.ErrRoomNotFound) {
		t.Errorf("room after shutdown = %v, want ErrRoomNotFound", err)
	}
	if _, err := st.UserStats(1); err == nil {
		t.Error("stats read on the closed store succeeded, want the pool-closed error")
	}
}
