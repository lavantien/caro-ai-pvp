package api

import (
	"log/slog"
	"net/http"
)

func NewServer(handler *Handler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/game/new", handler.CreateGame)
	mux.HandleFunc("GET /api/game/{id}", handler.GetGame)
	mux.HandleFunc("POST /api/game/{id}/move", handler.MakeMove)
	mux.HandleFunc("POST /api/game/{id}/ai-move", handler.MakeAIMove)
	mux.HandleFunc("POST /api/game/{id}/undo", handler.UndoMove)
	mux.HandleFunc("DELETE /api/game/{id}", handler.DeleteGame)
	mux.HandleFunc("GET /ws/uci", func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(logger, w, r)
	})

	var h http.Handler = mux
	h = CORSMiddleware(h)
	h = LoggingMiddleware(logger, h)
	h = RecoveryMiddleware(logger, h)

	return h
}
