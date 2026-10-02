package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

const defaultDBPath = "caro.db"

// HTTP lifecycle knobs of the serve command, named locals until the config
// hub carries server constants (gap reported with the milestone).
//
// ReadTimeout bounds reading one request, headers and body: a slow or
// silent client cannot pin a connection on the read side, and an SSE
// stream, which reads nothing after its headers, is never cut by it.
// IdleTimeout bounds keep-alive connections between requests.
// WriteTimeout stays 0 by decision: it deadlines the whole response, not
// idle periods, so every finite value would kill healthy SSE streams
// mid-series; a vanished stream reader is instead released by its request
// context and the drain backstop below. serveDrainTimeout bounds the
// graceful drain after the stop signal before the backstop cuts the rest.
const (
	httpReadTimeout   = 10 * time.Second
	httpIdleTimeout   = 60 * time.Second
	httpWriteTimeout  = 0
	serveDrainTimeout = 10 * time.Second
)

// serveSignals is the stop set of the serve command: the terminal Ctrl+C
// and the orchestrator's termination signal. A package var so the test
// freezes the set (signals cannot be raised at the process on Windows).
var serveSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// newServeServer builds the serve command's HTTP server with the
// lifecycle knobs above. A helper so the timeout configuration stays
// assertable without process control.
func newServeServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}
}

// runServe boots the full server: store with startup self-migration, the
// single-writer mutation queue, the pub-sub hub, the room manager, and the
// HTTP/SSE transport on the configured port. SIGINT or SIGTERM drains:
// in-flight requests get serveDrainTimeout to finish, the backstop cuts
// whatever outlives the window, and the stack closes in reverse boot
// order. A second signal during the drain force-exits: the first signal
// restores the default disposition.
func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDBPath, "SQLite database path")
	addr := fs.String("addr", fmt.Sprintf(":%d", config.HTTPPort), "listen address")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	store, err := server.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
		return 1
	}
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rooms := server.NewRoomManager(hub, store, wq)
	// Provisional M6b composition: the JSON/SSE API keeps its subtree
	// prefixes, the shell pages own the root. The M6b lead recomposes this
	// mux when the room and playback pages land.
	api := server.NewHTTPAPI(store, rooms)
	root := http.NewServeMux()
	root.Handle("/api/", api)
	root.Handle("/static/", api)
	root.Handle("/spike", api)
	root.Handle("/", server.NewShellPages(store, rooms))
	srv := newServeServer(*addr, root)

	ctx, stop := signal.NotifyContext(context.Background(), serveSignals...)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	fmt.Fprintf(os.Stderr, "caro: serving on %s (db %s)\n", *addr, *dbPath)

	var code int
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "caro:", err)
			code = 1
		}
	case <-ctx.Done():
		// First signal observed: restore the default disposition so a
		// second signal during the drain force-exits instead of being
		// swallowed by the drained notifier.
		stop()
	}
	closeServeStack(srv, rooms, wq, hub, store, serveDrainTimeout)
	return code
}

// closeServeStack is the ordered shutdown behind both exit arms: the HTTP
// server drains in-flight requests within drain, then the Close backstop
// cuts what outlives the window (an SSE stream past it included), and the
// rooms, queue, hub, and store close in reverse boot order. Extracted so
// the drain contract is testable without signals.
func closeServeStack(srv *http.Server, rooms *server.RoomManager, wq *server.WriteQueue,
	hub *server.Hub, store *server.Store, drain time.Duration) {
	dctx, cancel := context.WithTimeout(context.Background(), drain)
	defer cancel()
	if err := srv.Shutdown(dctx); err != nil {
		_ = srv.Close()
	}
	rooms.Shutdown()
	wq.Close()
	hub.Close()
	if err := store.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
	}
}

// runMigrate applies pending startup migrations and exits: the store's Open
// runs the migration chain, so this opens and closes.
func runMigrate(args []string) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDBPath, "SQLite database path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	store, err := server.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
		return 1
	}
	if err := store.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "caro: migrations applied to %s\n", *dbPath)
	return 0
}
