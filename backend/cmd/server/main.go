package main

import (
	"caro-ai-pvp/internal/api"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"
)

func main() {
	debug.SetMemoryLimit(2 * 1024 * 1024 * 1024)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	store := api.NewInMemoryStore()
	handler := api.NewHandler(store)
	server := api.NewServer(handler, logger)

	httpServer := &http.Server{
		Addr:    ":5207",
		Handler: server,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
		}
	}()

	cleanupTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for range cleanupTicker.C {
			removed := store.CleanupCompleted()
			if removed > 0 {
				logger.Info("cleanup", "removed", removed)
			}
		}
	}()

	<-quit
	logger.Info("shutting down")
	cleanupTicker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "err", err)
	}

	remaining := store.CleanupAll()
	if remaining > 0 {
		logger.Info("shutdown cleanup", "remaining", remaining)
	}

	fmt.Println("Server stopped")
}
