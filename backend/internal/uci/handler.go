package uci

import (
	"bufio"
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

type UCIHandler struct {
	ai     *engine.MinimaxAI
	board  domain.Board
	player domain.Player
	logger *slog.Logger
	writer io.Writer

	mu         sync.Mutex
	cancel     context.CancelFunc
	searchDone chan struct{}
	threads    int
	hashMB     int
	skillLevel int
}

func NewUCIHandler(logger *slog.Logger, writer io.Writer) *UCIHandler {
	return &UCIHandler{
		ai:         engine.NewMinimaxAI(logger, 4, 256),
		board:      domain.NewBoard(),
		player:     domain.PlayerRed,
		logger:     logger,
		writer:     writer,
		threads:    4,
		hashMB:     256,
		skillLevel: 5,
	}
}

func (h *UCIHandler) Board() domain.Board {
	return h.board
}

func (h *UCIHandler) SkillLevel() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.skillLevel
}

func (h *UCIHandler) HandleCommand(cmd string) {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return
	}

	switch fields[0] {
	case "uci":
		h.respond("id name Caro AI")
		h.respond("id author Caro AI Project")
		h.respond(fmt.Sprintf("option name Threads type spin default %d min 1 max 64", h.currentThreads()))
		h.respond("option name Hash type spin default 256 min 32 max 4096")
		h.respond("option name Skill Level type spin default 5 min 1 max 5")
		h.respond("uciok")

	case "isready":
		h.respond("readyok")

	case "ucinewgame":
		h.stopSearchAndWait()
		h.board = domain.NewBoard()
		h.player = domain.PlayerRed
		h.rebuildAI()

	case "position":
		h.stopSearchAndWait()
		h.handlePosition(fields[1:])

	case "go":
		h.handleGo(fields[1:])

	case "stop":
		h.stopSearch()

	case "quit":
		h.stopSearchAndWait()
		h.ai.Dispose()

	case "setoption":
		h.handleSetOption(fields[1:])
	}
}

func (h *UCIHandler) handlePosition(args []string) {
	if len(args) == 0 {
		return
	}

	if args[0] != "startpos" {
		h.respond("info string error unsupported position argument")
		return
	}
	h.board = domain.NewBoard()
	h.player = domain.PlayerRed
	if len(args) > 2 && args[1] == "moves" {
		moves := make([]domain.Position, 0, len(args)-2)
		for _, moveStr := range args[2:] {
			x, y, ok := ParseMove(moveStr)
			if !ok {
				// Reject the whole command: partially applying it would
				// desync the engine from the caller's board.
				h.respond(fmt.Sprintf("info string error invalid move %q; position not changed", moveStr))
				return
			}
			moves = append(moves, domain.Position{X: x, Y: y})
		}
		for _, m := range moves {
			h.board = h.board.PlaceStone(m.X, m.Y, h.player)
			h.player = h.player.Opponent()
		}
	}
}

func (h *UCIHandler) handleGo(args []string) {
	h.stopSearchAndWait()

	board := h.board
	player := h.player
	opts := parseGoOptions(args, player, h.skillSearchOptions())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	h.mu.Lock()
	h.cancel = cancel
	h.searchDone = done
	h.mu.Unlock()

	go func() {
		defer close(done)
		x, y, stats := h.ai.GetBestMove(board, player, opts, ctx)
		h.respond(fmt.Sprintf("info depth %d nodes %d nps %.0f score cp %d tt-hitrate %.2f threads %d",
			stats.DepthAchieved, stats.NodesSearched, stats.NodesPerSecond, stats.SearchScore, stats.TableHitRate, stats.ThreadCount))
		h.respond(fmt.Sprintf("bestmove %s", MoveToString(x, y)))
	}()
}

func (h *UCIHandler) handleSetOption(args []string) {
	// Expected shape: "name <Name...> value <Value>"; the name may span
	// several tokens (e.g. "Skill Level").
	nameStart, valueIdx := -1, -1
	for i, a := range args {
		if a == "name" && i+1 < len(args) {
			nameStart = i + 1
		}
		if a == "value" {
			valueIdx = i
			break
		}
	}
	if nameStart < 0 || valueIdx < 0 || valueIdx <= nameStart || valueIdx+1 >= len(args) {
		return
	}
	name := strings.Join(args[nameStart:valueIdx], " ")
	value := args[valueIdx+1]

	n, err := strconv.Atoi(value)
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	switch name {
	case "Threads":
		if n >= 1 && n <= 64 {
			h.threads = n
			h.rebuildAI()
		}
	case "Hash":
		if n >= 32 && n <= 4096 {
			h.hashMB = n
			h.rebuildAI()
		}
	case "Skill Level":
		if n >= 1 && n <= 5 {
			h.skillLevel = n
		}
	}
}

// rebuildAI recreates the engine with the configured threads/hash. Callers
// must hold h.mu and must have stopped any running search.
func (h *UCIHandler) rebuildAI() {
	h.ai.Dispose()
	h.ai = engine.NewMinimaxAI(h.logger, h.threads, h.hashMB)
}

// skillSearchOptions maps the configured skill level onto the engine's
// strength profile, capped by the configured thread count.
func (h *UCIHandler) skillSearchOptions() engine.SearchOptions {
	h.mu.Lock()
	skill := h.skillLevel
	threads := h.threads
	h.mu.Unlock()

	if skill < 1 || skill > 5 {
		skill = 5
	}
	profile := engine.GetDifficultyProfile(skill)
	goroutines := min(threads, profile.Goroutines)
	return engine.SearchOptions{
		ThreadCount:     goroutines,
		ParallelEnabled: goroutines > 1,
		TimeFraction:    profile.TimeFraction,
		UseVCF:          profile.UseVCF,
		MaxDepth:        profile.MaxDepth,
	}
}

// parseGoOptions overlays go-command arguments onto base options for the
// given side. movetime maps to a fixed budget; wtime/winc or btime/binc
// follow the side to move; depth caps the search.
func parseGoOptions(args []string, player domain.Player, base engine.SearchOptions) engine.SearchOptions {
	for i := 0; i+1 < len(args); i++ {
		val, err := strconv.ParseInt(args[i+1], 10, 64)
		if err != nil {
			continue
		}
		switch args[i] {
		case "movetime":
			base.TimeRemainingMs = val
			base.IncrementMs = 0
		case "depth":
			if d := int(val); d > 0 {
				base.MaxDepth = d
			}
		case "wtime":
			if player == domain.PlayerRed {
				base.TimeRemainingMs = val
			}
		case "btime":
			if player == domain.PlayerBlue {
				base.TimeRemainingMs = val
			}
		case "winc":
			if player == domain.PlayerRed {
				base.IncrementMs = val
			}
		case "binc":
			if player == domain.PlayerBlue {
				base.IncrementMs = val
			}
		}
	}
	return base
}

func (h *UCIHandler) stopSearch() {
	h.mu.Lock()
	cancel := h.cancel
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// stopSearchAndWait cancels any running search and waits for its bestmove to
// be emitted, so state changes never race an active search.
func (h *UCIHandler) stopSearchAndWait() {
	h.stopSearch()
	h.mu.Lock()
	done := h.searchDone
	h.mu.Unlock()
	if done != nil {
		<-done
	}
	h.mu.Lock()
	h.cancel = nil
	h.searchDone = nil
	h.mu.Unlock()
}

func (h *UCIHandler) currentThreads() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.threads
}

func (h *UCIHandler) respond(msg string) {
	fmt.Fprintln(h.writer, msg)
}

func RunUCILoop(handler *UCIHandler, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		handler.HandleCommand(line)
		if line == "quit" {
			handler.stopSearchAndWait()
			return
		}
	}
	if err := scanner.Err(); err != nil && handler.logger != nil {
		handler.logger.Error("uci read loop", "err", err)
	}
}
