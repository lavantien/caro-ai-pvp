package main

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestTreeEmitVariantsAndWeights(t *testing.T) {
	sa := &stubAnalyst{score: 5000, rankings: map[int][]rankedMove{0: rootOpeningRanking()}}
	bt, _ := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	entries, st, err := bt.emitEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no entries emitted")
	}
	rootEntries := 0
	wantRootVisits := visitWeight(bt.log.expansions[bt.root].bestGap, bt.params.prune)
	for _, e := range entries {
		if e.key == corpusZeroKey {
			rootEntries++
			if e.visits != wantRootVisits {
				t.Errorf("root visits = %d, want %d from the root gap", e.visits, wantRootVisits)
			}
		}
	}
	if rootEntries != 1 {
		t.Fatalf("empty-board entries = %d, want 1", rootEntries)
	}
	prev := [config.BookKeyBytes]byte{}
	for i, e := range entries {
		if i > 0 && !keyBefore(prev, e.key) {
			t.Fatalf("entries not strictly ascending at %d", i)
		}
		prev = e.key
	}
	if st.Emitted != len(entries) {
		t.Error("stats emitted count disagrees with entries")
	}
}

func TestTreeEmitSkipsLossFrozenOwnerNodes(t *testing.T) {
	bt, _ := newTestTree(t, (&stubAnalyst{score: 200}).analyze, (&stubSolver{nodes: 10}).probe)
	key := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(72), rules.Cell(0)))
	bt.log.frozen[key] = freezeRec{reason: "loss", score: -config.EvalMateMax}
	bt.log.analyses[key] = []leafRec{{budget: 100, score: -config.EvalMateMax, best: 5, nodes: 100}}
	entries, _, err := bt.emitEntries()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		for _, v := range variantPairs(key) {
			if e.key == v.key {
				t.Fatal("loss-frozen owner node must not emit")
			}
		}
	}
}

func TestTreeEmitSolverSourceWinsOverAnalysis(t *testing.T) {
	bt, _ := newTestTree(t, (&stubAnalyst{score: 200}).analyze, (&stubSolver{nodes: 10}).probe)
	b := boardWithMoves(t, rules.Cell(119), rules.Cell(72))
	key := canonicalBoardKey(b)
	bt.log.analyses[key] = []leafRec{{budget: 100, score: 300, best: legalNear(b), nodes: 100}}
	bt.log.solver[key] = solverRec{move: secondLegal(b), kind: "vct"}
	entries, st, err := bt.emitEntries()
	if err != nil {
		t.Fatal(err)
	}
	if st.SolverProofs == 0 {
		t.Fatal("solver proof not counted")
	}
	found := false
	for _, e := range entries {
		if e.key == key && e.source == config.BookSourceSolver && e.move == secondLegal(b) {
			found = true
		}
	}
	if !found {
		t.Fatal("solver entry missing or carrying the analysis move")
	}
}

func TestTreeEmitCoversBothOwners(t *testing.T) {
	bt, _ := newTestTree(t, (&stubAnalyst{score: 200}).analyze, (&stubSolver{nodes: 10}).probe)
	redNode := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(72)))
	blueNode := canonicalBoardKey(boardWithMoves(t, rules.Cell(119)))
	redBest := legalNear(boardWithMoves(t, rules.Cell(119), rules.Cell(72)))
	blueBest := legalNear(boardWithMoves(t, rules.Cell(119)))
	bt.log.analyses[redNode] = []leafRec{{budget: 100, score: 50, best: redBest, nodes: 100}}
	bt.log.analyses[blueNode] = []leafRec{{budget: 100, score: 50, best: blueBest, nodes: 100}}
	entries, _, err := bt.emitEntries()
	if err != nil {
		t.Fatal(err)
	}
	redSeen, blueSeen := false, false
	for _, e := range entries {
		if e.key == redNode {
			redSeen = true
			if e.move != redBest {
				t.Errorf("red entry move = %d, want the analysis best %d", e.move, redBest)
			}
		}
		if e.key == blueNode {
			blueSeen = true
			if e.move != blueBest {
				t.Errorf("blue entry move = %d, want the analysis best %d", e.move, blueBest)
			}
		}
	}
	if !redSeen {
		t.Fatal("red owner node missing from emission")
	}
	if !blueSeen {
		t.Fatal("blue owner node missing from emission: the blue seat must probe the trunk")
	}
	lostBlue := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(87), rules.Cell(71)))
	bt.log.frozen[lostBlue] = freezeRec{reason: "loss", score: -config.EvalMateMax}
	bt.log.analyses[lostBlue] = []leafRec{{budget: 100, score: -config.EvalMateMax, best: blueBest, nodes: 100}}
	entries, _, err = bt.emitEntries()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		for _, v := range variantPairs(lostBlue) {
			if e.key == v.key {
				t.Fatal("loss-frozen blue node emitted, a proven lost position must not advise")
			}
		}
	}
}

func TestTreeVisitWeightScalesWithGap(t *testing.T) {
	if got := visitWeight(0, config.BookTreePruneMarginMilli); got != 1 {
		t.Errorf("gap 0 weight = %d, want floor 1", got)
	}
	full := visitWeight(4*config.BookTreePruneMarginMilli, config.BookTreePruneMarginMilli)
	if full != uint32(config.BookTreeVisitScale) {
		t.Errorf("full gap weight = %d, want %d", full, config.BookTreeVisitScale)
	}
	if over := visitWeight(8*config.BookTreePruneMarginMilli, config.BookTreePruneMarginMilli); over != full {
		t.Errorf("gap beyond 4x margin must cap at full weight, got %d", over)
	}
}

func TestTreeWriteArtifactLoadsThroughBookContract(t *testing.T) {
	sa := &stubAnalyst{score: 5000, rankings: map[int][]rankedMove{0: rootOpeningRanking()}}
	bt, _ := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "book.carobook")
	stats := filepath.Join(dir, "tree-stats.txt")
	if _, err := bt.writeArtifact(out, stats, "2026-10-22T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	data := readAll(t, out)
	if !strings.HasPrefix(string(data), config.BookMagicLine) {
		t.Fatal("artifact missing CAROBOOK1 magic")
	}
	metaLine := strings.SplitN(strings.TrimPrefix(string(data), config.BookMagicLine), "\n", 2)[0]
	if !strings.Contains(metaLine, config.BookTreeGeneratorTag) {
		t.Errorf("meta %s lacks the tree generator tag", metaLine)
	}
	st := readAll(t, stats)
	for _, want := range []string{"canonical_entries", "emitted_entries", "solver_proofs", "frontier_at_stop", "total_nodes"} {
		if !strings.Contains(string(st), want) {
			t.Errorf("stats missing %s", want)
		}
	}
}

func TestTreeWriteArtifactRefusesEmpty(t *testing.T) {
	bt, _ := newTestTree(t, (&stubAnalyst{score: 200}).analyze, (&stubSolver{nodes: 10}).probe)
	dir := t.TempDir()
	if _, err := bt.writeArtifact(filepath.Join(dir, "b.carobook"), filepath.Join(dir, "s.txt"), "now"); err == nil {
		t.Fatal("zero-entry emission must refuse")
	}
}
