package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestPonderOutcome(t *testing.T) {
	cases := []struct {
		armed, hit, adopted bool
		want                string
	}{
		{false, false, false, ""},
		{false, true, true, ""},
		{true, false, false, "miss"},
		{true, false, true, "miss"},
		{true, true, false, "reject"},
		{true, true, true, "adopt"},
	}
	for _, c := range cases {
		if got := ponderOutcome(c.armed, c.hit, c.adopted); got != c.want {
			t.Errorf("ponderOutcome(%v, %v, %v) = %q, want %q", c.armed, c.hit, c.adopted, got, c.want)
		}
	}
}

func TestEmitMLineMatchesBotLogFormat(t *testing.T) {
	h8 := rules.Move(7*config.BoardStride + 7)
	st := engine.SearchStats{
		Depth: 9, Nodes: 7_280_000, Nps: 6_540_000, EBFMilli: 5800,
		TTHitPermille: 100, HashFullPermille: 20, FirstMoveFailHighPermille: 910,
		Score: 1200, Threads: 8, ElapsedNs: 1_110_000_000, AllocNs: 2_240_000_000,
	}
	st.PV[0] = h8
	st.PVLen = 1
	var buf bytes.Buffer
	emitMLine(&buf, 5, 9, rules.Red, h8, &st, config.BotLogTagPonder)
	want := fmt.Sprintf("g5 %s\n", fmt.Sprintf(config.BotLogFormat,
		9, "Red", "H8", 9, "7.28m", "6.54m", 5.8, 10, 2, 91, "+1200", 8,
		1.11, 2.24, config.BotLogTagPonder, "H8"))
	if buf.String() != want {
		t.Errorf("emitMLine output mismatch:\n got %q\nwant %q", buf.String(), want)
	}
}

func TestNoBookSeatsClearOnlyTheBookFlag(t *testing.T) {
	for _, name := range []string{"master", "grandmaster"} {
		base, ok := seats[name]
		if !ok {
			t.Fatalf("seat %q missing from the roster", name)
		}
		if !base.Book {
			t.Errorf("%s seat lost the book flag", name)
		}
		nobook := name + "-nobook"
		over, ok := seats[nobook]
		if !ok {
			t.Fatalf("override seat %q missing from the roster", nobook)
		}
		if over.Book {
			t.Errorf("%s override keeps the book flag, want the book cleared", nobook)
		}
		if over.BookMaxPly != base.BookMaxPly || over.Cores != base.Cores || over.TTBytes != base.TTBytes ||
			over.VCF != base.VCF || over.VCT != base.VCT || over.PonderThreads != base.PonderThreads {
			t.Errorf("%s override perturbs beyond the book flag: %+v over %+v", nobook, over, base)
		}
		if over.Name != nobook {
			t.Errorf("%s override name = %q, want the seat label", nobook, over.Name)
		}
	}
	if got := seatLabels("master-nobook,grandmaster-nobook"); len(got) != 2 || got[0] != "master-nobook" || got[1] != "grandmaster-nobook" {
		t.Errorf("seatLabels of the nobook pair = %v, want both override labels", got)
	}
}

func TestEmitPonderOutcome(t *testing.T) {
	var buf bytes.Buffer
	emitPonderOutcome(&buf, 5, 9, "master", "reject")
	if buf.String() != "g5 ponder master m9 reject\n" {
		t.Errorf("emitPonderOutcome = %q", buf.String())
	}
}
