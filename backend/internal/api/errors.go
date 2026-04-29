package api

import (
	"caro-ai-pvp/internal/domain"
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case err == domain.ErrGameNotFound:
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: err.Error()})
	case err == domain.ErrTooManyGames:
		writeJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "too_many_games", Message: err.Error()})
	case err == domain.ErrCellOccupied, err == domain.ErrPositionBounds,
		err == domain.ErrGameOver, err == domain.ErrOpenRule,
		err == domain.ErrInvalidLevel:
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal", Message: "Internal server error"})
	}
}
