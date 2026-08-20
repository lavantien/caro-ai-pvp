package api

import (
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"caro-ai-pvp/internal/persistence"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Handler struct {
	store   *InMemoryStore
	matches *persistence.MatchStore
	logger  interface {
		Info(msg string, args ...any)
	}
}

func NewHandler(store *InMemoryStore, matches *persistence.MatchStore, logger interface{ Info(string, ...any) }) *Handler {
	return &Handler{store: store, matches: matches, logger: logger}
}

func newGameID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Handler) CreateGame(w http.ResponseWriter, r *http.Request) {
	// Evict finished and idle sessions first so stale games never block new
	// ones between the periodic sweeps.
	h.store.CleanupCompleted()
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
	case "3+0":
		timeControl, initialTimeMs, incrementSeconds = "3+0", 180000, 0
	case "10+0":
		timeControl, initialTimeMs, incrementSeconds = "10+0", 600000, 0
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
	if req.RandomOpening && gameMode == domain.GameModeAivAI {
		session.applyRandomOpening(req.Seed)
	}
	h.store.Set(gameID, session)

	if h.matches != nil {
		redType, blueType := "human", "human"
		if gameMode == domain.GameModeAivAI {
			redType, blueType = "bot", "bot"
		} else if gameMode == domain.GameModePvAI {
			if redDiff != nil {
				redType = "bot"
			} else {
				blueType = "bot"
			}
		}
		h.matches.CreateGame(persistence.GameRecord{
			ID: gameID, GameMode: gameMode.String(), TimeControl: timeControl,
			RedType: redType, BlueType: blueType,
			RedDifficulty: redDiff, BlueDifficulty: blueDiff,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gameId": gameID,
		"state":  session.GetResponse(),
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

	resp, err := session.ApplyHumanMove(req.X, req.Y)
	if err != nil {
		writeError(w, err)
		return
	}

	if h.matches != nil {
		h.logHumanMove(id, req.X, req.Y, resp)
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
			MaxDepth:        profile.MaxDepth,
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

	start := time.Now()
	x, y, stats := ai.GetBestMove(board, player, opts, r.Context())
	thinkTime := time.Since(start).Milliseconds()

	resp, err := session.ApplyMove(x, y)
	if err != nil {
		writeError(w, err)
		return
	}

	if h.matches != nil {
		h.logAIMove(id, x, y, resp, difficulty, stats, thinkTime)
	}

	moveDetail := h.buildMoveDetail(resp, player.String(), x, y, stats, thinkTime)
	if h.logger != nil {
		h.logger.Info("move-statline", "gameId", id, "line", moveDetail.Statline)
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": resp, "lastMove": moveDetail})
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
	session, ok := h.store.Get(id)
	if !ok {
		writeError(w, domain.ErrGameNotFound)
		return
	}
	if h.matches != nil {
		resp := session.GetResponse()
		winner := resp.Winner
		if winner == "" || winner == "none" {
			winner = "abandoned"
		}
		h.matches.CompleteGame(id, winner, resp.MoveNumber)
	}
	h.store.Delete(id)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *Handler) logHumanMove(gameID string, x, y int, resp GameResponse) {
	player := resp.CurrentPlayer
	moveNum := resp.MoveNumber
	if moveNum > 0 {
		moveNum--
		player = opponentOf(player)
	}
	h.matches.RecordMove(persistence.MoveRecord{
		GameID:     gameID,
		MoveNumber: moveNum,
		Player:     player,
		PosX:       x,
		PosY:       y,
		IsBot:      false,
	})
	if resp.IsGameOver {
		h.matches.CompleteGame(gameID, resp.Winner, resp.MoveNumber)
	}
}

func (h *Handler) logAIMove(gameID string, x, y int, resp GameResponse, difficulty *int, stats engine.SearchStats, thinkTimeMs int64) {
	player := resp.CurrentPlayer
	moveNum := resp.MoveNumber
	if moveNum > 0 {
		moveNum--
		player = opponentOf(player)
	}
	remainingMs := int64(resp.RedTimeRemaining * 1000)
	if player == "blue" {
		remainingMs = int64(resp.BlueTimeRemaining * 1000)
	}
	depth := stats.DepthAchieved
	nodes := stats.NodesSearched
	nps := stats.NodesPerSecond
	hitRate := stats.TableHitRate
	score := stats.SearchScore
	threads := stats.ThreadCount
	allocMs := stats.AllocatedTimeMs
	mt := "exact"

	h.matches.RecordMove(persistence.MoveRecord{
		GameID:          gameID,
		MoveNumber:      moveNum,
		Player:          player,
		PosX:            x,
		PosY:            y,
		IsBot:           true,
		Difficulty:      difficulty,
		ThinkTimeMs:     &thinkTimeMs,
		RemainingTimeMs: &remainingMs,
		SearchDepth:     &depth,
		NodesSearched:   &nodes,
		NPS:             &nps,
		TTHitRate:       &hitRate,
		SearchScore:     &score,
		ThreadsUsed:     &threads,
		AllocatedTimeMs: &allocMs,
		MoveType:        &mt,
	})
	if resp.IsGameOver {
		h.matches.CompleteGame(gameID, resp.Winner, resp.MoveNumber)
	}
}

func formatStatlineNodes(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatStatlineNPS(nps float64) string {
	switch {
	case nps >= 1_000_000:
		return fmt.Sprintf("%.0fM", nps/1_000_000)
	case nps >= 1_000:
		return fmt.Sprintf("%.0fK", nps/1_000)
	default:
		return fmt.Sprintf("%.0f", nps)
	}
}

func (h *Handler) buildMoveDetail(resp GameResponse, player string, x, y int, stats engine.SearchStats, thinkTimeMs int64) MoveDetailResponse {
	moveNum := resp.MoveNumber - 1
	pos := fmt.Sprintf("%c%d", rune('a'+x), y+1)
	remainingMs := int64(resp.RedTimeRemaining * 1000)
	if player == "blue" {
		remainingMs = int64(resp.BlueTimeRemaining * 1000)
	}

	mt := "exact"
	if stats.MoveType != "" {
		mt = stats.MoveType
	}

	vcfTag := ""
	if mt == "vcf" {
		vcfTag = " [VCF]"
	} else if mt == "vcf-block" {
		vcfTag = " [VCF-BLOCK]"
	}

	statline := fmt.Sprintf("M%2d %-4s %s  d=%-2d n=%-7s nps=%-5s tt=%3d%% s=%+d thr=%d t=%.1fs alloc=%.1fs%s",
		moveNum, player, pos,
		stats.DepthAchieved,
		formatStatlineNodes(stats.NodesSearched),
		formatStatlineNPS(stats.NodesPerSecond),
		int(stats.TableHitRate*100),
		stats.SearchScore,
		stats.ThreadCount,
		float64(thinkTimeMs)/1000,
		float64(stats.AllocatedTimeMs)/1000,
		vcfTag,
	)

	return MoveDetailResponse{
		MoveNumber:      moveNum,
		Player:          player,
		Pos:             pos,
		Statline:        statline,
		ThinkTimeMs:     thinkTimeMs,
		RemainingTimeMs: remainingMs,
		EngineStats: EngineStatsResponse{
			Depth:           stats.DepthAchieved,
			Nodes:           stats.NodesSearched,
			NPS:             stats.NodesPerSecond,
			TTHitRate:       stats.TableHitRate,
			Score:           stats.SearchScore,
			Threads:         stats.ThreadCount,
			AllocatedTimeMs: stats.AllocatedTimeMs,
			MoveType:        mt,
		},
	}
}

func opponentOf(currentPlayer string) string {
	if currentPlayer == "red" {
		return "blue"
	}
	return "red"
}
