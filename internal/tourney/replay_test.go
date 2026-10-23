package tourney

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func cellName(row, col int) string {
	return string(rune('A'+col)) + strconv.Itoa(row+1)
}

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

func drawMoves() []string {
	red, blue := drawSplit()
	red[1], red[len(red)-1] = red[len(red)-1], red[1]
	out := make([]string, 0, len(red)+len(blue))
	for i := range red {
		out = append(out, red[i], blue[i])
	}
	return out
}

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
	for i, c := range red {
		if c == cellName(15, 15) {
			red[1], red[i] = red[i], red[1]
			break
		}
	}
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
	if wonBy, err := replayGame(clean, server.Draw, geom.Default()); err != nil || wonBy != nil {
		t.Errorf("clean full-board draw = (%v, %v), want (nil, nil)", wonBy, err)
	}
	short := parseMoves(t, sweepRedMoves)
	_, err := replayGame(short, server.Draw, geom.Default())
	if err == nil || !strings.Contains(err.Error(), "cells") {
		t.Errorf("short-board draw = %v, want the cell-count refusal", err)
	}
	five := parseMoves(t, drawMovesWithFinalFive())
	if len(five) != config.BoardCells {
		t.Fatalf("final-five draw fill = %d moves, want %d", len(five), config.BoardCells)
	}
	if tag, ferr := replayGame(five, server.BlueWins, geom.Default()); ferr != nil || tag == nil {
		t.Fatalf("final-five fill under the blue verdict = (%v, %v), want the closing five", tag, ferr)
	}
	_, err = replayGame(five, server.Draw, geom.Default())
	if err == nil || !strings.Contains(err.Error(), "completes a five") {
		t.Errorf("five-completing draw = %v, want the final-stone refusal", err)
	}
}

func TestReplayGameRefusals(t *testing.T) {
	if _, err := replayGame(nil, server.RedWins, geom.Default()); err == nil || !strings.Contains(err.Error(), "empty board") {
		t.Errorf("empty board = %v, want the empty-board refusal", err)
	}
	repeat := parseMoves(t, append(append([]string(nil), sweepRedMoves...), "D4"))
	if _, err := replayGame(repeat, server.RedWins, geom.Default()); err == nil || !strings.Contains(err.Error(), "illegal") {
		t.Errorf("repeated cell = %v, want the illegal-move refusal", err)
	}
}

func TestReplayGameTagsBothSeats(t *testing.T) {
	red := parseMoves(t, sweepRedMoves)
	tag, err := replayGame(red, server.RedWins, geom.Default())
	if err != nil || tag == nil || *tag != server.WonByOpenFour {
		t.Errorf("red sweep = (%v, %v), want the open-four tag", tag, err)
	}
	blue := parseMoves(t, sweepBlueMoves)
	tag, err = replayGame(blue, server.BlueWins, geom.Default())
	if err != nil || tag == nil || *tag != server.WonByOpenFour {
		t.Errorf("blue sweep = (%v, %v), want the open-four tag", tag, err)
	}
	scatter := parseMoves(t, sweepRedMoves[:len(sweepRedMoves)-1])
	if _, err := replayGame(scatter, server.RedWins, geom.Default()); err == nil || !strings.Contains(err.Error(), "completes no five") {
		t.Errorf("unearned win = %v, want the no-five refusal", err)
	}
}
