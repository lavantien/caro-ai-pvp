package main

import (
	"bytes"
	"fmt"
	"strings"
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

func TestResultFor(t *testing.T) {
	cases := []struct {
		ref, winner string
		want        Result
	}{
		{"easy", "", Draw},
		{"easy", "easy", Win},
		{"easy", "hard", Loss},
		{"easy-vcf", "easy", Loss},
		{"easy-vcf", "easy-vcf", Win},
	}
	for _, c := range cases {
		if got := resultFor(c.ref, c.winner); got != c.want {
			t.Errorf("resultFor(%q, %q) = %d, want %d", c.ref, c.winner, got, c.want)
		}
	}
}

func TestEmitDuelPairAndUnpaired(t *testing.T) {
	var buf bytes.Buffer
	emitDuelPair(&buf, "easy", "medium", 3, PairWinDraw)
	emitDuelUnpaired(&buf, "easy", "medium", 7, Loss)
	want := "duel easy vs medium pair 3: easy 1.5-0.5\nduel easy vs medium unpaired game 7: easy loss\n"
	if buf.String() != want {
		t.Errorf("duel emission = %q, want %q", buf.String(), want)
	}
}

func TestPrintResultsKeepsLegacyBlockAndAppendsPtnml(t *testing.T) {
	results := map[string]*tally{
		"easy":   {name: "easy", wins: 9, losses: 3, games: 12, redWins: 5, blueWins: 4, ponderArms: 2, ponderAdopts: 1},
		"medium": {name: "medium", wins: 3, losses: 9, games: 12, redWins: 2, blueWins: 1},
	}
	easyMedium := &duel{a: "easy", b: "medium"}
	easyMedium.pt.AddGame(Win)
	easyMedium.pt.AddGame(Win)
	easyMedium.pt.AddGame(Draw)
	easyMedium.pt.Finish()
	var buf bytes.Buffer
	printResults(&buf, []string{"easy", "medium"}, results, []*duel{easyMedium})
	pad := func(label string) string {
		return label + strings.Repeat(" ", 22-len(label)) + " "
	}
	want := strings.Join([]string{
		"results (wins-losses-draws over games, red/blue win split, ponder adopts/arms)",
		pad("easy") + "9-3-0 over 12, red wins 5 blue wins 4, ponder 1/2",
		pad("medium") + "3-9-0 over 12, red wins 2 blue wins 1, ponder 0/0",
		"ptnml (duel pairs merged from the first seat's side: 2-0 1.5-0.5 1-1 0.5-1.5 0-0)",
		pad("easy vs medium") + "1-0-0-0-0 over 1 pairs, 1 unpaired",
		"",
	}, "\n")
	if buf.String() != want {
		t.Errorf("printResults output:\n got %q\nwant %q", buf.String(), want)
	}
}

func TestPrintResultsWithoutDuelsSkipsPtnmlBlock(t *testing.T) {
	results := map[string]*tally{"easy": {name: "easy"}}
	var buf bytes.Buffer
	printResults(&buf, []string{"easy"}, results, nil)
	want := "results (wins-losses-draws over games, red/blue win split, ponder adopts/arms)\neasy" +
		strings.Repeat(" ", 19) + "0-0-0 over 0, red wins 0 blue wins 0, ponder 0/0\n"
	if buf.String() != want {
		t.Errorf("printResults without duels = %q, want %q", buf.String(), want)
	}
}
