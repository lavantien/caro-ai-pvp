package main

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func replayIndependently(t *testing.T, variant string, names []string) (rules.Color, string) {
	t.Helper()
	g, err := geom.ByName(variant)
	if err != nil {
		t.Fatalf("%s geometry: %v", variant, err)
	}
	b := rules.NewBoardFor(g)
	for i, name := range names {
		parsed, err := g.ParseCell(name)
		if err != nil {
			t.Fatalf("%s name %q: %v", variant, name, err)
		}
		c := rules.Cell(parsed)
		if !b.IsLegal(c) {
			t.Fatalf("%s move %d %q illegal", variant, i, name)
		}
		side := b.Side
		b.Make(c)
		last := i == len(names)-1
		if won := b.FastLastMoveWin(side, c); won != last {
			t.Fatalf("%s move %d %q win = %v, want exactly the final move to win", variant, i, name, won)
		}
		if !last {
			continue
		}
		b.Unmake()
		tag := server.WonByTag(b, side)
		b.Make(c)
		return side, tag
	}
	t.Fatalf("%s replay never ended", variant)
	return rules.Empty, ""
}

func TestSeededWinsEndExactlyOnTheirFinalMove(t *testing.T) {
	for variant, names := range map[string][]string{
		config.DefaultVariant:   seedSquareWinNames,
		config.Variants[1].Name: seedHexWinNames,
	} {
		winner, tag := replayIndependently(t, variant, names)
		if winner != rules.Red {
			t.Errorf("%s seed winner = %v, want red", variant, winner)
		}
		seedWinner, seedTag := replayOutcome(variant, names)
		if seedWinner != winner || seedTag != tag {
			t.Errorf("%s seed outcome = (%v, %q), independent replay = (%v, %q)", variant, seedWinner, seedTag, winner, tag)
		}
	}
}
