package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func TestCrossSeatsHalveShapeOnly(t *testing.T) {
	for _, name := range []string{"master", "grandmaster"} {
		base := seats[name]
		crossName := name + "-cross"
		cross, ok := seats[crossName]
		if !ok {
			t.Fatalf("seat %q missing from the roster", crossName)
		}
		if cross.Cores != base.Cores/2 {
			t.Errorf("%s cores = %d, want half of %d", crossName, cross.Cores, base.Cores)
		}
		if cross.TTBytes != base.TTBytes/2 {
			t.Errorf("%s ttbytes = %d, want half of %d", crossName, cross.TTBytes, base.TTBytes)
		}
		if cross.Book != base.Book || cross.BookMaxPly != base.BookMaxPly || cross.VCF != base.VCF ||
			cross.VCT != base.VCT || cross.PonderThreads != base.PonderThreads || cross.Name != crossName {
			t.Errorf("%s perturbs beyond the halved shape: %+v over %+v", crossName, cross, base)
		}
		if got := seatLabels("master-cross,grandmaster-cross"); len(got) != 2 {
			t.Errorf("seatLabels of the cross pair = %v, want both", got)
		}
	}
}

func probeDrive(t *testing.T, red, blue string, moveBudget time.Duration, maxMoves int) []string {
	t.Helper()
	redSeat := &seatState{label: red, tier: seats[red], eng: server.NewBotSearcher(seats[red]), tally: &tally{name: red}}
	blueSeat := &seatState{label: blue, tier: seats[blue], eng: server.NewBotSearcher(seats[blue]), tally: &tally{name: blue}}
	defer redSeat.eng.Close()
	defer blueSeat.eng.Close()
	st := map[rules.Color]*seatState{rules.Red: redSeat, rules.Blue: blueSeat}
	var lines []string
	b := rules.NewBoard()
	var legalBuf [config.BoardCells]rules.Move
	for move := 1; move <= maxMoves; move++ {
		side := b.Side
		s := st[side]
		mv, stats, tag := s.eng.Search(b, engine.NewFixedBudget(moveBudget))
		if !b.IsLegal(rules.Cell(mv)) {
			n := b.LegalMoves(legalBuf[:])
			t.Fatalf("%s returned illegal move on probe move %d, legal count %d", s.label, move, n)
		}
		rendered := server.MLine(move, side, mv, &stats, tag)
		if rendered == "" {
			t.Fatalf("%s rendered an empty M-line on probe move %d", s.label, move)
		}
		lines = append(lines, rendered)
		b.Make(rules.Cell(mv))
		if b.FastLastMoveWin(side, rules.Cell(mv)) || b.IsFull() {
			return lines
		}
	}
	return lines
}

func TestCrossAndNobookSeatsRenderMLines(t *testing.T) {
	for _, pair := range [][2]string{
		{"master-cross", "master-nobook"},
		{"grandmaster-cross", "grandmaster-nobook"},
	} {
		lines := probeDrive(t, pair[0], pair[1], 100*time.Millisecond, 24)
		if len(lines) < 8 {
			t.Fatalf("%s vs %s probe rendered only %d M-lines", pair[0], pair[1], len(lines))
		}
	}
}

func TestNobookProbeNeverCarriesBookTag(t *testing.T) {
	lines := probeDrive(t, "master-cross", "master-nobook", 100*time.Millisecond, 24)
	for i, line := range lines {
		if (i+1)%2 == 0 && strings.Contains(line, config.BotLogTagBook) {
			t.Fatalf("blue master-nobook probe M-line carries the book tag: %q", line)
		}
	}
}
