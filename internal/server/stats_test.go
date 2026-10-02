package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func mustMove(t *testing.T, name string) rules.Move {
	t.Helper()
	cell, err := rules.ParseCell(name)
	if err != nil {
		t.Fatalf("parse cell %q: %v", name, err)
	}
	return rules.Move(cell)
}

func setPV(t *testing.T, st *engine.SearchStats, names ...string) {
	t.Helper()
	if len(names) > len(st.PV) {
		t.Fatalf("pv of %d moves overflows the %d-slot line", len(names), len(st.PV))
	}
	for i, name := range names {
		st.PV[i] = mustMove(t, name)
	}
	st.PVLen = len(names)
}

// TestMLineGoldensImplication15 reproduces the three Implication 1.5 example
// lines verbatim from hand-built SearchStats values: the tag arrives as a
// config.BotLogTag constant, the empty string leaves the healthy line
// without a tag section, and the M9 score sits on the mateWin lattice at
// EvalMateMax - 9*EvalMateScoreStep.
func TestMLineGoldensImplication15(t *testing.T) {
	cases := []struct {
		name string
		line string
		num  int
		side rules.Color
		move string
		tag  string
		st   engine.SearchStats
		pv   []string
	}{
		{
			name: "healthy",
			line: "M24, Red, J9, d=14, n=6.25m, nps=2.5m, ebf=2.1, tt=38%, hf=45%, fh1=93%, s=+150, thr=4, t=2.50, alloc=2.50, pv=J9 K10 K9 L9 M8 L8",
			num:  24, side: rules.Red, move: "J9", tag: "",
			st: engine.SearchStats{
				Depth: 14, Nodes: 6_250_000, Nps: 2_500_000, EBFMilli: 2100,
				TTHitPermille: 380, HashFullPermille: 450, FirstMoveFailHighPermille: 930,
				Score: 150, Threads: 4, ElapsedNs: 2_500_000_000, AllocNs: 2_500_000_000,
			},
			pv: []string{"J9", "K10", "K9", "L9", "M8", "L8"},
		},
		{
			name: "vct hit",
			line: "M31, Blue, G7, d=9, n=45k, nps=1.2m, ebf=1.4, tt=18%, hf=60%, fh1=88%, s=M9, thr=4, t=0.03, alloc=3.00, [VCT], pv=G7 H7 G8 G6 G9 G10 F8 E9 I8",
			num:  31, side: rules.Blue, move: "G7", tag: config.BotLogTagVCT,
			st: engine.SearchStats{
				Depth: 9, Nodes: 45_000, Nps: 1_200_000, EBFMilli: 1400,
				TTHitPermille: 180, HashFullPermille: 600, FirstMoveFailHighPermille: 880,
				Score:     config.EvalMateMax - 9*config.EvalMateScoreStep,
				Threads:   4,
				ElapsedNs: 30_000_000, AllocNs: 3_000_000_000,
			},
			pv: []string{"G7", "H7", "G8", "G6", "G9", "G10", "F8", "E9", "I8"},
		},
		{
			name: "ponder hit",
			line: "M12, Red, H10, d=16, n=18m, nps=2.8m, ebf=2.0, tt=44%, hf=78%, fh1=91%, s=-25, thr=4, t=0.01, alloc=4.00, [PONDER], pv=H10 I9 J8 K7 J10",
			num:  12, side: rules.Red, move: "H10", tag: config.BotLogTagPonder,
			st: engine.SearchStats{
				Depth: 16, Nodes: 18_000_000, Nps: 2_800_000, EBFMilli: 2000,
				TTHitPermille: 440, HashFullPermille: 780, FirstMoveFailHighPermille: 910,
				Score: -25, Threads: 4, ElapsedNs: 10_000_000, AllocNs: 4_000_000_000,
			},
			pv: []string{"H10", "I9", "J8", "K7", "J10"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setPV(t, &c.st, c.pv...)
			if got := MLine(c.num, c.side, mustMove(t, c.move), &c.st, c.tag); got != c.line {
				t.Errorf("MLine:\n got %q\nwant %q", got, c.line)
			}
		})
	}
}

// TestMLineMatchesBotLogFormat pins the emitter against the canonical
// config.BotLogFormat on values off the golden grid: compact rounding
// boundaries, permille rounding, the empty PV trailing "pv=", and a zeroed
// search.
func TestMLineMatchesBotLogFormat(t *testing.T) {
	rich := engine.SearchStats{
		Depth: 7, Nodes: 1_050_000, Nps: 999_995, EBFMilli: 1535,
		TTHitPermille: 385, HashFullPermille: 454, FirstMoveFailHighPermille: 929,
		Score:     -(config.EvalMateMax - 3*config.EvalMateScoreStep),
		Threads:   8,
		ElapsedNs: 1_234_567_890, AllocNs: 9_876_543_210,
	}
	setPV(t, &rich, "A1")
	empty := engine.SearchStats{
		Depth: 1, Nodes: 0, Nps: 0, EBFMilli: 0, Score: 0,
		Threads: 1, ElapsedNs: 0, AllocNs: 0,
	}
	cases := []struct {
		name string
		num  int
		side rules.Color
		move string
		tag  string
		st   *engine.SearchStats
		args []any
	}{
		{
			name: "rich with vcf tag", num: 7, side: rules.Blue, move: "A1",
			tag: config.BotLogTagVCF, st: &rich,
			args: []any{7, "Blue", "A1", 7, "1.05m", "1m", 1.5, 39, 45, 93, "-M3", 8,
				float64(rich.ElapsedNs) / float64(time.Second),
				float64(rich.AllocNs) / float64(time.Second),
				config.BotLogTagVCF, "A1"},
		},
		{
			name: "zeroed empty pv", num: 1, side: rules.Red, move: "A1",
			tag: "", st: &empty,
			args: []any{1, "Red", "A1", 1, "0", "0", 0.0, 0, 0, 0, "+0", 1, 0.0, 0.0, "", ""},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := fmt.Sprintf(config.BotLogFormat, c.args...)
			got := MLine(c.num, c.side, mustMove(t, c.move), c.st, c.tag)
			if got != want {
				t.Errorf("MLine vs BotLogFormat:\n got %q\nwant %q", got, want)
			}
		})
	}
	if got := MLine(cases[1].num, cases[1].side, mustMove(t, cases[1].move), cases[1].st, ""); got[len(got)-3:] != "pv=" {
		t.Errorf("empty PV must leave a bare trailing pv=, got %q", got)
	}
}

// TestCompactNumberTable pins the n=/nps= rule: plain digits below a
// thousand, then a mantissa of at most three significant digits with
// trailing zeros trimmed, k below a million, m at and above. Second decimal
// rounds half up; a mantissa rounding to 1000k promotes to 1m, and above 1e9
// the mantissa widens because the spec has no billion scale.
func TestCompactNumberTable(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{42, "42"},
		{999, "999"},
		{1_000, "1k"},
		{1_050, "1.05k"},
		{1_500, "1.5k"},
		{45_000, "45k"},
		{45_500, "45.5k"},
		{999_994, "999.99k"},
		{999_995, "1m"},
		{999_999, "1m"},
		{1_000_000, "1m"},
		{1_004_999, "1m"},
		{1_005_000, "1.01m"},
		{1_050_000, "1.05m"},
		{1_999_999, "2m"},
		{2_500_000, "2.5m"},
		{6_250_000, "6.25m"},
		{18_000_000, "18m"},
		{123_400_000, "123.4m"},
		{999_995_000, "1000m"},
		{2_500_000_000, "2500m"},
	}
	var buf []byte
	for _, c := range cases {
		if got := string(appendCompact(buf[:0], c.in)); got != c.want {
			t.Errorf("appendCompact(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestScoreFormatting pins s=: signed integer milliunits inside the mate
// band (zero keeps the plus so the sign column never varies), mate scores as
// M-distance over the mateWin lattice, negated lattice points as -M-distance,
// and the early-break boundary EvalMateMax - EvalMateScoreStep = mateWin(0)
// as M1.
func TestScoreFormatting(t *testing.T) {
	mate := func(dist int) int {
		return config.EvalMateMax - dist*config.EvalMateScoreStep
	}
	cases := []struct {
		in   int
		want string
	}{
		{150, "+150"},
		{25, "+25"},
		{0, "+0"},
		{-25, "-25"},
		{-1, "-1"},
		{mate(1), "M1"},
		{mate(2), "M2"},
		{mate(9), "M9"},
		{mate(config.SearchMaxPly), "M64"},
		{-mate(9), "-M9"},
		{-mate(1), "-M1"},
		{-mate(config.SearchMaxPly), "-M64"},
	}
	var buf []byte
	for _, c := range cases {
		if got := string(appendScore(buf[:0], c.in)); got != c.want {
			t.Errorf("appendScore(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestStatsCellNameMatchesCodec walks every board cell and holds the
// emitter's allocation-free coordinate naming to rules.CellName, the
// canonical codec.
func TestStatsCellNameMatchesCodec(t *testing.T) {
	var buf []byte
	for cell := 0; cell < config.BoardCells; cell++ {
		want, err := rules.CellName(rules.Cell(cell))
		if err != nil {
			t.Fatalf("cell %d: %v", cell, err)
		}
		if got := string(appendCellName(buf[:0], rules.Move(cell))); got != want {
			t.Fatalf("appendCellName(%d) = %q, want %q", cell, got, want)
		}
	}
}

func TestAppendMLineZeroAlloc(t *testing.T) {
	st := &engine.SearchStats{
		Depth: config.SearchMaxPly, Nodes: 999_999_999_999, Nps: 123_456_789,
		EBFMilli: 32000, TTHitPermille: 1000, HashFullPermille: 1000,
		FirstMoveFailHighPermille: 1000,
		Score:                     config.EvalMateMax - config.EvalMateScoreStep,
		Threads:                   16, ElapsedNs: 9_000_000_000_000, AllocNs: 9_000_000_000_000,
	}
	for i := range st.PV {
		st.PV[i] = mustMove(t, "P16")
	}
	st.PVLen = config.SearchMaxPly
	buf := make([]byte, 0, 512)
	move := mustMove(t, "P16")
	allocs := testing.AllocsPerRun(100, func() {
		buf = AppendMLine(buf[:0], 999_999, rules.Red, move, st, config.BotLogTagPonder)
	})
	if allocs != 0 {
		t.Errorf("AppendMLine allocated %v times per run, want 0 (line %q)", allocs, buf)
	}
}
