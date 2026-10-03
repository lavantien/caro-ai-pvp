package tourney

// The replay validator's own arms, driven straight at replayGame: the draw
// verdict's full-board demand (a clean fill, a short board, and a full board
// whose final stone completes a five), the illegal-move refusal, the empty
// board refusal, and the won-by tag in both seat colors.

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// cellName renders one board coordinate the way rules.ParseCell reads it.
func cellName(row, col int) string {
	return string(rune('A'+col)) + strconv.Itoa(row+1)
}

// drawSplit colors the empty board by (row+2*col) mod 4, red holding
// residues 0 and 1 and blue 2 and 3: the same fill the server bot tests
// ride, where every straight window of 5 cells holds both colors along all
// four directions, so no five can exist at any point of the fill.
func drawSplit() (red, blue []string) {
	for row := range config.BoardSize {
		for col := range config.BoardSize {
			if (row+2*col)%4 < 2 {
				red = append(red, cellName(row, col))
			} else {
				blue = append(blue, cellName(row, col))
			}
		}
	}
	return red, blue
}

// drawMoves interleaves the split with red first. Red's second stone swaps
// with its last to clear the opening distance, mirroring the server
// fixture; the fill's final move is blue's last cell.
func drawMoves() []string {
	red, blue := drawSplit()
	red[1], red[len(red)-1] = red[len(red)-1], red[1]
	out := make([]string, 0, len(red)+len(blue))
	for i := range red {
		out = append(out, red[i], blue[i])
	}
	return out
}

// drawMovesWithFinalFive reshapes the fill so the final stone of the game
// completes a blue five that the win law accepts: the line sits at row 7
// columns 0..4, against the board edge, because the both-ends-blocked law
// rejects a five the opponent caps on both sides and every interior cell of
// a full board is a stone. Two pattern-red cells of the line flip to blue,
// two blue cells of column 1 flip back to red for parity, and the fifth
// stone of the line lands as the very last move of the game. Every other
// window still holds both colors, so the five exists only once the final
// stone closes it.
func drawMovesWithFinalFive() []string {
	red, blue := drawSplit()
	take := func(list []string, name string) []string {
		for i, c := range list {
			if c == name {
				return append(list[:i], list[i+1:]...)
			}
		}
		panic("tourney test: " + name + " missing from the split")
	}
	for _, name := range []string{cellName(7, 1), cellName(7, 3)} {
		red = take(red, name)
		blue = append(blue, name)
	}
	for _, name := range []string{cellName(0, 1), cellName(1, 1)} {
		blue = take(blue, name)
		red = append(red, name)
	}
	// The parity flips land at the tail of red, so red's opening swap
	// targets the far corner by name instead of the last slice cell.
	for i, c := range red {
		if c == cellName(15, 15) {
			red[1], red[i] = red[i], red[1]
			break
		}
	}
	// The line's inner four stones play as blue's opening moves, the edge
	// stone closes the game.
	rest := blue
	for _, name := range []string{cellName(7, 0), cellName(7, 1), cellName(7, 2), cellName(7, 3), cellName(7, 4)} {
		rest = take(rest, name)
	}
	blue = append(append([]string{cellName(7, 1), cellName(7, 2), cellName(7, 3), cellName(7, 4)}, rest...), cellName(7, 0))
	out := make([]string, 0, len(red)+len(blue))
	for i := range red {
		out = append(out, red[i], blue[i])
	}
	return out
}

func parseMoves(t *testing.T, names []string) []rules.Move {
	t.Helper()
	out := make([]rules.Move, len(names))
	for i, name := range names {
		cell, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		out[i] = rules.Move(cell)
	}
	return out
}

func TestReplayGameDrawVerdicts(t *testing.T) {
	clean := parseMoves(t, drawMoves())
	if len(clean) != config.BoardCells {
		t.Fatalf("clean draw fill = %d moves, want %d", len(clean), config.BoardCells)
	}
	if wonBy, err := replayGame(clean, server.Draw); err != nil || wonBy != nil {
		t.Errorf("clean full-board draw = (%v, %v), want (nil, nil)", wonBy, err)
	}

	// A draw verdict over a short board refuses before the five check.
	short := parseMoves(t, sweepRedMoves)
	_, err := replayGame(short, server.Draw)
	if err == nil || !strings.Contains(err.Error(), "cells") {
		t.Errorf("short-board draw = %v, want the cell-count refusal", err)
	}

	// A full board whose final stone completes a five refuses the draw
	// verdict; the premise is pinned first through the same replay under
	// the blue-win verdict, proving the last stone does close the line.
	five := parseMoves(t, drawMovesWithFinalFive())
	if len(five) != config.BoardCells {
		t.Fatalf("final-five draw fill = %d moves, want %d", len(five), config.BoardCells)
	}
	if tag, ferr := replayGame(five, server.BlueWins); ferr != nil || tag == nil {
		t.Fatalf("final-five fill under the blue verdict = (%v, %v), want the closing five", tag, ferr)
	}
	_, err = replayGame(five, server.Draw)
	if err == nil || !strings.Contains(err.Error(), "completes a five") {
		t.Errorf("five-completing draw = %v, want the final-stone refusal", err)
	}
}

func TestReplayGameRefusals(t *testing.T) {
	// An empty move list carries no terminal position at all.
	if _, err := replayGame(nil, server.RedWins); err == nil || !strings.Contains(err.Error(), "empty board") {
		t.Errorf("empty board = %v, want the empty-board refusal", err)
	}
	// A repeated cell is illegal on the replayed board.
	repeat := parseMoves(t, append(append([]string(nil), sweepRedMoves...), "D4"))
	if _, err := replayGame(repeat, server.RedWins); err == nil || !strings.Contains(err.Error(), "illegal") {
		t.Errorf("repeated cell = %v, want the illegal-move refusal", err)
	}
}

func TestReplayGameTagsBothSeats(t *testing.T) {
	red := parseMoves(t, sweepRedMoves)
	tag, err := replayGame(red, server.RedWins)
	if err != nil || tag == nil || *tag != server.WonByOpenFour {
		t.Errorf("red sweep = (%v, %v), want the open-four tag", tag, err)
	}
	blue := parseMoves(t, sweepBlueMoves)
	tag, err = replayGame(blue, server.BlueWins)
	if err != nil || tag == nil || *tag != server.WonByOpenFour {
		t.Errorf("blue sweep = (%v, %v), want the open-four tag", tag, err)
	}
	// A win verdict the final stone does not earn refuses the replay.
	scatter := parseMoves(t, sweepRedMoves[:len(sweepRedMoves)-1])
	if _, err := replayGame(scatter, server.RedWins); err == nil || !strings.Contains(err.Error(), "completes no five") {
		t.Errorf("unearned win = %v, want the no-five refusal", err)
	}
}
