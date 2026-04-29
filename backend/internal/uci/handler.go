package uci

import (
	"bufio"
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

type UCIHandler struct {
	ai     *engine.MinimaxAI
	board  domain.Board
	player domain.Player
	logger *slog.Logger
	writer io.Writer
}

func NewUCIHandler(logger *slog.Logger, writer io.Writer) *UCIHandler {
	return &UCIHandler{
		ai:     engine.NewMinimaxAI(logger, 4),
		board:  domain.NewBoard(),
		player: domain.PlayerRed,
		logger: logger,
		writer: writer,
	}
}

func (h *UCIHandler) Board() domain.Board {
	return h.board
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
		h.respond("option name Threads type spin default 4 min 1 max 64")
		h.respond("option name Hash type spin default 64 min 32 max 4096")
		h.respond("option name Ponder type check default false")
		h.respond("option name Skill Level type spin default 5 min 1 max 5")
		h.respond("uciok")

	case "isready":
		h.respond("readyok")

	case "ucinewgame":
		h.board = domain.NewBoard()
		h.player = domain.PlayerRed
		h.ai = engine.NewMinimaxAI(h.logger, 4)

	case "position":
		h.handlePosition(fields[1:])

	case "go":
		h.handleGo(fields[1:])

	case "stop":

	case "quit":
		h.ai.Dispose()

	case "setoption":
	}
}

func (h *UCIHandler) handlePosition(args []string) {
	if len(args) == 0 {
		return
	}

	if args[0] == "startpos" {
		h.board = domain.NewBoard()
		h.player = domain.PlayerRed
		if len(args) > 2 && args[1] == "moves" {
			for _, moveStr := range args[2:] {
				x, y, ok := ParseMove(moveStr)
				if !ok {
					continue
				}
				h.board = h.board.PlaceStone(x, y, h.player)
				h.player = h.player.Opponent()
			}
		}
	}
}

func (h *UCIHandler) handleGo(args []string) {
	opts := engine.SearchOptions{
		TimeFraction:    1.0,
		UseVCF:          true,
		ParallelEnabled: true,
		ThreadCount:     4,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "movetime":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &opts.TimeRemainingMs)
				i++
			}
		case "wtime", "btime":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &opts.TimeRemainingMs)
				i++
			}
		}
	}

	x, y, stats := h.ai.GetBestMove(h.board, h.player, opts, context.Background())
	h.respond(fmt.Sprintf("info depth %d nodes %d nps %.0f score cp %d tt-hitrate %.2f threads %d",
		stats.DepthAchieved, stats.NodesSearched, stats.NodesPerSecond, stats.SearchScore, stats.TableHitRate, stats.ThreadCount))
	h.respond(fmt.Sprintf("bestmove %s", MoveToString(x, y)))
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
			return
		}
	}
}
