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
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

const defaultDBPath = "db/caro.db"

const (
	httpReadTimeout   = 10 * time.Second
	httpIdleTimeout   = 60 * time.Second
	httpWriteTimeout  = 0
	serveDrainTimeout = 10 * time.Second
)

var serveSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func newServeServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}
}

func newRootMux(store *server.Store, rooms *server.RoomManager, tourneys server.TourneyService) *http.ServeMux {
	api := server.NewHTTPAPI(store, rooms)
	root := http.NewServeMux()
	root.Handle("/api/", api)
	root.Handle("/static/", api)
	root.Handle("/spike", api)
	server.NewRoomPages(rooms, store).Mount(root)
	server.NewTournamentPages(store, tourneys).Mount(root)
	server.NewLobbyPages(store, rooms).Mount(root)
	root.Handle("/", server.NewShellPages(store, rooms, tourneys))
	return root
}

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
	tourneys := newTourneyService(tourney.NewManager(tourney.NewStore(store), tourney.RoomSource{RM: rooms}), rooms)
	srv := newServeServer(*addr, newRootMux(store, rooms, tourneys))
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
		stop()
	}
	closeServeStack(srv, rooms, wq, hub, store, serveDrainTimeout)
	return code
}

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
