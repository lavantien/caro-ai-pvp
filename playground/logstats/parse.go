package main

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// header is one series file's three header lines parsed apart: the run and
// series ids, the pairing (first name = host = red in game 1), and the time
// control with the best-of length.
type header struct {
	run, series                          int64
	redName, redTier, blueName, blueTier string
	initialSec, incSec, boLen            int
}

// mLine is one parsed bot telemetry line.
type mLine struct {
	move        int
	side, cell  string
	depth       int
	nodes, nps  uint64
	ebf         float64
	tt, hf, fh1 int
	score       int
	thr         int
	t, alloc    float64
	tag, pv     string
}

// gameLine is one parsed game summary; wonBy is empty when the summary
// carries no classification.
type gameLine struct {
	game    int
	outcome string
	moves   int
	wonBy   string
}

// verdictLine is one parsed series verdict: the room side string and the
// billed first-minus-second participant wins.
type verdictLine struct {
	winner                string
	firstWins, secondWins int
}

// badLine is one line no parser accepted: counted and listed, never dropped.
type badLine struct {
	path   string
	lineno int
	text   string
}

// record keeps one file's data lines in arrival order, the order the seat
// fold walks them.
type record struct {
	ml *mLine
	gl *gameLine
}

// seriesFile is one parsed log file.
type seriesFile struct {
	path    string
	head    header
	hasHead bool
	records []record
	verdict *verdictLine
	bad     []badLine
}

// compactK and compactM mirror the emitter's compact scales
// (internal/server/stats.go compactKilo/compactMega): k below a million, m
// at and above. displayCompact renders means on the same scales.
const (
	compactK = 1_000
	compactM = 1_000_000
)

// mateFloor mirrors the emitter's mate band floor (internal/server/stats.go
// appendScore): scores at or beyond it are mate distances on the mateWin
// lattice, everything inside is a milliunit leaf score.
var mateFloor = config.EvalMateMax - config.SearchMaxPly*config.EvalMateScoreStep

// tagBodies are the M-line tag tokens, derived from the config constants the
// emitter appends after alloc.
var tagBodies = []string{
	strings.TrimPrefix(config.BotLogTagVCF, ", "),
	strings.TrimPrefix(config.BotLogTagVCT, ", "),
	strings.TrimPrefix(config.BotLogTagPonder, ", "),
}

// parseFile reads one series log line by line, classifying by the writer's
// leading tokens. Duplicated structural lines (a second header line, a
// second verdict) and anything unparsable land in the bad ledger; data lines
// accumulate in arrival order. A trailing newline is the writer's, a blank
// or CRLF-polluted line is corruption and is ledgered.
func parseFile(path string) (seriesFile, error) {
	f := seriesFile{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return f, fmt.Errorf("read %s: %w", path, err)
	}
	var sawRun, sawPairing, sawTC, sawVerdict bool
	bad := func(lineno int, line string) {
		f.bad = append(f.bad, badLine{path: path, lineno: lineno, text: line})
	}
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		lineno := i + 1
		switch {
		case strings.HasPrefix(line, "run "):
			if sawRun {
				bad(lineno, line)
				continue
			}
			run, series, ok := parseRunLine(line)
			if !ok {
				bad(lineno, line)
				continue
			}
			f.head.run, f.head.series, sawRun = run, series, true
		case strings.HasPrefix(line, "pairing "):
			if sawPairing {
				bad(lineno, line)
				continue
			}
			rn, rt, bn, bt, ok := parsePairingLine(line)
			if !ok {
				bad(lineno, line)
				continue
			}
			f.head.redName, f.head.redTier, f.head.blueName, f.head.blueTier, sawPairing =
				rn, rt, bn, bt, true
		case strings.HasPrefix(line, "tc "):
			if sawTC {
				bad(lineno, line)
				continue
			}
			initial, inc, bo, ok := parseTCLine(line)
			if !ok {
				bad(lineno, line)
				continue
			}
			f.head.initialSec, f.head.incSec, f.head.boLen, sawTC = initial, inc, bo, true
		case strings.HasPrefix(line, "game "):
			if g, ok := parseGameLine(line); ok {
				f.records = append(f.records, record{gl: &g})
			} else {
				bad(lineno, line)
			}
		case strings.HasPrefix(line, "series "):
			if sawVerdict {
				bad(lineno, line)
				continue
			}
			if v, ok := parseVerdictLine(line); ok {
				f.verdict, sawVerdict = &v, true
			} else {
				bad(lineno, line)
			}
		case len(line) > 1 && line[0] == 'M' && line[1] >= '0' && line[1] <= '9':
			if m, ok := parseMLine(line); ok {
				f.records = append(f.records, record{ml: &m})
			} else {
				bad(lineno, line)
			}
		default:
			bad(lineno, line)
		}
	}
	f.hasHead = sawRun && sawPairing && sawTC
	return f, nil
}

// parseRunLine reads the header's opening line, the first of the three
// tourney.Logs.WriteSeriesHeader writes.
func parseRunLine(line string) (run, series int64, ok bool) {
	n, err := fmt.Sscanf(line, "run %d series %d", &run, &series)
	return run, series, err == nil && n == 2
}

// parsePairingLine reads the header's pairing line: the first name is the
// host, red in game 1. Tier names come from config.Tiers and never hold
// parentheses, so the split rides the last ") vs " boundary and each side
// cuts at its last " (".
func parsePairingLine(line string) (redName, redTier, blueName, blueTier string, ok bool) {
	rest, ok := strings.CutPrefix(line, "pairing ")
	if !ok {
		return "", "", "", "", false
	}
	i := strings.LastIndex(rest, ") vs ")
	if i < 0 {
		return "", "", "", "", false
	}
	if redName, redTier, ok = cutTier(rest[:i+1]); !ok {
		return "", "", "", "", false
	}
	blueName, blueTier, ok = cutTier(rest[i+len(") vs "):])
	return redName, redTier, blueName, blueTier, ok
}

// cutTier splits "name (tier)" at its last " (" boundary.
func cutTier(s string) (name, tier string, ok bool) {
	if !strings.HasSuffix(s, ")") {
		return "", "", false
	}
	j := strings.LastIndex(s, " (")
	if j < 0 {
		return "", "", false
	}
	name, tier = s[:j], s[j+2:len(s)-1]
	if name == "" || tier == "" || strings.ContainsAny(tier, "()") {
		return "", "", false
	}
	return name, tier, true
}

// parseTCLine reads the header's time-control line.
func parseTCLine(line string) (initial, inc, bo int, ok bool) {
	n, err := fmt.Sscanf(line, "tc %d+%d bo%d", &initial, &inc, &bo)
	return initial, inc, bo, err == nil && n == 3 && initial >= 0 && inc >= 0 && bo >= 1
}

// parseGameLine reads the per-game summary. It mirrors the unexported sprint
// of internal/tourney/runner.go:422-425: "game %d: %s, %d moves" with
// ", won by "+tag appended only when the game carries a classification
// (server.WonByFour, WonByOpenFour, WonByDoubleFour); draws omit it.
func parseGameLine(line string) (g gameLine, ok bool) {
	rest, ok := strings.CutPrefix(line, "game ")
	if !ok {
		return g, false
	}
	i := strings.Index(rest, ": ")
	if i < 0 {
		return g, false
	}
	if g.game, ok = intAt(rest[:i]); !ok || g.game < 1 {
		return g, false
	}
	parts := strings.SplitN(rest[i+2:], ", ", 2)
	if len(parts) != 2 {
		return g, false
	}
	switch parts[0] {
	case server.OutcomeRed, server.OutcomeBlue, server.OutcomeDraw:
		g.outcome = parts[0]
	default:
		return g, false
	}
	movesTok := parts[1]
	if k, wb, found := strings.Cut(parts[1], ", won by "); found {
		if wb != server.WonByFour && wb != server.WonByOpenFour && wb != server.WonByDoubleFour {
			return g, false
		}
		movesTok, g.wonBy = k, wb
	}
	v, ok2 := strings.CutSuffix(movesTok, " moves")
	if !ok2 {
		return g, false
	}
	if g.moves, ok2 = intAt(v); !ok2 {
		return g, false
	}
	return g, true
}

// parseVerdictLine reads the closing verdict. It mirrors the unexported
// sprint of internal/tourney/runner.go:479-481: "series %s %d-%d" over the
// room side string and the billed red-first minus blue-first wins.
func parseVerdictLine(line string) (v verdictLine, ok bool) {
	rest, ok2 := strings.CutPrefix(line, "series ")
	if !ok2 {
		return v, false
	}
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) != 2 {
		return v, false
	}
	switch parts[0] {
	case server.SideHost.String(), server.SideGuest.String(), server.SideNone.String():
		v.winner = parts[0]
	default:
		return v, false
	}
	score := strings.SplitN(parts[1], "-", 2)
	if len(score) != 2 {
		return v, false
	}
	if v.firstWins, ok = intAt(score[0]); !ok {
		return v, false
	}
	if v.secondWins, ok = intAt(score[1]); !ok {
		return v, false
	}
	return v, true
}

// parseMLine splits one Implication 1.5 M-line on its ", " separators and
// reads every field back. The tail shape is alloc=<f>, an optional tag token
// ([VCF], [VCT], [PONDER]), then the final pv=<cells or empty>; cells are
// validated through rules.ParseCell, the codec the emitter names moves with.
func parseMLine(line string) (m mLine, ok bool) {
	parts := strings.Split(line, ", ")
	if len(parts) < 15 {
		return m, false
	}
	if !strings.HasPrefix(parts[len(parts)-1], "pv=") {
		return m, false
	}
	mid := parts[1 : len(parts)-1]
	if len(mid) == 14 {
		if !slices.Contains(tagBodies, mid[13]) {
			return m, false
		}
		m.tag = mid[13]
		mid = mid[:13]
	}
	if len(mid) != 13 {
		return m, false
	}
	if m.move, ok = intAt(strings.TrimPrefix(parts[0], "M")); !ok || m.move < 1 {
		return m, false
	}
	if mid[0] != "Red" && mid[0] != "Blue" {
		return m, false
	}
	m.side = mid[0]
	m.cell = mid[1]
	if _, err := rules.ParseCell(m.cell); err != nil {
		return m, false
	}
	if m.depth, ok = intField(mid[2], "d="); !ok {
		return m, false
	}
	if m.nodes, ok = compactField(mid[3], "n="); !ok {
		return m, false
	}
	if m.nps, ok = compactField(mid[4], "nps="); !ok {
		return m, false
	}
	if m.ebf, ok = floatField(mid[5], "ebf="); !ok {
		return m, false
	}
	if m.tt, ok = pctField(mid[6], "tt="); !ok {
		return m, false
	}
	if m.hf, ok = pctField(mid[7], "hf="); !ok {
		return m, false
	}
	if m.fh1, ok = pctField(mid[8], "fh1="); !ok {
		return m, false
	}
	if m.score, ok = scoreField(mid[9]); !ok {
		return m, false
	}
	if m.thr, ok = intField(mid[10], "thr="); !ok {
		return m, false
	}
	if m.t, ok = floatField(mid[11], "t="); !ok {
		return m, false
	}
	if m.alloc, ok = floatField(mid[12], "alloc="); !ok {
		return m, false
	}
	m.pv = strings.TrimPrefix(parts[len(parts)-1], "pv=")
	if m.pv != "" {
		for _, cell := range strings.Split(m.pv, " ") {
			if _, err := rules.ParseCell(cell); err != nil {
				return m, false
			}
		}
	}
	return m, true
}

// intAt reads a plain non-negative integer.
func intAt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 0
}

func intField(tok, key string) (int, bool) {
	v, ok := strings.CutPrefix(tok, key)
	if !ok {
		return 0, false
	}
	return intAt(v)
}

func floatField(tok, key string) (float64, bool) {
	v, ok := strings.CutPrefix(tok, key)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) && f >= 0
}

func pctField(tok, key string) (int, bool) {
	v, ok := strings.CutPrefix(tok, key)
	if !ok {
		return 0, false
	}
	if v, ok = strings.CutSuffix(v, "%"); !ok {
		return 0, false
	}
	n, ok := intAt(v)
	return n, ok && n <= 100
}

func compactField(tok, key string) (uint64, bool) {
	v, ok := strings.CutPrefix(tok, key)
	if !ok {
		return 0, false
	}
	return parseCompact(v)
}

// scoreField reads s=<score>: a mate marker M<dist>/-M<dist> on the mateWin
// lattice (config.EvalMateMax - dist*config.EvalMateScoreStep), else a
// signed milliunit integer whose zero keeps its plus (+0).
func scoreField(tok string) (int, bool) {
	v, ok := strings.CutPrefix(tok, "s=")
	if !ok {
		return 0, false
	}
	return parseScore(v)
}

func parseScore(s string) (int, bool) {
	// maxDist bounds the distance so the lattice arithmetic cannot overflow.
	maxDist := config.EvalMateMax / config.EvalMateScoreStep
	if dist, cut := strings.CutPrefix(s, "-M"); cut {
		d, ok := intAt(dist)
		if !ok || d < 1 || d > maxDist {
			return 0, false
		}
		return -(config.EvalMateMax - d*config.EvalMateScoreStep), true
	}
	if dist, cut := strings.CutPrefix(s, "M"); cut {
		d, ok := intAt(dist)
		if !ok || d < 1 || d > maxDist {
			return 0, false
		}
		return config.EvalMateMax - d*config.EvalMateScoreStep, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// parseCompact reads one compact count (6.25m, 45k, 2500m, 0) back to its
// integer value; the emitter's three significant digits make the round trip
// exact for every value it writes.
func parseCompact(s string) (uint64, bool) {
	body, scale := s, uint64(1)
	if b, ok := strings.CutSuffix(s, "k"); ok {
		body, scale = b, compactK
	} else if b, ok := strings.CutSuffix(s, "m"); ok {
		body, scale = b, compactM
	}
	if body == "" || strings.ContainsAny(body, "+-eE") {
		return 0, false
	}
	f, err := strconv.ParseFloat(body, 64)
	if err != nil || math.IsNaN(f) || f < 0 || f*float64(scale) >= math.MaxUint64 {
		return 0, false
	}
	return uint64(math.Round(f * float64(scale))), true
}
