package main

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// renderReport writes the markdown evidence report; every list arrives
// sorted, so the same logs render identical bytes.
func renderReport(w io.Writer, a *analysis) {
	fmt.Fprintf(w, "# logstats report\n\n")
	fmt.Fprintf(w, "- dir: %s\n- files: %d\n- runs: %d (%s)\n- series: %d\n- games: %d\n- m-lines: %d\n\n",
		a.dir, a.files, len(a.runIDs), joinInt64(a.runIDs), a.series, a.games, a.mlines)

	fmt.Fprintf(w, "## participants\n\n")
	fmt.Fprintf(w, "| participant | tier | series | series wins | wins | losses | draws |\n")
	fmt.Fprintf(w, "| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, p := range a.participants {
		fmt.Fprintf(w, "| %s | %s | %d | %d | %d | %d | %d |\n",
			p.name, p.tier, p.seriesPlayed, p.seriesWins, p.gameWins, p.gameLosses, p.gameDraws)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "## telemetry per tier\n\n")
	for i := range a.telemetry {
		t := &a.telemetry[i]
		fmt.Fprintf(w, "### tier %s\n\n", t.tier)
		fmt.Fprintf(w, "- moves: %d\n", t.moves)
		fmt.Fprintf(w, "- depth: mean %.1f, median %.1f, max %d\n",
			meanInt(t.depths), medianInt(t.depths), maxOf(t.depths))
		fmt.Fprintf(w, "- mean n: %s, mean nps: %s\n",
			displayCompact(meanU64(t.nodes)), displayCompact(meanU64(t.nps)))
		fmt.Fprintf(w, "- mean t: %.2f s, mean alloc: %.2f s, mean alloc/t: %s\n",
			meanF(t.ts), meanF(t.allocs), displayUtil(t))
		fmt.Fprintf(w, "- mean tt: %.1f%%\n", meanInt(t.tts))
		// The score mean excludes mate-band lines: their mega-magnitudes
		// would drown the milliunit leaf scores the mean exists to show.
		nonMate := nonMateScores(t.scores)
		fmt.Fprintf(w, "- score: min %s, max %s, mean %.1f over %d non-mate lines\n",
			displayScore(minOf(t.scores)), displayScore(maxOf(t.scores)),
			meanInt(nonMate), len(nonMate))
		fmt.Fprintf(w, "- mate markers: +M %d, -M %d\n", t.matePos, t.mateNeg)
		fmt.Fprintf(w, "- anomalies: d=1 non-first %d, zero nps %d, dup consecutive %d\n\n",
			t.anomD1, t.anomZeroNps, t.anomDup)
	}

	fmt.Fprintf(w, "## strength verdict\n\n")
	for i, p := range a.participants {
		fmt.Fprintf(w, "%d. %s (%s): series wins %d, game wins %d\n",
			i+1, p.name, p.tier, p.seriesWins, p.gameWins)
	}
	fmt.Fprintln(w)
	if len(a.inversions) == 0 {
		fmt.Fprintf(w, "inversions: none\n")
	} else {
		fmt.Fprintf(w, "inversions:\n")
		for _, inv := range a.inversions {
			fmt.Fprintf(w, "- %s\n", inv)
		}
	}
	fmt.Fprintf(w, "zero-sum: %s\n\n", a.zeroSum)

	fmt.Fprintf(w, "## parse integrity\n\n")
	fmt.Fprintf(w, "- unparsable lines: %d\n", len(a.bad))
	for _, b := range a.bad {
		fmt.Fprintf(w, "  - %s:%d: %q\n", b.path, b.lineno, b.text)
	}
	fmt.Fprintf(w, "- unattributed m-lines (no pairing header): %d\n", a.unattributedM)
	fmt.Fprintf(w, "- files without a full header: %d\n", len(a.noHeader))
	for _, p := range a.noHeader {
		fmt.Fprintf(w, "  - %s\n", p)
	}
	fmt.Fprintf(w, "- unfinished series (no verdict): %d\n", len(a.unfinished))
	for _, p := range a.unfinished {
		fmt.Fprintf(w, "  - %s\n", p)
	}
	fmt.Fprintf(w, "- fold mismatches: %d\n", len(a.foldMismatch))
	for _, m := range a.foldMismatch {
		fmt.Fprintf(w, "  - %s\n", m)
	}
}

func joinInt64(s []int64) string {
	parts := make([]string, len(s))
	for i, v := range s {
		parts[i] = strconv.FormatInt(v, 10)
	}
	return strings.Join(parts, ", ")
}

func minOf(s []int) int {
	if len(s) == 0 {
		return 0
	}
	return slices.Min(s)
}

func maxOf(s []int) int {
	if len(s) == 0 {
		return 0
	}
	return slices.Max(s)
}

// displayUtil renders the mean granted-budget utilization alloc/t; lines
// with zero elapsed time carry no ratio.
func displayUtil(t *tierTelemetry) string {
	if t.utilN == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", t.utilSum/float64(t.utilN))
}

// displayCompact renders a mean count on the emitter's compact scales
// (compactK, compactM in parse.go), the way the M-lines themselves write
// counts.
func displayCompact(f float64) string {
	switch {
	case f >= compactM:
		return trimZero(f/compactM) + "m"
	case f >= compactK:
		return trimZero(f/compactK) + "k"
	}
	return strconv.FormatFloat(math.Round(f), 'f', 0, 64)
}

func trimZero(f float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(f, 'f', 2, 64), "0"), ".")
}

// displayScore renders a parsed score back on the emitter's law
// (internal/server/stats.go appendScore): the mate band as its M-distance,
// everything inside as the signed milliunit integer.
func displayScore(v int) string {
	switch {
	case v >= mateFloor:
		return fmt.Sprintf("M%d", (config.EvalMateMax-v)/config.EvalMateScoreStep)
	case v <= -mateFloor:
		return fmt.Sprintf("-M%d", (config.EvalMateMax+v)/config.EvalMateScoreStep)
	}
	return strconv.Itoa(v)
}
