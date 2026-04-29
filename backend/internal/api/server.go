package api

import (
	"log/slog"
	"net/http"
)

func NewServer(handler *Handler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/games", handler.CreateGame)
	mux.HandleFunc("GET /api/games/{id}", handler.GetGame)
	mux.HandleFunc("POST /api/games/{id}/moves", handler.MakeMove)
	mux.HandleFunc("POST /api/games/{id}/ai-moves", handler.MakeAIMove)
	mux.HandleFunc("POST /api/games/{id}/undo", handler.UndoMove)
	mux.HandleFunc("DELETE /api/games/{id}", handler.DeleteGame)
	mux.HandleFunc("GET /ws/uci", func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(logger, w, r)
	})

	var h http.Handler = mux
	h = CORSMiddleware(h)
	h = LoggingMiddleware(logger, h)
	h = RecoveryMiddleware(logger, h)

	return h
}
