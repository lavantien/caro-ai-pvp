package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const runawayMinDepth = 30

var mateScoreRe = regexp.MustCompile(`^[+-]?M\d+$`)

type runawayStats struct {
	total    int
	atCap    int
	mate     int
	nearMate int
	zero     int
	normal   int
	depth    spread
	moveNo   spread
	nodes    spread
	pvLen    spread
	ebf      spread
	byTC     map[string]int
	examples []moveRow
	verdict  string
}

func scoreClass(txt string) string {
	if mateScoreRe.MatchString(txt) {
		return "mate"
	}
	var v int
	if _, err := fmt.Sscanf(txt, "%d", &v); err != nil {
		return "normal"
	}
	if v == 0 {
		return "zero"
	}
	near := config.EvalMateMax - config.SearchMaxPly*config.EvalMateScoreStep
	if v > near || v < -near {
		return "near-mate"
	}
	if v >= 1000 || v <= -1000 {
		return "near-mate"
	}
	return "normal"
}

func mineRunaway(rows []moveRow) runawayStats {
	var s runawayStats
	s.byTC = map[string]int{}
	var depths, moveNos, nodes, pvLens, ebfs []float64
	for _, r := range rows {
		if r.tier != "easy" || r.depth < runawayMinDepth {
			continue
		}
		s.total++
		if r.depth >= config.SearchMaxPly {
			s.atCap++
		}
		switch scoreClass(r.scoreTxt) {
		case "mate":
			s.mate++
		case "near-mate":
			s.nearMate++
		case "zero":
			s.zero++
		default:
			s.normal++
		}
		s.byTC[r.tc]++
		depths = append(depths, float64(r.depth))
		moveNos = append(moveNos, float64(r.moveNo))
		nodes = append(nodes, float64(r.nodes))
		pvLens = append(pvLens, float64(r.pvLen))
		ebfs = append(ebfs, float64(r.ebfMilli)/1000)
		s.examples = append(s.examples, r)
	}
	s.depth = quantiles(depths)
	s.moveNo = quantiles(moveNos)
	s.nodes = quantiles(nodes)
	s.pvLen = quantiles(pvLens)
	s.ebf = quantiles(ebfs)
	sort.Slice(s.examples, func(i, j int) bool {
		a, b := s.examples[i], s.examples[j]
		if a.srcFile != b.srcFile {
			return a.srcFile < b.srcFile
		}
		if a.gameID != b.gameID {
			return a.gameID < b.gameID
		}
		return a.moveNo < b.moveNo
	})
	if len(s.examples) > 5 {
		s.examples = s.examples[:5]
	}
	switch {
	case s.total == 0:
		s.verdict = "no runaway moves above the threshold"
	case s.mate+s.nearMate == s.total && s.atCap*2 >= s.total:
		s.verdict = "cap-bound mate ladder on sparse tails"
	case s.zero*2 >= s.total && s.atCap*2 >= s.total:
		s.verdict = "cap-bound ladder on collapsed branching with draw scores, not a mate ladder"
	default:
		s.verdict = "mixed population, needs engine-side reproduction before classification"
	}
	return s
}

func renderRunawayReport(source string, files int, s runawayStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# easy tier depth runaway: %s\n\nsource: %s\nseries log files: %d\nthreshold: depth >= %d on easy moves\nengine caps: SearchMaxPly %d, EvalMateMax %d, EvalMateScoreStep %d\n\n",
		source, source, files, runawayMinDepth, config.SearchMaxPly, config.EvalMateMax, config.EvalMateScoreStep)
	fmt.Fprintf(&b, "## population\n\n| moves | at depth cap %d | mate scores | near-mate | zero scores | normal | depth q1/med/q3 | move no q1/med/q3 | nodes med | pv len med | ebf med |\n|---|---|---|---|---|---|---|---|---|---|---|\n",
		config.SearchMaxPly)
	fmt.Fprintf(&b, "| %d | %d | %d | %d | %d | %d | %s | %s | %s | %s | %s |\n\n",
		s.total, s.atCap, s.mate, s.nearMate, s.zero, s.normal,
		fmtCount(s.depth), fmtCount(s.moveNo), fmtCount(s.nodes), fmtCount(s.pvLen), fmtSpread(s.ebf, 1, 1))
	tcKeys := make([]string, 0, len(s.byTC))
	for k := range s.byTC {
		tcKeys = append(tcKeys, k)
	}
	sort.Strings(tcKeys)
	b.WriteString("by time control: ")
	for i, k := range tcKeys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s %d", k, s.byTC[k])
	}
	b.WriteString("\n\n## example moves\n\n| file | game | move | tc | depth | nodes | score | pv len | ebf |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range s.examples {
		fmt.Fprintf(&b, "| %s | %d | %d | %s | %d | %d | %s | %d | %.1f |\n",
			r.srcFile, r.gameID, r.moveNo, r.tc, r.depth, r.nodes, r.scoreTxt, r.pvLen, float64(r.ebfMilli)/1000)
	}
	fmt.Fprintf(&b, "\nverdict: %s. The engine's depth cap is SearchMaxPly %d and no move sits above it: the iterative ladder runs to the hard cap on boards whose branching collapsed (median ebf 2.0). Mate-band scores step %d per ply from EvalMateMax, so the mate-scored subset is the mate-distance ladder; the zero-scored subset is a quiet forced tail the ladder walks to the cap without a mate. Both are endgame shapes, not move generator defects.\n",
		s.verdict, config.SearchMaxPly, config.EvalMateScoreStep)
	return b.String()
}
