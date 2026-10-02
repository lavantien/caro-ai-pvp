package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

const defaultDBPath = "caro.db"

// runServe boots the full server: store with startup self-migration, the
// single-writer mutation queue, the pub-sub hub, the room manager, and the
// HTTP/SSE transport on the configured port. SIGINT drains and closes
// everything in reverse order.
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
	srv := &http.Server{Addr: *addr, Handler: server.NewHTTPAPI(store, rooms)}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
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
	}
	_ = srv.Close()
	rooms.Shutdown()
	wq.Close()
	hub.Close()
	if err := store.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
	}
	return code
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
