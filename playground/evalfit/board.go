package main

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type sample struct {
	Series string
	F      [5]int32
	Y      float64
}

type seat struct {
	Name string
	Tier string
}

type gameMove struct {
	Color rules.Color
	Cell  rules.Cell
}

type gameOutcome uint8

const (
	outcomeDraw gameOutcome = iota
	outcomeRedWin
	outcomeBlueWin
)

func (o gameOutcome) winner() (rules.Color, bool) {
	switch o {
	case outcomeRedWin:
		return rules.Red, true
	case outcomeBlueWin:
		return rules.Blue, true
	}
	return 0, false
}

type parsedGame struct {
	Red     seat
	Blue    seat
	Outcome gameOutcome
	Moves   []gameMove
}

func featureVector(b *rules.Board, mover rules.Color) [5]int32 {
	var red, blue [config.PatternClassCount]int32
	for d := range config.PatternDirections {
		for cell := range config.BoardCells {
			c := rules.Cell(cell)
			red[pattern.Lookup(0, pattern.Index(b, c, d, rules.Red)).Class]++
			blue[pattern.Lookup(0, pattern.Index(b, c, d, rules.Blue)).Class]++
		}
	}
	own, opp := &red, &blue
	if mover == rules.Blue {
		own, opp = &blue, &red
	}
	var f [5]int32
	for c := 1; c < config.PatternClassCount; c++ {
		f[c-1] = own[c] - opp[c]
	}
	return f
}

func reconstructGame(g parsedGame, series string) ([]sample, error) {
	b := rules.NewBoard()
	samples := make([]sample, 0, len(g.Moves))
	want := rules.Red
	for i, m := range g.Moves {
		if m.Color != want {
			return nil, fmt.Errorf("game move %d color %v breaks the red/blue alternation", i+1, m.Color)
		}
		if b.Occupied(m.Cell) {
			return nil, fmt.Errorf("game move %d plays occupied cell %d", i+1, m.Cell)
		}
		y := 0.5
		if winner, decisive := g.Outcome.winner(); decisive {
			if winner == m.Color {
				y = 1.0
			} else {
				y = 0.0
			}
		}
		samples = append(samples, sample{Series: series, F: featureVector(b, m.Color), Y: y})
		b.Make(m.Cell)
		want = want.Opponent()
	}
	if winner, decisive := g.Outcome.winner(); decisive {
		if !b.Wins(winner) {
			color := "red"
			if winner == rules.Blue {
				color = "blue"
			}
			return nil, fmt.Errorf("recorded %s winner has no winning line on the final board", color)
		}
		return samples, nil
	}
	if !b.IsFull() {
		return nil, fmt.Errorf("recorded draw but the final board is not full (%d of 256 cells)", b.MoveCount)
	}
	return samples, nil
}
