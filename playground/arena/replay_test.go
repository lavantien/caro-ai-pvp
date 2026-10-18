package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const replayDirSuffix = "regate32-r1"

func requireTourneyRoots(t *testing.T) []string {
	t.Helper()
	roots := tourneyLogRoots()
	if len(roots) == 0 {
		t.Skipf("no %s directory discoverable from the test working directory or %s", config.TournamentLogRoot, tourneyLogsEnv)
	}
	return roots
}

func runDirsBySuffix(t *testing.T, roots []string, suffix string) []string {
	t.Helper()
	var dirs []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
				dirs = append(dirs, filepath.Join(root, e.Name()))
			}
		}
	}
	sort.Strings(dirs)
	return dirs
}

func seriesFiles(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "run*_s*.txt"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	if len(matches) == 0 {
		t.Fatalf("no series logs under %s", dir)
	}
	return matches
}

func TestReplayHandDerivedOpenings(t *testing.T) {
	roots := requireTourneyRoots(t)
	dirs := runDirsBySuffix(t, roots, "-"+replayDirSuffix)
	if len(dirs) == 0 {
		t.Skipf("no recorded %s run directory under %v on this host", replayDirSuffix, roots)
	}
	dir := dirs[len(dirs)-1]
	cases := []struct {
		file string
		want string
	}{
		{"run26_s492_master-1-vs-easy-1.txt", "0-1-0-0-0 over 1 pairs, 1 unpaired"},
		{"run26_s509_easy-1-vs-medium-1.txt", "0-0-0-1-0 over 1 pairs, 1 unpaired"},
		{"run26_s495_master-1-vs-master-2.txt", "0-0-0-0-1 over 1 pairs, 0 unpaired"},
		{"run26_s491_master-1-vs-grandmaster-1.txt", "0-0-1-0-0 over 1 pairs, 1 unpaired"},
		{"run26_s500_grandmaster-1-vs-easy-1.txt", "1-0-0-0-0 over 1 pairs, 0 unpaired"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			sl, err := parseSeriesLog(filepath.Join(dir, c.file))
			if err != nil {
				t.Fatalf("parseSeriesLog: %v", err)
			}
			if got := replaySeriesPtnml(sl).Summary(); got != c.want {
				t.Errorf("%s replay = %q, want %q", c.file, got, c.want)
			}
		})
	}
}

func TestReplayRunWideReconciliation(t *testing.T) {
	roots := requireTourneyRoots(t)
	dirs := runDirsBySuffix(t, roots, "-"+replayDirSuffix)
	if len(dirs) == 0 {
		t.Skipf("no recorded %s run directory under %v on this host", replayDirSuffix, roots)
	}
	dir := dirs[len(dirs)-1]
	if _, err := os.Stat(filepath.Join(dir, "summary.txt")); err != nil {
		t.Fatalf("%s carries no summary.txt: %v", dir, err)
	}
	for _, path := range seriesFiles(t, dir) {
		sl, err := parseSeriesLog(path)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(path), err)
			continue
		}
		wins, draws, losses := sl.winDrawLoss()
		if sl.hasVerdict {
			if wins != sl.firstWins || losses != sl.secondWins {
				t.Errorf("%s: parsed %d-%d against verdict %d-%d", filepath.Base(path), wins, losses, sl.firstWins, sl.secondWins)
			}
			want := "drawn"
			switch {
			case sl.firstWins > sl.secondWins:
				want = sl.redFirst
			case sl.secondWins > sl.firstWins:
				want = sl.blueFirst
			}
			if sl.verdictWinner != want {
				t.Errorf("%s: verdict winner %q, want %q", filepath.Base(path), sl.verdictWinner, want)
			}
		}
		pt := replaySeriesPtnml(sl)
		if pt.Games() != len(sl.games) {
			t.Errorf("%s: replay games %d against parsed %d", filepath.Base(path), pt.Games(), len(sl.games))
		}
		if pt.pairs != len(sl.games)/2 || pt.unpaired != len(sl.games)%2 {
			t.Errorf("%s: pairs %d unpaired %d over %d games", filepath.Base(path), pt.pairs, pt.unpaired, len(sl.games))
		}
		if want := 2*wins + draws; pt.HalfPoints() != want {
			t.Errorf("%s: half points %d against game-level %d", filepath.Base(path), pt.HalfPoints(), want)
		}
	}
}

func TestReplayNewestSettledRun(t *testing.T) {
	roots := requireTourneyRoots(t)
	var dirs []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, e.Name(), "summary.txt")); err != nil {
				continue
			}
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	if len(dirs) == 0 {
		t.Skipf("no settled run directory under %v on this host", roots)
	}
	sort.Strings(dirs)
	dir := dirs[len(dirs)-1]
	settled := 0
	for _, path := range seriesFiles(t, dir) {
		sl, err := parseSeriesLog(path)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(path), err)
			continue
		}
		if sl.hasVerdict {
			settled++
		}
		pt := replaySeriesPtnml(sl)
		if pt.pairs*2+pt.unpaired != len(sl.games) {
			t.Errorf("%s: pair accounting broken: %s", filepath.Base(path), pt.Summary())
		}
	}
	if settled == 0 {
		t.Errorf("%s: no settled series found", dir)
	}
}
