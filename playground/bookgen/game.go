package main

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func seatTier(name string) (config.Tier, error) {
	for i := range config.Tiers {
		if config.Tiers[i].Name == name {
			t := config.Tiers[i]
			t.Book = false
			return t, nil
		}
	}
	return config.Tier{}, fmt.Errorf("bookgen: unknown tier %q", name)
}

func openingMove(gameIdx int) rules.Move {
	return rules.Move(config.BookOpeningCells[gameIdx%len(config.BookOpeningCells)])
}

func playCorpusGame(tier config.Tier, tcIdx, gameIdx, redSeat int, mlines io.Writer) gameResult {
	engines := [2]server.Searcher{server.NewBotSearcher(tier), server.NewBotSearcher(tier)}
	defer engines[0].Close()
	defer engines[1].Close()
	return playGame(engines, tcIdx, gameIdx, redSeat, mlines)
}

func playGame(engines [2]server.Searcher, tcIdx, gameIdx, redSeat int, mlines io.Writer) gameResult {
	clocks := [2]*clock.GameClock{clock.NewGameClock(tcIdx), clock.NewGameClock(tcIdx)}
	var legalBuf [config.BoardCells]rules.Move
	b := rules.NewBoard()
	res := gameResult{}
	turnStart := time.Now()
	for {
		side := b.Side
		seat := redSeat
		if side == rules.Blue {
			seat = 1 - redSeat
		}
		var mv rules.Move
		legal := true
		if b.MoveCount == 0 {
			mv = openingMove(gameIdx)
			name, nerr := rules.CellName(rules.Cell(mv))
			if nerr != nil {
				name = strconv.Itoa(int(mv))
			}
			fmt.Fprintf(mlines, "g%d opening %s\n", gameIdx+1, name)
			if !b.IsLegal(rules.Cell(mv)) {
				n := b.LegalMoves(legalBuf[:])
				fmt.Fprintf(mlines, "g%d illegal opening %d, fell back (legal count %d)\n", gameIdx+1, mv, n)
				mv = legalBuf[0]
				legal = false
			}
		} else {
			budget := clocks[seat].Budget()
			var st engine.SearchStats
			var tag string
			mv, st, tag = engines[seat].Search(b, engine.NewFixedBudget(budget))
			fmt.Fprintf(mlines, "g%d %s\n", gameIdx+1, server.MLine(b.MoveCount+1, side, mv, &st, tag))
			if !b.IsLegal(rules.Cell(mv)) {
				n := b.LegalMoves(legalBuf[:])
				fmt.Fprintf(mlines, "g%d illegal %d from seat %d, fell back (legal count %d)\n", gameIdx+1, mv, seat, n)
				mv = legalBuf[0]
				legal = false
			}
		}
		if !legal {
			res.illegal++
		} else if b.MoveCount < config.BookMaxPly {
			res.records = append(res.records, posRec{key: keyOf(b), move: mv})
		}
		cell := rules.Cell(mv)
		b.Make(cell)
		clocks[seat].Commit(time.Since(turnStart))
		turnStart = time.Now()
		res.moves++
		if b.FastLastMoveWin(side, cell) {
			res.winner = "red"
			if side == rules.Blue {
				res.winner = "blue"
			}
			return res
		}
		if b.IsFull() {
			res.winner = "draw"
			return res
		}
	}
}
