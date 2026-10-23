package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

func writeWithRealLogs(t *testing.T, run, series int64, tcIdx, boLen int,
	red, blue tourney.Participant, variant string, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	logs := tourney.NewLogs(dir)
	if err := logs.WriteSeriesHeader(run, series, tcIdx, boLen, red, blue, variant); err != nil {
		t.Fatalf("write header: %v", err)
	}
	for _, l := range lines {
		if err := logs.WriteSeriesLine(run, series, l); err != nil {
			t.Fatalf("write line %q: %v", l, err)
		}
	}
	if err := logs.CloseSeries(run, series); err != nil {
		t.Fatalf("close series: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("log dir holds %d files (err %v), want the one series log", len(entries), err)
	}
	return filepath.Join(dir, entries[0].Name())
}

func TestHeaderPinnedToRealWriter(t *testing.T) {
	tcIdx, ok := config.TCIndex(3, 2)
	if !ok {
		t.Fatalf("config carries no 3+2 time control")
	}
	tc := config.TimeControls[tcIdx]
	red := tourney.Participant{Slot: 0, Name: "alpha bot", Tier: "easy"}
	blue := tourney.Participant{Slot: 1, Name: "beta-2", Tier: "hard"}
	path := writeWithRealLogs(t, 7, 3, tcIdx, 5, red, blue, config.DefaultVariant, nil)
	f, err := parseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !f.hasHead {
		t.Fatalf("header not recognized: %+v", f.head)
	}
	want := header{
		run: 7, series: 3,
		redName: "alpha bot", redTier: "easy",
		blueName: "beta-2", blueTier: "hard",
		initialMin: tc.InitialMin, incSec: tc.IncrementSec, boLen: 5,
		variant: config.DefaultVariant,
	}
	if f.head != want {
		t.Errorf("header = %+v, want %+v", f.head, want)
	}
	if len(f.bad) != 0 {
		t.Errorf("bad lines = %v, want none", f.bad)
	}
	if f.verdict != nil {
		t.Errorf("verdict = %+v, want none", *f.verdict)
	}
}

func TestLinesPinnedToRealWriter(t *testing.T) {
	tcIdx, _ := config.TCIndex(1, 0)
	red := tourney.Participant{Slot: 0, Name: "gamma", Tier: "medium"}
	blue := tourney.Participant{Slot: 1, Name: "delta", Tier: "easy"}
	st := engine.SearchStats{
		Depth: 6, Nodes: 12_500, Nps: 625_000, EBFMilli: 1800,
		TTHitPermille: 120, HashFullPermille: 340, FirstMoveFailHighPermille: 900,
		Score: -75, Threads: 2, ElapsedNs: 200_000_000, AllocNs: 1_000_000_000,
	}
	st.PV[0], st.PV[1] = mustMove(t, "D4"), mustMove(t, "E5")
	st.PVLen = 2
	ml := server.MLine(2, rules.Blue, mustMove(t, "D4"), &st, config.BotLogTagVCF)
	path := writeWithRealLogs(t, 1, 0, tcIdx, 3, red, blue, config.DefaultVariant, []string{
		ml,
		"game 1: delta (blue) beat gamma (red), 11 moves, won by double 4",
		"series delta 0-1",
	})
	f, err := parseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(f.bad) != 0 {
		t.Errorf("bad lines = %v, want none", f.bad)
	}
	if len(f.records) != 2 {
		t.Fatalf("records = %d, want the M-line and the game summary", len(f.records))
	}
	m := f.records[0].ml
	if m == nil || m.move != 2 || m.side != "Blue" || m.cell != "D4" || m.cellFlat != 51 ||
		m.tag != strings.TrimPrefix(config.BotLogTagVCF, ", ") || m.pv != "D4 E5" {
		t.Errorf("m-line record = %+v", m)
	}
	g := f.records[1].gl
	if g == nil || g.game != 1 || g.outcome != server.OutcomeBlue ||
		g.moves != 11 || g.wonBy != server.WonByDoubleFour {
		t.Errorf("game record = %+v", g)
	}
	if f.verdict == nil || f.verdict.winner != "delta" ||
		f.verdict.firstWins != 0 || f.verdict.secondWins != 1 {
		t.Errorf("verdict = %+v", f.verdict)
	}
}

func mustMove(t *testing.T, name string) rules.Move {
	t.Helper()
	cell, err := rules.ParseCell(name)
	if err != nil {
		t.Fatalf("parse cell %q: %v", name, err)
	}
	return rules.Move(cell)
}

func mustVariantGeom(t *testing.T, variant string) *geom.Geometry {
	t.Helper()
	g, err := geom.ByName(variant)
	if err != nil {
		t.Fatalf("geometry %q: %v", variant, err)
	}
	return g
}

func variantMove(t *testing.T, g *geom.Geometry, name string) rules.Move {
	t.Helper()
	cell, err := g.ParseCell(name)
	if err != nil {
		t.Fatalf("parse cell %q: %v", name, err)
	}
	return rules.Move(cell)
}

func TestHexSeriesParsesThroughTheHexGeometry(t *testing.T) {
	tcIdx, _ := config.TCIndex(1, 0)
	red := tourney.Participant{Slot: 0, Name: "alpha", Tier: "easy"}
	blue := tourney.Participant{Slot: 1, Name: "beta", Tier: "hard"}
	hex := mustVariantGeom(t, "hex")
	base := engine.SearchStats{
		Depth: 9, Nodes: 45_000, Nps: 1_200_000, EBFMilli: 1400,
		TTHitPermille: 180, HashFullPermille: 600, FirstMoveFailHighPermille: 880,
		Score: 150, Threads: 4, ElapsedNs: 30_000_000, AllocNs: 3_000_000_000,
	}
	withPV := base
	withPV.PV[0], withPV.PVLen = variantMove(t, hex, "I17"), 1
	lines := []string{
		string(server.AppendMLine(nil, hex, 3, rules.Red, variantMove(t, hex, "W12"), &base, "")),
		string(server.AppendMLine(nil, hex, 5, rules.Red, variantMove(t, hex, "L23"), &base, config.BotLogTagVCF)),
		string(server.AppendMLine(nil, hex, 7, rules.Red, variantMove(t, hex, "L12"), &withPV, "")),
	}
	path := writeWithRealLogs(t, 9, 1, tcIdx, 3, red, blue, "hex", lines)
	f, err := parseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f.head.variant != "hex" {
		t.Errorf("header variant = %q, want hex", f.head.variant)
	}
	if len(f.bad) != 0 {
		t.Fatalf("bad lines = %v, want the hex-only cell names to parse", f.bad)
	}
	if len(f.records) != 3 {
		t.Fatalf("records = %d, want the three hex M-lines", len(f.records))
	}
	want := []struct {
		num  int
		cell string
		flat int
	}{
		{3, "W12", 275},
		{5, "L23", 517},
		{7, "L12", 264},
	}
	for i, w := range want {
		m := f.records[i].ml
		if m == nil || m.move != w.num || m.cell != w.cell || m.cellFlat != w.flat {
			t.Errorf("hex M-line %d = %+v, want move %d cell %s flat %d", i, m, w.num, w.cell, w.flat)
		}
	}
	if m := f.records[2].ml; m.pv != "I17" {
		t.Errorf("hex pv = %q, want I17", m.pv)
	}

	sq := mustVariantGeom(t, config.DefaultVariant)
	sqLine := string(server.AppendMLine(nil, sq, 7, rules.Red, variantMove(t, sq, "L12"), &base, ""))
	sqPath := writeWithRealLogs(t, 9, 2, tcIdx, 3, red, blue, config.DefaultVariant, []string{sqLine})
	sf, err := parseFile(sqPath)
	if err != nil {
		t.Fatalf("parse square: %v", err)
	}
	if len(sf.bad) != 0 || len(sf.records) != 1 {
		t.Fatalf("square parse bad %v records %d, want a clean single M-line", sf.bad, len(sf.records))
	}
	if m := sf.records[0].ml; m.cell != "L12" || m.cellFlat != 187 {
		t.Errorf("square L12 flat = %d, want 187", m.cellFlat)
	}

	offRegion := string(server.AppendMLine(nil, hex, 8, rules.Red, rules.Move(23), &base, ""))
	if _, ok := parseMLine(offRegion, hex); ok {
		t.Error("off-region cell A2 accepted under the hex geometry, want rejection")
	}
}

func TestMLinePinnedToRealEmitter(t *testing.T) {
	cases := []struct {
		name string
		num  int
		side rules.Color
		move string
		flat int
		tag  string
		st   engine.SearchStats
		pv   []string
	}{
		{name: "healthy", num: 24, side: rules.Red, move: "J9", flat: 137, tag: "",
			st: engine.SearchStats{
				Depth: 14, Nodes: 6_250_000, Nps: 2_500_000, EBFMilli: 2100,
				TTHitPermille: 380, HashFullPermille: 450, FirstMoveFailHighPermille: 930,
				Score: 150, Threads: 4, ElapsedNs: 2_500_000_000, AllocNs: 2_500_000_000,
			},
			pv: []string{"J9", "K10", "K9", "L9", "M8", "L8"}},
		{name: "mate for mover", num: 31, side: rules.Blue, move: "G7", flat: 102, tag: config.BotLogTagVCT,
			st: engine.SearchStats{
				Depth: 9, Nodes: 45_000, Nps: 1_200_000, EBFMilli: 1400,
				TTHitPermille: 180, HashFullPermille: 600, FirstMoveFailHighPermille: 880,
				Score: config.EvalMateMax - 9*config.EvalMateScoreStep, Threads: 4,
				ElapsedNs: 30_000_000, AllocNs: 3_000_000_000,
			},
			pv: []string{"G7", "H7"}},
		{name: "mate against mover", num: 7, side: rules.Blue, move: "A1", flat: 0, tag: config.BotLogTagVCF,
			st: engine.SearchStats{
				Depth: 7, Nodes: 1_050_000, Nps: 2_000_000, EBFMilli: 1500,
				TTHitPermille: 385, HashFullPermille: 454, FirstMoveFailHighPermille: 929,
				Score: -(config.EvalMateMax - 3*config.EvalMateScoreStep), Threads: 8,
				ElapsedNs: 1_230_000_000, AllocNs: 9_870_000_000,
			},
			pv: []string{"A1"}},
		{name: "signed zero", num: 12, side: rules.Red, move: "H10", flat: 151, tag: config.BotLogTagPonder,
			st: engine.SearchStats{
				Depth: 16, Nodes: 18_000_000, Nps: 2_800_000, EBFMilli: 2000,
				TTHitPermille: 440, HashFullPermille: 780, FirstMoveFailHighPermille: 910,
				Score: 0, Threads: 4, ElapsedNs: 10_000_000, AllocNs: 4_000_000_000,
			},
			pv: []string{"H10", "I9"}},
		{name: "negative leaf", num: 3, side: rules.Red, move: "P16", flat: 255, tag: "",
			st: engine.SearchStats{
				Depth: 5, Nodes: 999, Nps: 999, EBFMilli: 1200,
				TTHitPermille: 10, HashFullPermille: 20, FirstMoveFailHighPermille: 30,
				Score: -25, Threads: 1, ElapsedNs: 0, AllocNs: 0,
			}},
		{name: "zeroed empty pv", num: 1, side: rules.Red, move: "A1", flat: 0, tag: "",
			st: engine.SearchStats{Depth: 1, Threads: 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for i, name := range c.pv {
				c.st.PV[i] = mustMove(t, name)
			}
			c.st.PVLen = len(c.pv)
			line := server.MLine(c.num, c.side, mustMove(t, c.move), &c.st, c.tag)
			m, ok := parseMLine(line, geom.Default())
			if !ok {
				t.Fatalf("parseMLine(%q) rejected the emitter's line", line)
			}
			side := "Red"
			if c.side == rules.Blue {
				side = "Blue"
			}
			checks := []struct {
				what string
				got  any
				want any
			}{
				{"move", m.move, c.num},
				{"side", m.side, side},
				{"cell", m.cell, c.move},
				{"cellFlat", m.cellFlat, c.flat},
				{"depth", m.depth, c.st.Depth},
				{"nodes", m.nodes, c.st.Nodes},
				{"nps", m.nps, c.st.Nps},
				{"ebf", m.ebf, float64(c.st.EBFMilli) / config.EvalMilliUnit},
				{"tt", m.tt, (c.st.TTHitPermille + 5) / 10},
				{"hf", m.hf, (c.st.HashFullPermille + 5) / 10},
				{"fh1", m.fh1, (c.st.FirstMoveFailHighPermille + 5) / 10},
				{"score", m.score, c.st.Score},
				{"thr", m.thr, c.st.Threads},
				{"t", m.t, float64(c.st.ElapsedNs) / 1e9},
				{"alloc", m.alloc, float64(c.st.AllocNs) / 1e9},
				{"tag", m.tag, strings.TrimPrefix(c.tag, ", ")},
				{"pv", m.pv, strings.Join(c.pv, " ")},
			}
			for _, ch := range checks {
				if ch.got != ch.want {
					t.Errorf("%s = %v, want %v (line %q)", ch.what, ch.got, ch.want, line)
				}
			}
		})
	}
}

func TestMLineRejectsCorruption(t *testing.T) {
	good := "M4, Red, C3, d=2, n=100, nps=50, ebf=1.2, tt=10%, hf=20%, fh1=30%, s=+5, thr=1, t=0.10, alloc=1.00, pv=D4"
	for what, bad := range map[string]string{
		"truncated":       "M4, Red, C3, d=2",
		"move zero":       strings.Replace(good, "M4", "M0", 1),
		"side":            strings.Replace(good, ", Red,", ", Green,", 1),
		"cell":            strings.Replace(good, ", C3,", ", C33,", 1),
		"depth":           strings.Replace(good, "d=2", "d=x", 1),
		"nodes":           strings.Replace(good, "n=100", "n=-5k", 1),
		"nps suffix":      strings.Replace(good, "nps=50", "nps=50q", 1),
		"tt percent":      strings.Replace(good, "tt=10%", "tt=10", 1),
		"score":           strings.Replace(good, "s=+5", "s=banana", 1),
		"threads":         strings.Replace(good, "thr=1", "thr=x", 1),
		"negative time":   strings.Replace(good, "t=0.10", "t=-1.00", 1),
		"unknown tag":     strings.Replace(good, "alloc=1.00,", "alloc=1.00, [BOGUS],", 1),
		"corrupt pv cell": strings.Replace(good, "pv=D4", "pv=Z1", 1),
	} {
		if _, ok := parseMLine(bad, geom.Default()); ok {
			t.Errorf("%s: parseMLine(%q) accepted a corrupt line", what, bad)
		}
	}
	if _, ok := parseMLine(good, geom.Default()); !ok {
		t.Errorf("parseMLine(%q) rejected a healthy line", good)
	}
}

func TestParseStructuralLines(t *testing.T) {
	if run, series, ok := parseRunLine("run 12 series 4"); !ok || run != 12 || series != 4 {
		t.Errorf("parseRunLine = %d %d %v", run, series, ok)
	}
	for _, bad := range []string{"run 12", "warmup 12 series 4", "run x series 4"} {
		if _, _, ok := parseRunLine(bad); ok {
			t.Errorf("parseRunLine(%q) accepted", bad)
		}
	}

	rn, rt, bn, bt, ok := parsePairingLine("pairing alpha bot (easy) vs beta-2 (hard)")
	if !ok || rn != "alpha bot" || rt != "easy" || bn != "beta-2" || bt != "hard" {
		t.Errorf("parsePairingLine = %q %q %q %q %v", rn, rt, bn, bt, ok)
	}
	for _, bad := range []string{
		"pairing alpha (easy) vs beta",
		"pairing alpha (easy) versus beta (hard)",
		"pairing (easy) vs beta (hard)",
		"pairing alpha () vs beta (hard)",
		"alpha (easy) vs beta (hard)",
	} {
		if _, _, _, _, ok := parsePairingLine(bad); ok {
			t.Errorf("parsePairingLine(%q) accepted", bad)
		}
	}

	if initial, inc, bo, variant, ok := parseTCLine("tc 1+0 bo3"); !ok || initial != 1 || inc != 0 || bo != 3 || variant != config.DefaultVariant {
		t.Errorf("parseTCLine = %d %d %d %q %v", initial, inc, bo, variant, ok)
	}
	if initial, inc, bo, variant, ok := parseTCLine("tc 1+0 bo3 hex"); !ok || initial != 1 || inc != 0 || bo != 3 || variant != "hex" {
		t.Errorf("parseTCLine hex = %d %d %d %q %v", initial, inc, bo, variant, ok)
	}
	for _, bad := range []string{
		"tc 1+0 bo0",
		"tc 1+0",
		"tc x+0 bo3",
		"time 1+0 bo3",
		"tc 1+0 bo3 hexagon",
		"tc 1+0 bo3 hex extra",
		"tc 1+0 bo3 hexagon hex",
	} {
		if _, _, _, _, ok := parseTCLine(bad); ok {
			t.Errorf("parseTCLine(%q) accepted", bad)
		}
	}

	if g, ok := parseGameLine("game 2: b (blue) beat a (red), 17 moves, won by open 4"); !ok ||
		g.game != 2 || g.outcome != server.OutcomeBlue || g.moves != 17 || g.wonBy != server.WonByOpenFour {
		t.Errorf("parseGameLine = %+v %v", g, ok)
	}
	if g, ok := parseGameLine("game 1: a (red) vs b (blue), drawn at 40 moves"); !ok ||
		g.outcome != server.OutcomeDraw || g.wonBy != "" {
		t.Errorf("parseGameLine draw = %+v %v", g, ok)
	}
	for _, bad := range []string{
		"game 0: a (red) beat b (blue), 9 moves",
		"game 1: a (green) beat b (blue), 9 moves",
		"game 1: a (red) beat",
		"game 1: a (red) beat b (blue), x moves",
		"game 1: a (red) beat b (blue), 9 moves, won by 3",
		"match 1: a (red) beat b (blue), 9 moves",
	} {
		if _, ok := parseGameLine(bad); ok {
			t.Errorf("parseGameLine(%q) accepted", bad)
		}
	}

	if v, ok := parseVerdictLine("series host 2-1"); !ok ||
		v.winner != server.SideHost.String() || v.firstWins != 2 || v.secondWins != 1 {
		t.Errorf("parseVerdictLine = %+v %v", v, ok)
	}
	for _, bad := range []string{
		"series 1-0",
		"series  2-1",
		"series only 2",
		"series only a-b",
		"series only -1-0",
		"result host 2-1",
	} {
		if _, ok := parseVerdictLine(bad); ok {
			t.Errorf("parseVerdictLine(%q) accepted", bad)
		}
	}
}

func TestParseFileLedgersBadLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run1_s0_a-vs-b.txt")
	body := "run 1 series 0\n" +
		"pairing a (easy) vs b (hard)\n" +
		"tc 1+0 bo3\n" +
		"M1, Red, A1, d=1, n=10, nps=10, ebf=1.0, tt=0%, hf=0%, fh1=0%, s=+0, thr=1, t=0.01, alloc=1.00, pv=\n" +
		"run 1 series 0\n" +
		"garbage\n" +
		"\n" +
		"series host 1-0\n" +
		"series host 1-0\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := parseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(f.bad) != 4 {
		t.Fatalf("bad = %v, want the duplicate run, garbage, blank, and duplicate verdict", f.bad)
	}
	wantLineno := []int{5, 6, 7, 9}
	for i, b := range f.bad {
		if b.lineno != wantLineno[i] || b.path != path {
			t.Errorf("bad[%d] = %s:%d, want lineno %d of %s", i, b.path, b.lineno, wantLineno[i], path)
		}
	}
	if f.bad[2].text != "" {
		t.Errorf("blank line ledger text = %q, want empty", f.bad[2].text)
	}
	if !f.hasHead || len(f.records) != 1 || f.verdict == nil {
		t.Errorf("healthy content lost: hasHead %v records %d verdict %v", f.hasHead, len(f.records), f.verdict)
	}
}
