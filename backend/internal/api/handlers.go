package api

import (
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type Handler struct {
	store  *InMemoryStore
	logger interface {
		Info(msg string, args ...any)
	}
}

func NewHandler(store *InMemoryStore) *Handler {
	return &Handler{store: store}
}

func newGameID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Handler) CreateGame(w http.ResponseWriter, r *http.Request) {
	if h.store.Count() >= domain.MaxConcurrentGames {
		writeError(w, domain.ErrTooManyGames)
		return
	}

	var req CreateGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
		return
	}

	timeControl := "7+5"
	initialTimeMs := int64(420000)
	incrementSeconds := 5
	switch req.TimeControl {
	case "1+0", "bullet":
		timeControl, initialTimeMs, incrementSeconds = "1+0", 60000, 0
	case "3+2", "blitz":
		timeControl, initialTimeMs, incrementSeconds = "3+2", 180000, 2
	case "15+10", "classical":
		timeControl, initialTimeMs, incrementSeconds = "15+10", 900000, 10
	}

	gameMode := domain.ParseGameMode(req.GameMode)
	redDiff := req.RedDifficulty
	blueDiff := req.BlueDifficulty
	if req.Difficulty != nil {
		if redDiff == nil {
			d := *req.Difficulty
			redDiff = &d
		}
		if blueDiff == nil {
			d := *req.Difficulty
			blueDiff = &d
		}
	}

	if redDiff != nil && (*redDiff < 1 || *redDiff > 5) {
		writeError(w, domain.ErrInvalidLevel)
		return
	}
	if blueDiff != nil && (*blueDiff < 1 || *blueDiff > 5) {
		writeError(w, domain.ErrInvalidLevel)
		return
	}

	gameID := newGameID()
	session := NewGameSession(timeControl, initialTimeMs, incrementSeconds, gameMode, redDiff, blueDiff, nil, func() int {
		return h.store.ActiveGameCount()
	})
	h.store.Set(gameID, session)

	writeJSON(w, http.StatusOK, map[string]any{
		"game_id": gameID,
		"state":   session.GetResponse(),
	})
}

func (h *Handler) GetGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, ok := h.store.Get(id)
	if !ok {
		writeError(w, domain.ErrGameNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": session.GetResponse()})
}

func (h *Handler) MakeMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, ok := h.store.Get(id)
	if !ok {
		writeError(w, domain.ErrGameNotFound)
		return
	}

	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: err.Error()})
		return
	}

	resp, err := session.ApplyMove(req.X, req.Y)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": resp})
}

func (h *Handler) MakeAIMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, ok := h.store.Get(id)
	if !ok {
		writeError(w, domain.ErrGameNotFound)
		return
	}

	board, player, isGameOver, timeRemainingMs, incrementSeconds, moveNumber, difficulty := session.ExtractForAI()
	if isGameOver {
		writeError(w, domain.ErrGameOver)
		return
	}

	ai := session.GetOrCreateAI(player)

	var opts engine.SearchOptions
	if difficulty != nil && *difficulty >= 1 && *difficulty <= 5 {
		profile := engine.GetDifficultyProfile(*difficulty)
		opts = engine.SearchOptions{
			TimeRemainingMs: timeRemainingMs,
			IncrementMs:     int64(incrementSeconds) * 1000,
			MoveNumber:      moveNumber,
			ThreadCount:     profile.Goroutines,
			PonderEnabled:   profile.Ponder,
			ParallelEnabled: profile.Goroutines > 1,
			TimeFraction:    profile.TimeFraction,
			UseVCF:          profile.UseVCF,
		}
	} else {
		opts = engine.SearchOptions{
			TimeRemainingMs: timeRemainingMs,
			IncrementMs:     int64(incrementSeconds) * 1000,
			MoveNumber:      moveNumber,
			PonderEnabled:   true,
			ParallelEnabled: true,
			TimeFraction:    1.0,
			UseVCF:          true,
		}
	}

	x, y := ai.GetBestMove(board, player, opts, r.Context())

	resp, err := session.ApplyMove(x, y)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": resp})
}

func (h *Handler) UndoMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, ok := h.store.Get(id)
	if !ok {
		writeError(w, domain.ErrGameNotFound)
		return
	}

	resp, err := session.UndoLastMove()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": resp})
}

func (h *Handler) DeleteGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := h.store.Get(id); !ok {
		writeError(w, domain.ErrGameNotFound)
		return
	}
	h.store.Delete(id)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
