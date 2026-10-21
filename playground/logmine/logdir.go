package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	pairingRe  = regexp.MustCompile(`^pairing (.+) \((.+)\) vs (.+) \((.+)\)$`)
	tcRe       = regexp.MustCompile(`^tc (\S+) bo(\d+)$`)
	gameWinRe  = regexp.MustCompile(`^game \d+: (.+) \((red|blue)\) beat (.+) \((red|blue)\), \d+ moves.*$`)
	gameDrawRe = regexp.MustCompile(`^game \d+: (.+) \(red\) vs (.+) \(blue\), drawn at \d+ moves$`)
	verdictRe  = regexp.MustCompile(`^series .+ (\d+)-(\d+)$`)
)

type seriesLog struct {
	tc         string
	firstTier  string
	secondTier string
	moves      []moveRow
	outcomes   []outcomeRow
	firstWins  int
	secondWins int
	rejects    int
}

func parseSeriesLog(body []byte) (seriesLog, error) {
	var s seriesLog
	names := map[string]string{}
	var pending []struct {
		side string
		ml   mLine
	}
	gameID := int64(0)
	flush := func(redTier, blueTier, outcome string) {
		gameID++
		for _, p := range pending {
			r := moveRow{
				tc: s.tc, gameID: gameID,
				depth: p.ml.depth, nodes: p.ml.nodes, nps: p.ml.nps,
				ebfMilli: int(p.ml.ebf * 1000), tt: p.ml.tt * 10, hf: p.ml.hf * 10, fh1: p.ml.fh1 * 10,
				threads: p.ml.thr, tag: p.ml.tag,
				elapsedNS: int64(p.ml.t * 1e9), allocNS: int64(p.ml.alloc * 1e9),
				moveNo: p.ml.move, scoreTxt: p.ml.score, pvLen: pvWordCount(p.ml.pv),
				cell: p.ml.cell, pvCells: strings.Fields(p.ml.pv),
			}
			if p.side == "Red" {
				r.tier = redTier
			} else {
				r.tier = blueTier
			}
			s.moves = append(s.moves, r)
		}
		pending = pending[:0]
		s.outcomes = append(s.outcomes, outcomeRow{
			key:   outcomeKey{tc: s.tc, redTier: redTier, blueTier: blueTier, outcome: outcome},
			count: 1,
		})
	}
	for _, raw := range splitLines(body) {
		switch {
		case pairingRe.MatchString(raw):
			m := pairingRe.FindStringSubmatch(raw)
			names[m[1]] = m[2]
			names[m[3]] = m[4]
			s.firstTier, s.secondTier = m[2], m[4]
		case tcRe.MatchString(raw):
			s.tc = tcRe.FindStringSubmatch(raw)[1]
		case gameWinRe.MatchString(raw):
			m := gameWinRe.FindStringSubmatch(raw)
			redName, blueName := seatNames(m[1], m[2], m[3], m[4])
			flush(names[redName], names[blueName], winnerOutcome(m[2]))
		case gameDrawRe.MatchString(raw):
			m := gameDrawRe.FindStringSubmatch(raw)
			flush(names[m[1]], names[m[2]], "draw")
		case verdictRe.MatchString(raw):
			m := verdictRe.FindStringSubmatch(raw)
			s.firstWins = atoiSafe(m[1])
			s.secondWins = atoiSafe(m[2])
		default:
			ml, err := parseMLine(raw)
			if err != nil {
				if raw != "" && !startsWith(raw, "run ") {
					s.rejects++
				}
				continue
			}
			pending = append(pending, struct {
				side string
				ml   mLine
			}{ml.side, ml})
		}
	}
	return s, nil
}

func seatNames(a, aColor, b, bColor string) (string, string) {
	if aColor == "red" {
		return a, b
	}
	return b, a
}

func winnerOutcome(winnerColor string) string {
	if winnerColor == "red" {
		return "red"
	}
	return "blue"
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func pvWordCount(pv string) int {
	if pv == "" {
		return 0
	}
	n := 1
	for i := 0; i < len(pv); i++ {
		if pv[i] == ' ' {
			n++
		}
	}
	return n
}

func splitLines(body []byte) []string {
	var out []string
	start := 0
	for i, b := range body {
		if b == '\n' {
			line := body[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			out = append(out, string(line))
			start = i + 1
		}
	}
	if start < len(body) {
		out = append(out, string(body[start:]))
	}
	return out
}

type logDirResult struct {
	tournament []moveRow
	firstWins  []firstWins
	outcomes   []outcomeRow
	rejects    int
	files      int
}

func mineLogDir(root string) (logDirResult, error) {
	var res logDirResult
	firstAgg := map[outcomeKey]int{}
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(d.Name()) != ".txt" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return res, fmt.Errorf("logmine: walk %s: %w", root, err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			return res, fmt.Errorf("logmine: read %s: %w", path, err)
		}
		s, err := parseSeriesLog(body)
		if err != nil {
			return res, err
		}
		res.files++
		base := filepath.Base(path)
		for i := range s.moves {
			s.moves[i].srcFile = base
		}
		res.tournament = append(res.tournament, s.moves...)
		res.rejects += s.rejects
		if s.firstTier != "" && s.secondTier != "" && (s.firstWins != 0 || s.secondWins != 0) {
			res.firstWins = append(res.firstWins, firstWins{
				tc: s.tc, firstTier: s.firstTier, secondTier: s.secondTier,
				firstWins: s.firstWins, secondWins: s.secondWins,
			})
		}
		for _, o := range s.outcomes {
			firstAgg[o.key] += o.count
		}
	}
	for k, v := range firstAgg {
		res.outcomes = append(res.outcomes, outcomeRow{key: k, count: v})
	}
	sort.Slice(res.outcomes, func(i, j int) bool {
		a, b := res.outcomes[i].key, res.outcomes[j].key
		if a.tc != b.tc {
			return a.tc < b.tc
		}
		if a.redTier != b.redTier {
			return a.redTier < b.redTier
		}
		if a.blueTier != b.blueTier {
			return a.blueTier < b.blueTier
		}
		return a.outcome < b.outcome
	})
	return res, nil
}
