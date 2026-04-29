package main

import (
	"caro-ai-pvp/internal/uci"
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := uci.NewUCIHandler(logger, os.Stdout)
	uci.RunUCILoop(handler, os.Stdin)
}
