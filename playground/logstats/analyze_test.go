package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// writeSyntheticLogs writes raw log files into a fresh dir, the shapes no
// real writer emits (corruption, missing verdicts) included.
func writeSyntheticLogs(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// analyzeDir runs the tool's whole read-parse-analyze pipeline over dir.
func analyzeDir(t *testing.T, dir string) *analysis {
	t.Helper()
	paths, err := readSeriesLogs(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	files := make([]seriesFile, 0, len(paths))
	for _, p := range paths {
		f, err := parseFile(p)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		files = append(files, f)
	}
	return analyze(dir, files)
}

// headerBlock spells the three header lines the tourney writer emits, with
// the first listed time control and a bo3.
func headerBlock(run, series int, redName, redTier, blueName, blueTier string) string {
	tc := config.TimeControls[0]
	return fmt.Sprintf("run %d series %d\npairing %s (%s) vs %s (%s)\ntc %d+%d bo3\n",
		run, series, redName, redTier, blueName, blueTier, tc.InitialMin, tc.IncrementSec)
}

// mlineOf builds one M-line through the real emitter, its shape trimmed to
// the fields the telemetry section reads.
func mlineOf(t *testing.T, num int, side rules.Color, cell string, depth, score int, nps uint64) string {
	t.Helper()
	st := engine.SearchStats{
		Depth: depth, Nodes: 1000, Nps: nps, EBFMilli: 1200,
		TTHitPermille: 100, HashFullPermille: 200, FirstMoveFailHighPermille: 300,
		Score: score, Threads: 1, ElapsedNs: 500_000_000, AllocNs: 1_000_000_000,
	}
	st.PV[0] = mustMove(t, cell)
	st.PVLen = 1
	return server.MLine(num, side, mustMove(t, cell), &st, "")
}

func findParticipant(t *testing.T, a *analysis, name string) participantStats {
	t.Helper()
	for _, p := range a.participants {
		if p.name == name {
			return p
		}
	}
	t.Fatalf("participant %q missing from %v", name, a.participants)
	return participantStats{}
}

func findTelemetry(t *testing.T, a *analysis, tier string) *tierTelemetry {
	t.Helper()
	for i := range a.telemetry {
		if a.telemetry[i].tier == tier {
			return &a.telemetry[i]
		}
	}
	t.Fatalf("tier %q missing from telemetry %v", tier, a.telemetry)
	return nil
}

// TestSeatFoldBo3WithDraw walks the fold law of internal/tourney/runner.go
// through a bo3 with a draw: red alternates after every game, so alpha
// holds red in games 1 and 3, beta in game 2, and the series settles drawn
// at 1-1.
func TestSeatFoldBo3WithDraw(t *testing.T) {
	body := headerBlock(1, 0, "alpha", "easy", "beta", "hard") +
		"game 1: alpha (red) beat beta (blue), 9 moves, won by 4\n" +
		"game 2: beta (red) beat alpha (blue), 7 moves, won by open 4\n" +
		"game 3: alpha (red) vs beta (blue), drawn at 40 moves\n" +
		"series drawn 1-1\n"
	dir := writeSyntheticLogs(t, map[string]string{"run1_s0_alpha-vs-beta.txt": body})
	a := analyzeDir(t, dir)
	alpha := findParticipant(t, a, "alpha")
	if alpha.seriesPlayed != 1 || alpha.seriesWins != 0 ||
		alpha.gameWins != 1 || alpha.gameLosses != 1 || alpha.gameDraws != 1 {
		t.Errorf("alpha fold = %+v", alpha)
	}
	beta := findParticipant(t, a, "beta")
	if beta.seriesPlayed != 1 || beta.seriesWins != 0 ||
		beta.gameWins != 1 || beta.gameLosses != 1 || beta.gameDraws != 1 {
		t.Errorf("beta fold = %+v", beta)
	}
	if a.games != 3 || len(a.foldMismatch) != 0 || len(a.unfinished) != 0 {
		t.Errorf("integrity: games %d, foldMismatch %v, unfinished %v", a.games, a.foldMismatch, a.unfinished)
	}
}

// TestAnalyzeFullReport drives the whole pipeline over two series logs: a
// finished medium-vs-easy series with telemetry anomalies and a corrupt
// line, and an unfinished hard-vs-hard series, then checks the inventory,
// the fold, the inversion verdicts, the integrity ledgers, and that the
// rendered markdown is complete and deterministic.
func TestAnalyzeFullReport(t *testing.T) {
	finished := headerBlock(1, 0, "gamma", "medium", "delta", "easy") +
		mlineOf(t, 1, rules.Red, "C3", 4, 10, 1000) + "\n" +
		mlineOf(t, 2, rules.Blue, "D4", 2, -10, 1000) + "\n" +
		mlineOf(t, 3, rules.Red, "E5", 1, 7, 1000) + "\n" + // d=1 on a non-first move
		mlineOf(t, 4, rules.Blue, "F6", 2, -7, 0) + "\n" + // zero nps
		mlineOf(t, 5, rules.Red, "E5", 3, 7, 1000) + "\n" + // dup pair with M6
		mlineOf(t, 6, rules.Blue, "E5", 3, 7, 1000) + "\n" +
		"game 1: gamma (red) beat delta (blue), 6 moves, won by 4\n" +
		"M9, Red, corrupted\n" +
		mlineOf(t, 7, rules.Blue, "G7", 5, 20, 1000) + "\n" +
		"game 2: gamma (blue) beat delta (red), 5 moves\n" +
		"series gamma 2-0\n"
	unfinished := headerBlock(1, 1, "epsilon", "hard", "zeta", "hard") +
		mlineOf(t, 8, rules.Blue, "B2", 6, 30, 1000) + "\n" +
		"game 1: zeta (blue) beat epsilon (red), 5 moves\n" +
		"\n"
	dir := writeSyntheticLogs(t, map[string]string{
		"run1_s0_gamma-vs-delta.txt":  finished,
		"run1_s1_epsilon-vs-zeta.txt": unfinished,
	})
	a := analyzeDir(t, dir)

	if a.files != 2 || a.series != 2 || a.games != 3 || a.mlines != 8 {
		t.Errorf("inventory = files %d series %d games %d mlines %d", a.files, a.series, a.games, a.mlines)
	}
	if len(a.runIDs) != 1 || a.runIDs[0] != 1 {
		t.Errorf("runIDs = %v", a.runIDs)
	}

	gamma := findParticipant(t, a, "gamma")
	if gamma.seriesPlayed != 1 || gamma.seriesWins != 1 || gamma.gameWins != 2 || gamma.gameLosses != 0 {
		t.Errorf("gamma = %+v", gamma)
	}
	delta := findParticipant(t, a, "delta")
	if delta.gameWins != 0 || delta.gameLosses != 2 {
		t.Errorf("delta = %+v", delta)
	}
	zeta := findParticipant(t, a, "zeta")
	if zeta.gameWins != 1 || zeta.seriesWins != 0 {
		t.Errorf("zeta = %+v", zeta)
	}

	medium := findTelemetry(t, a, "medium")
	if medium.moves != 4 || medium.anomD1 != 1 || medium.anomZeroNps != 0 || medium.anomDup != 0 {
		t.Errorf("medium telemetry = %+v", medium)
	}
	easy := findTelemetry(t, a, "easy")
	if easy.moves != 3 || easy.anomZeroNps != 1 || easy.anomDup != 1 {
		t.Errorf("easy telemetry = %+v", easy)
	}
	hard := findTelemetry(t, a, "hard")
	if hard.moves != 1 {
		t.Errorf("hard telemetry = %+v", hard)
	}

	if len(a.inversions) != 3 {
		t.Errorf("inversions = %v, want gamma over zeta, gamma over epsilon, delta over epsilon", a.inversions)
	} else {
		want := fmt.Sprintf("INVERSION: %s (tier %s) ranks %d above %s (tier %s)", "gamma", "medium", 1, "zeta", "hard")
		if a.inversions[0] != want {
			t.Errorf("inversions[0] = %q, want %q", a.inversions[0], want)
		}
	}
	if !strings.Contains(a.zeroSum, "zero-sum holds") {
		t.Errorf("zeroSum = %q", a.zeroSum)
	}

	if len(a.bad) != 2 {
		t.Fatalf("bad = %v, want the corrupt M-line and the blank line", a.bad)
	}
	if a.bad[0].lineno != 11 || a.bad[0].text != "M9, Red, corrupted" {
		t.Errorf("bad[0] = %s:%d %q", a.bad[0].path, a.bad[0].lineno, a.bad[0].text)
	}
	if filepath.Base(a.bad[0].path) != "run1_s0_gamma-vs-delta.txt" {
		t.Errorf("bad[0] path = %s", a.bad[0].path)
	}
	if a.bad[1].lineno != 6 || a.bad[1].text != "" {
		t.Errorf("bad[1] = %s:%d %q, want the blank line 6", a.bad[1].path, a.bad[1].lineno, a.bad[1].text)
	}
	if len(a.unfinished) != 1 || filepath.Base(a.unfinished[0]) != "run1_s1_epsilon-vs-zeta.txt" {
		t.Errorf("unfinished = %v", a.unfinished)
	}
	if len(a.noHeader) != 0 || len(a.foldMismatch) != 0 {
		t.Errorf("noHeader %v foldMismatch %v, want none", a.noHeader, a.foldMismatch)
	}

	var first, second strings.Builder
	renderReport(&first, a)
	renderReport(&second, a)
	if first.String() != second.String() {
		t.Errorf("rendering not deterministic")
	}
	for _, want := range []string{
		"- m-lines: 8",
		"### tier medium",
		"- anomalies: d=1 non-first 1, zero nps 0, dup consecutive 0",
		"- unparsable lines: 2",
		"- unfinished series (no verdict): 1",
		"zero-sum: wins 3, losses 3, draws 0: zero-sum holds",
	} {
		if !strings.Contains(first.String(), want) {
			t.Errorf("report missing %q\n%s", want, first.String())
		}
	}
}

// TestAnalyzeEmptyDir renders the zero report over a dir with no logs.
func TestAnalyzeEmptyDir(t *testing.T) {
	a := analyzeDir(t, writeSyntheticLogs(t, map[string]string{}))
	if a.files != 0 || a.games != 0 || a.mlines != 0 || len(a.participants) != 0 {
		t.Errorf("empty analysis = %+v", a)
	}
	var report strings.Builder
	renderReport(&report, a)
	if !strings.Contains(report.String(), "- files: 0") || !strings.Contains(report.String(), "- unparsable lines: 0") {
		t.Errorf("empty report =\n%s", report.String())
	}
}

// TestAnalyzeHeaderlessFile checks the never-dropped rule at file scale: a
// log with data lines but no header still counts its inventory and lands in
// the missing-header ledger without attributing anything.
func TestAnalyzeHeaderlessFile(t *testing.T) {
	body := mlineOf(t, 1, rules.Red, "A1", 2, 5, 1000) + "\ngame 1: a (red) beat b (blue), 1 moves\n"
	a := analyzeDir(t, writeSyntheticLogs(t, map[string]string{"stray.txt": body}))
	if a.files != 1 || a.mlines != 1 || a.games != 1 || a.unattributedM != 1 {
		t.Errorf("inventory = %+v", a)
	}
	if len(a.noHeader) != 1 || len(a.participants) != 0 || len(a.telemetry) != 0 {
		t.Errorf("attribution = noHeader %v participants %v telemetry %v", a.noHeader, a.participants, a.telemetry)
	}
}

// TestAnalyzeFoldMismatchDetected flags a verdict that disagrees with the
// billed games.
func TestAnalyzeFoldMismatchDetected(t *testing.T) {
	body := headerBlock(1, 0, "a", "easy", "b", "hard") +
		"game 1: a (red) beat b (blue), 9 moves, won by 4\n" +
		"series a 2-0\n"
	a := analyzeDir(t, writeSyntheticLogs(t, map[string]string{"run1_s0_a-vs-b.txt": body}))
	if len(a.foldMismatch) != 1 {
		t.Errorf("foldMismatch = %v, want the folded 1-0 against the billed 2-0", a.foldMismatch)
	}
	if len(a.unfinished) != 0 {
		t.Errorf("unfinished = %v", a.unfinished)
	}
}

// TestReadSeriesLogsMissingDir errors instead of reporting an empty run.
func TestReadSeriesLogsMissingDir(t *testing.T) {
	if _, err := readSeriesLogs(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Errorf("readSeriesLogs over a missing dir succeeded")
	}
}
