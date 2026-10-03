package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// The aggregation laws, all mirroring the conductor of
// internal/tourney/runner.go: game 1 seats the pairing's first name on red
// and the benchmark alternation passes red to the other participant after
// every game, draws included; the verdict line names the winning
// participant (or "drawn") and its billed wins must match the fold.

// participantStats is one participant's folded record across every series.
type participantStats struct {
	name, tier   string
	tierRank     int
	seriesPlayed int
	seriesWins   int
	gameWins     int
	gameLosses   int
	gameDraws    int
}

// tierTelemetry aggregates the M-lines attributed to one tier's seats.
type tierTelemetry struct {
	tier  string
	rank  int
	moves int
	// Sample slices keep the medians honest; means come off them too.
	depths  []int
	nodes   []uint64
	nps     []uint64
	ts      []float64
	allocs  []float64
	tts     []int
	scores  []int
	matePos int
	mateNeg int
	// Anomalies: depth 1 on a non-first move, zero node rate, and two
	// consecutive M-lines repeating the same cell and score.
	anomD1      int
	anomZeroNps int
	anomDup     int
	utilSum     float64
	utilN       int
}

// analysis is the whole report's data, every list pre-sorted for
// deterministic rendering.
type analysis struct {
	dir           string
	files         int
	series        int
	runIDs        []int64
	games         int
	mlines        int
	unattributedM int
	participants  []participantStats
	telemetry     []tierTelemetry
	inversions    []string
	zeroSum       string
	bad           []badLine
	noHeader      []string
	unfinished    []string
	foldMismatch  []string
}

// analyze folds every parsed file into the report's data. Files without a
// full header still count their games and M-lines in the inventory but
// attribute nothing; the integrity lists carry them.
func analyze(dir string, files []seriesFile) *analysis {
	a := &analysis{dir: dir, files: len(files)}
	parts := map[string]*participantStats{}
	tiers := map[string]*tierTelemetry{}
	seenRuns := map[int64]bool{}
	seenSeries := map[[2]int64]bool{}

	part := func(name, tier string) *participantStats {
		if p, ok := parts[name]; ok {
			return p
		}
		p := &participantStats{name: name, tier: tier, tierRank: tierRank(tier)}
		parts[name] = p
		return p
	}
	telem := func(tier string) *tierTelemetry {
		if t, ok := tiers[tier]; ok {
			return t
		}
		t := &tierTelemetry{tier: tier, rank: tierRank(tier)}
		tiers[tier] = t
		return t
	}

	for _, f := range files {
		a.bad = append(a.bad, f.bad...)
		var pf, ps *participantStats
		if f.hasHead {
			pf = part(f.head.redName, f.head.redTier)
			ps = part(f.head.blueName, f.head.blueTier)
			pf.seriesPlayed++
			ps.seriesPlayed++
			if !seenRuns[f.head.run] {
				seenRuns[f.head.run] = true
				a.runIDs = append(a.runIDs, f.head.run)
			}
			key := [2]int64{f.head.run, f.head.series}
			if !seenSeries[key] {
				seenSeries[key] = true
				a.series++
			}
		} else {
			a.noHeader = append(a.noHeader, f.path)
			a.series++
		}

		redIsFirst := true
		firstWins, secondWins := 0, 0
		prevKey := ""
		for _, r := range f.records {
			if r.ml != nil {
				a.mlines++
				if !f.hasHead {
					a.unattributedM++
					continue
				}
				redSeat, blueSeat := pf, ps
				if !redIsFirst {
					redSeat, blueSeat = ps, pf
				}
				mover := redSeat
				if r.ml.side != "Red" {
					mover = blueSeat
				}
				t := telem(mover.tier)
				t.moves++
				t.depths = append(t.depths, r.ml.depth)
				t.nodes = append(t.nodes, r.ml.nodes)
				t.nps = append(t.nps, r.ml.nps)
				t.ts = append(t.ts, r.ml.t)
				t.allocs = append(t.allocs, r.ml.alloc)
				t.tts = append(t.tts, r.ml.tt)
				t.scores = append(t.scores, r.ml.score)
				if r.ml.score >= mateFloor {
					t.matePos++
				} else if r.ml.score <= -mateFloor {
					t.mateNeg++
				}
				if r.ml.depth == 1 && r.ml.move > 1 {
					t.anomD1++
				}
				if r.ml.nps == 0 {
					t.anomZeroNps++
				}
				if r.ml.t > 0 {
					t.utilSum += r.ml.alloc / r.ml.t
					t.utilN++
				}
				key := r.ml.cell + "|" + strconv.Itoa(r.ml.score)
				if key == prevKey {
					t.anomDup++
				}
				prevKey = key
				continue
			}
			a.games++
			if !f.hasHead {
				continue
			}
			redSeat, blueSeat := pf, ps
			if !redIsFirst {
				redSeat, blueSeat = ps, pf
			}
			switch r.gl.outcome {
			case server.OutcomeRed:
				redSeat.gameWins++
				blueSeat.gameLosses++
				if redSeat == pf {
					firstWins++
				} else {
					secondWins++
				}
			case server.OutcomeBlue:
				blueSeat.gameWins++
				redSeat.gameLosses++
				if blueSeat == pf {
					firstWins++
				} else {
					secondWins++
				}
			default:
				redSeat.gameDraws++
				blueSeat.gameDraws++
			}
			redIsFirst = !redIsFirst
		}

		if f.verdict == nil {
			if f.hasHead {
				a.unfinished = append(a.unfinished, f.path)
			}
			continue
		}
		if !f.hasHead {
			continue
		}
		switch f.verdict.winner {
		case f.head.redName:
			pf.seriesWins++
		case f.head.blueName:
			ps.seriesWins++
		}
		want := "drawn"
		switch {
		case firstWins > secondWins:
			want = f.head.redName
		case secondWins > firstWins:
			want = f.head.blueName
		}
		if f.verdict.firstWins != firstWins || f.verdict.secondWins != secondWins || f.verdict.winner != want {
			a.foldMismatch = append(a.foldMismatch, fmt.Sprintf(
				"%s: folded %d-%d (%s) against verdict %s %d-%d",
				f.path, firstWins, secondWins, want,
				f.verdict.winner, f.verdict.firstWins, f.verdict.secondWins))
		}
	}

	for name := range parts {
		a.participants = append(a.participants, *parts[name])
	}
	slices.SortFunc(a.participants, func(x, y participantStats) int {
		if x.seriesWins != y.seriesWins {
			return y.seriesWins - x.seriesWins
		}
		if x.gameWins != y.gameWins {
			return y.gameWins - x.gameWins
		}
		return strings.Compare(x.name, y.name)
	})

	// Telemetry in ladder order, unknown tiers after, by name.
	for i := range config.Tiers {
		if t, ok := tiers[config.Tiers[i].Name]; ok {
			a.telemetry = append(a.telemetry, *t)
		}
	}
	var unknown []string
	for name := range tiers {
		if tierRank(name) < 0 {
			unknown = append(unknown, name)
		}
	}
	slices.Sort(unknown)
	for _, name := range unknown {
		a.telemetry = append(a.telemetry, *tiers[name])
	}

	// Inversions: a weaker tier's participant ranked above a stronger one's.
	for i := range a.participants {
		for j := i + 1; j < len(a.participants); j++ {
			pi, pj := a.participants[i], a.participants[j]
			if pi.tierRank >= 0 && pj.tierRank >= 0 && pi.tierRank < pj.tierRank {
				a.inversions = append(a.inversions, fmt.Sprintf(
					"INVERSION: %s (tier %s) ranks %d above %s (tier %s)",
					pi.name, pi.tier, i+1, pj.name, pj.tier))
			}
		}
	}

	wins, losses, draws := 0, 0, 0
	for _, p := range a.participants {
		wins += p.gameWins
		losses += p.gameLosses
		draws += p.gameDraws
	}
	verdict := "zero-sum holds"
	if wins != losses || draws%2 != 0 {
		verdict = "ZERO-SUM BROKEN"
	}
	a.zeroSum = fmt.Sprintf("wins %d, losses %d, draws %d: %s", wins, losses, draws, verdict)

	slices.Sort(a.runIDs)
	return a
}

// tierRank maps a tier name onto the config.Tiers strength ladder index
// (easy < medium < hard); unknown names rank -1 and sit outside every
// inversion check.
func tierRank(name string) int {
	for i := range config.Tiers {
		if config.Tiers[i].Name == name {
			return i
		}
	}
	return -1
}

// mean and median helpers over the telemetry samples; empty slices report
// zero so the renderer never divides by nothing.
func meanInt(s []int) float64 {
	if len(s) == 0 {
		return 0
	}
	sum := 0
	for _, v := range s {
		sum += v
	}
	return float64(sum) / float64(len(s))
}

func meanU64(s []uint64) float64 {
	if len(s) == 0 {
		return 0
	}
	var sum float64
	for _, v := range s {
		sum += float64(v)
	}
	return sum / float64(len(s))
}

func meanF(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	var sum float64
	for _, v := range s {
		sum += v
	}
	return sum / float64(len(s))
}

// nonMateScores filters the mate band out of a score sample: those lines
// are mate distances on the mateWin lattice, not milliunit leaf scores.
func nonMateScores(s []int) []int {
	out := make([]int, 0, len(s))
	for _, v := range s {
		if v > -mateFloor && v < mateFloor {
			out = append(out, v)
		}
	}
	return out
}

func medianInt(s []int) float64 {
	if len(s) == 0 {
		return 0
	}
	c := slices.Clone(s)
	slices.Sort(c)
	if len(c)%2 == 1 {
		return float64(c[len(c)/2])
	}
	return float64(c[len(c)/2-1]+c[len(c)/2]) / 2
}
