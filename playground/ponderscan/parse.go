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

type mLine struct {
	move  int
	side  string
	nodes uint64
	nps   uint64
	t     float64
	alloc float64
	tag   string
}

type gameLine struct {
	game    int
	outcome string
}

type seriesFile struct {
	path     string
	redName  string
	blueName string
	records  []record
}

type record struct {
	ml *mLine
	gl *gameLine
}

const (
	compactK = 1_000
	compactM = 1_000_000
)

var tagBodies = []string{
	strings.TrimPrefix(config.BotLogTagVCF, ", "),
	strings.TrimPrefix(config.BotLogTagVCT, ", "),
	strings.TrimPrefix(config.BotLogTagPonder, ", "),
}

var ponderBody = strings.TrimPrefix(config.BotLogTagPonder, ", ")

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
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, "pairing "):
			rn, bn, ok := parsePairingNames(line)
			if !ok {
				return f, fmt.Errorf("%s:%d: bad pairing line", path, i+1)
			}
			f.redName, f.blueName = rn, bn
		case strings.HasPrefix(line, "game "):
			g, ok := parseGameLine(line)
			if !ok {
				return f, fmt.Errorf("%s:%d: bad game line", path, i+1)
			}
			f.records = append(f.records, record{gl: &g})
		case len(line) > 1 && line[0] == 'M' && line[1] >= '0' && line[1] <= '9':
			m, ok := parseMLine(line)
			if !ok {
				return f, fmt.Errorf("%s:%d: bad m-line", path, i+1)
			}
			f.records = append(f.records, record{ml: &m})
		}
	}
	if f.redName == "" {
		return f, fmt.Errorf("%s: no pairing header", path)
	}
	return f, nil
}

func parsePairingNames(line string) (string, string, bool) {
	rest, ok := strings.CutPrefix(line, "pairing ")
	if !ok {
		return "", "", false
	}
	i := strings.LastIndex(rest, ") vs ")
	if i < 0 {
		return "", "", false
	}
	red, _, ok := cutTier(rest[:i+1])
	if !ok {
		return "", "", false
	}
	blue, _, ok := cutTier(rest[i+len(") vs "):])
	if !ok {
		return "", "", false
	}
	return red, blue, true
}

func cutTier(s string) (string, string, bool) {
	if !strings.HasSuffix(s, ")") {
		return "", "", false
	}
	j := strings.LastIndex(s, " (")
	if j < 0 {
		return "", "", false
	}
	name, tier := s[:j], s[j+2:len(s)-1]
	if name == "" || tier == "" || strings.ContainsAny(tier, "()") {
		return "", "", false
	}
	return name, tier, true
}

func parseGameLine(line string) (g gameLine, ok bool) {
	rest, ok := strings.CutPrefix(line, "game ")
	if !ok {
		return g, false
	}
	head, body, found := strings.Cut(rest, ": ")
	if !found {
		return g, false
	}
	if g.game, ok = intAt(head); !ok || g.game < 1 {
		return g, false
	}
	switch {
	case strings.Contains(body, " (red) beat "):
		g.outcome = "red"
	case strings.Contains(body, " (blue) beat "):
		g.outcome = "blue"
	case strings.Contains(body, ", drawn at "):
		g.outcome = "draw"
	default:
		return g, false
	}
	return g, true
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
	if m.nodes, ok = compactField(mid[3], "n="); !ok {
		return m, false
	}
	if m.nps, ok = compactField(mid[4], "nps="); !ok {
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

func floatAt(tok, key string) (float64, bool) {
	v, ok := strings.CutPrefix(tok, key)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) && f >= 0
}

func compactField(tok, key string) (uint64, bool) {
	v, ok := strings.CutPrefix(tok, key)
	if !ok {
		return 0, false
	}
	return parseCompact(v)
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
