package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type header struct {
	run, series                          int64
	redName, redTier, blueName, blueTier string
	initialMin, incSec, boLen            int
}

type mLine struct {
	move     int
	side     string
	depth    int
	nodes    uint64
	nps      uint64
	score    int
	t, alloc float64
	tag      string
}

type gameLine struct {
	game    int
	outcome string
	moves   int
	wonBy   string
}

type verdictLine struct {
	winner                string
	firstWins, secondWins int
}

type badLine struct {
	path   string
	lineno int
	text   string
}

type record struct {
	ml *mLine
	gl *gameLine
}

type seriesFile struct {
	path    string
	head    header
	hasHead bool
	records []record
	verdict *verdictLine
	bad     []badLine
}

const (
	compactK = 1_000
	compactM = 1_000_000
)

var mateFloor = config.EvalMateMax - config.SearchMaxPly*config.EvalMateScoreStep

var tagBodies = []string{
	strings.TrimPrefix(config.BotLogTagVCF, ", "),
	strings.TrimPrefix(config.BotLogTagVCT, ", "),
	strings.TrimPrefix(config.BotLogTagPonder, ", "),
}

func readSeriesLogs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !strings.HasSuffix(name, ".txt") || !seriesLogName(name) {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	slices.Sort(paths)
	return paths, nil
}

func seriesLogName(name string) bool {
	rest, ok := strings.CutSuffix(name, ".txt")
	if !ok {
		return false
	}
	i := strings.IndexByte(rest, '_')
	return i > 0 && strings.HasPrefix(rest, "run") && strings.HasPrefix(rest[i+1:], "s")
}

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
			f.head.initialMin, f.head.incSec, f.head.boLen, sawTC = initial, inc, bo, true
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

func parseRunLine(line string) (run, series int64, ok bool) {
	n, err := fmt.Sscanf(line, "run %d series %d", &run, &series)
	return run, series, err == nil && n == 2
}

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

func parseTCLine(line string) (initial, inc, bo int, ok bool) {
	n, err := fmt.Sscanf(line, "tc %d+%d bo%d", &initial, &inc, &bo)
	return initial, inc, bo, err == nil && n == 3 && initial >= 0 && inc >= 0 && bo >= 1
}

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
	body := rest[i+2:]
	switch {
	case strings.Contains(body, " (red) beat "):
		g.outcome = outcomeRed
	case strings.Contains(body, " (blue) beat "):
		g.outcome = outcomeBlue
	case strings.Contains(body, " (red) vs ") && strings.Contains(body, ", drawn at "):
		g.outcome = outcomeDraw
	default:
		return g, false
	}
	movesTok := body
	if k, wb, found := strings.Cut(body, ", won by "); found {
		if wb != wonByFour && wb != wonByOpenFour && wb != wonByDoubleFour {
			return g, false
		}
		movesTok, g.wonBy = k, wb
	}
	j := strings.LastIndex(movesTok, ", ")
	if j < 0 {
		return g, false
	}
	tail, ok2 := strings.CutSuffix(movesTok[j+2:], " moves")
	if !ok2 {
		return g, false
	}
	tail = strings.TrimPrefix(tail, "drawn at ")
	if g.moves, ok2 = intAt(tail); !ok2 {
		return g, false
	}
	return g, true
}

func parseVerdictLine(line string) (v verdictLine, ok bool) {
	rest, ok2 := strings.CutPrefix(line, "series ")
	if !ok2 {
		return v, false
	}
	i := strings.LastIndex(rest, " ")
	if i <= 0 {
		return v, false
	}
	scoreTok := rest[i+1:]
	v.winner = rest[:i]
	if v.winner == "" {
		return v, false
	}
	score := strings.SplitN(scoreTok, "-", 2)
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
	if m.depth, ok = intField(mid[2], "d="); !ok {
		return m, false
	}
	if m.nodes, ok = compactField(mid[3], "n="); !ok {
		return m, false
	}
	if m.nps, ok = compactField(mid[4], "nps="); !ok {
		return m, false
	}
	if _, ok = floatAt(mid[5], "ebf="); !ok {
		return m, false
	}
	if _, ok = pctAt(mid[6], "tt="); !ok {
		return m, false
	}
	if _, ok = pctAt(mid[7], "hf="); !ok {
		return m, false
	}
	if _, ok = pctAt(mid[8], "fh1="); !ok {
		return m, false
	}
	if m.score, ok = scoreField(mid[9]); !ok {
		return m, false
	}
	if _, ok = intField(mid[10], "thr="); !ok {
		return m, false
	}
	if m.t, ok = floatAt(mid[11], "t="); !ok {
		return m, false
	}
	if m.alloc, ok = floatAt(mid[12], "alloc="); !ok {
		return m, false
	}
	return m, true
}

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

func floatAt(tok, key string) (float64, bool) {
	v, ok := strings.CutPrefix(tok, key)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) && f >= 0
}

func pctAt(tok, key string) (int, bool) {
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

func scoreField(tok string) (int, bool) {
	v, ok := strings.CutPrefix(tok, "s=")
	if !ok {
		return 0, false
	}
	return parseScore(v)
}

func parseScore(s string) (int, bool) {
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
