package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

func main() {
	corpusFlag := flag.String("corpus", "", "path to bookgen corpus file")
	flag.Parse()
	path, err := resolveCorpus(*corpusFlag)
	if err != nil {
		fmt.Println("corpus:", err)
		os.Exit(1)
	}
	games, err := loadCorpus(path)
	if err != nil {
		fmt.Println("load:", err)
		os.Exit(1)
	}
	positions := pickPositions(games, positionMin)
	tactical := tacticalPositions(games, tacticalWant)
	if len(positions) < positionMin {
		fmt.Printf("positions: got %d, want %d\n", len(positions), positionMin)
		os.Exit(1)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Println("outdir:", err)
		os.Exit(1)
	}
	fmt.Printf("corpus %s games %d positions %d tactical %d\n", filepath.Base(path), len(games), len(positions), len(tactical))
	runExperiment1(tactical)
	runExperiment2(games)
	runExperiment3(positions)
	runExperiment4(positions)
}

func resolveCorpus(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	var found []string
	for _, base := range []string{"", "../../", "../../../"} {
		c := base + defaultCorpusRel
		if _, err := os.Stat(c); err == nil {
			found = append(found, c)
		}
	}
	for _, extra := range []string{"inst2/book-corpus-b.txt", "inst3/book-corpus-b.txt"} {
		for _, base := range []string{"", "../../", "../../../"} {
			c := base + "playground/bookgen/out/" + extra
			if _, err := os.Stat(c); err == nil {
				found = append(found, c)
				break
			}
		}
	}
	if len(found) == 0 {
		return "", fmt.Errorf("no corpus file found, pass -corpus")
	}
	return strings.Join(found, ","), nil
}

func runExperiment1(tactical []benchPos) {
	shipped := vcf.New(vcf.KindVCF)
	s := newSolver()
	type runSpec struct {
		mode  orderMode
		limit uint8
		name  string
	}
	runs := []runSpec{
		{orderShipped, 0, "mirror"},
		{orderWaveTie, 0, "waveTie"},
		{orderWaveFirst, 0, "waveFirst"},
		{orderPrune, 2, "prune2"},
		{orderPrune, 3, "prune3"},
		{orderPrune, 4, "prune4"},
	}
	totals := map[string]int{}
	flips := map[string]int{}
	refNodes := 0
	twoPhaseNodes := 0
	withSources := 0
	var out vcf.SolverStats
	f, err := os.Create(filepath.Join(outDir, "exp1-wavefront.md"))
	if err != nil {
		fmt.Println("exp1 out:", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "# experiment 1 wavefront ordering vs shipped vcf\n\nbudget %d nodes, %d tactical positions\n\n| position | ref found nodes | sources | mirror | waveTie | waveFirst | prune2 | prune3 | prune4 |\n|---|---|---|---|---|---|---|---|---|\n", solveBudget, len(tactical))
	for _, p := range tactical {
		want := shipped.Solve(p.board, solveBudget, nil, &out)
		refNodes += int(out.Nodes)
		wave := wavefront(p.board, p.board.Side)
		sources := 0
		for _, d := range wave {
			if d == 0 {
				sources++
			}
		}
		if sources > 0 {
			withSources++
		}
		row := fmt.Sprintf("| g%d p%d | %v %d | %d", p.game, p.ply, out.Found, out.Nodes, sources)
		mirrorNodes := 0
		for _, r := range runs {
			found, nodes, _ := s.solve(p.board, solveBudget, r.mode, r.limit, wave)
			totals[r.name] += nodes
			if found != want {
				flips[r.name]++
			}
			if r.name == "mirror" {
				mirrorNodes = nodes
			}
			row += fmt.Sprintf(" | %v %d", found, nodes)
			if r.name == "prune3" {
				twoPhaseNodes += nodes
				if !found {
					mf, mn, _ := s.solve(p.board, solveBudget, orderShipped, 0, wave)
					twoPhaseNodes += mn
					if !mf && want {
						flips["twoPhase"]++
					}
				}
			}
		}
		rf, rn, _ := s.solve(p.board, solveBudget, orderShipped, 0, wave)
		if rf != want || rn != mirrorNodes {
			fmt.Printf("exp1 STATE CARRYOVER g%d p%d: repeat mirror %v/%d first %v/%d want %v\n", p.game, p.ply, rf, rn, want, mirrorNodes, want)
		}
		fmt.Fprintln(f, row+" |")
	}
	totals["twoPhase"] = twoPhaseNodes
	fmt.Printf("exp1 ref total nodes %d, %d/%d positions have four sources\n", refNodes, withSources, len(tactical))
	for _, r := range runs {
		fmt.Printf("exp1 %-10s total nodes %8d verdict flips %d\n", r.name, totals[r.name], flips[r.name])
	}
	fmt.Printf("exp1 %-10s total nodes %8d verdict flips %d\n", "twoPhase", totals["twoPhase"], flips["twoPhase"])
}

func runExperiment2(games []corpusGame) {
	tracker := &candTracker{}
	moves, parityFails, keyFails := 0, 0, 0
	for _, g := range games {
		if len(g.keys) < 2 {
			continue
		}
		b, err := boardFromKey(g.keys[0])
		if err != nil {
			continue
		}
		*tracker = candTracker{}
		if keyOfBoard(b) != g.keys[0] {
			keyFails++
			continue
		}
		for i := range g.moves {
			cell := rules.Cell(g.moves[i])
			tracker.make(cell)
			b.Make(cell)
			moves++
			if !tracker.matches(b) {
				parityFails++
			}
			if i+1 < len(g.keys) && keyOfBoard(b) != g.keys[i+1] {
				keyFails++
			}
		}
	}
	f, _ := os.Create(filepath.Join(outDir, "exp2-candgen.md"))
	defer f.Close()
	fmt.Fprintf(f, "# experiment 2 incremental candidate set\n\nmoves %d parity fails %d key fails %d\nthroughput: see BenchmarkDilateCandidateSet and BenchmarkCandIncremental\n", moves, parityFails, keyFails)
	fmt.Printf("exp2 moves %d parity fails %d key fails %d\n", moves, parityFails, keyFails)
}

func runExperiment3(positions []benchPos) {
	truthCount, screenCount, falsePos := 0, 0, 0
	for _, p := range positions {
		truth := tableFours(p.board, p.board.Side)
		screen := convFours(p.board, p.board.Side)
		s, fp := screenTightness(truth, screen)
		truthCount += countTrue(truth)
		screenCount += s
		falsePos += fp
	}
	f, _ := os.Create(filepath.Join(outDir, "exp3-conv.md"))
	defer f.Close()
	fmt.Fprintf(f, "# experiment 3 convolution screen vs table extraction\n\nscreen cells %d truth cells %d false positives %d (%.1f%%)\nthroughput: see BenchmarkTableFoursBoard and BenchmarkConvFoursBoard\n",
		screenCount, truthCount, falsePos, 100*float64(falsePos)/float64(max(1, screenCount)))
	fmt.Printf("exp3 screen/truth %d/%d fp %.1f%%\n", screenCount, truthCount, 100*float64(falsePos)/float64(max(1, screenCount)))
}

func countTrue(xs []bool) int {
	n := 0
	for _, x := range xs {
		if x {
			n++
		}
	}
	return n
}

func runExperiment4(positions []benchPos) {
	p := positions[0]
	hashBefore, sideBefore, movesBefore := p.board.Hash, p.board.Side, p.board.MoveCount
	e := newMcts(p.board, mctsSeed)
	start := time.Now()
	wins := map[rules.Color]int{}
	for range mctsPlayouts {
		wins[e.playout()]++
	}
	elapsed := time.Since(start)
	if p.board.Hash != hashBefore || p.board.Side != sideBefore || p.board.MoveCount != movesBefore {
		fmt.Println("exp4 FATAL board not restored")
		os.Exit(1)
	}
	depth := 0
	var walk func(n *mctsNode, d int)
	walk = func(n *mctsNode, d int) {
		if d > depth {
			depth = d
		}
		for _, c := range n.children {
			walk(c, d+1)
		}
	}
	walk(e.root, 0)
	rate := float64(mctsPlayouts) / elapsed.Seconds()
	f, _ := os.Create(filepath.Join(outDir, "exp4-mcts.md"))
	defer f.Close()
	fmt.Fprintf(f, "# experiment 4 uct core\n\nplayouts %d in %.2f s = %.0f playouts/s\nred %d blue %d draw %d nodes %d root children %d max depth %d\n",
		mctsPlayouts, elapsed.Seconds(), rate, wins[rules.Red], wins[rules.Blue], wins[rules.Empty], e.used, len(e.root.children), depth)
	fmt.Printf("exp4 playouts/s %.0f (red %d blue %d draw %d nodes %d root children %d max depth %d) %.2f s\n",
		rate, wins[rules.Red], wins[rules.Blue], wins[rules.Empty], e.used, len(e.root.children), depth, elapsed.Seconds())
}
