package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type stubAnalyst struct {
	calls    int
	score    int
	rankings map[int][]rankedMove
	defaultK []rankedMove
}

func (s *stubAnalyst) analyze(b *rules.Board, budget, k int) (leafRec, []rankedMove) {
	s.calls++
	score := s.score
	rank := s.defaultK
	if s.rankings != nil {
		if r, ok := s.rankings[b.MoveCount]; ok {
			rank = r
		}
	}
	if rank == nil && b.MoveCount == 0 {
		rank = rootOpeningRanking()
	}
	if rank == nil {
		rank = topLegal(b, 6, score)
	}
	best := rank[0]
	return leafRec{budget: budget, score: best.Score, best: best.Move, nodes: uint64(budget)}, rank
}

func topLegal(b *rules.Board, k, score int) []rankedMove {
	var out []rankedMove
	for c := 0; c < config.BoardCells && len(out) < k; c++ {
		if !b.IsLegal(rules.Cell(c)) {
			continue
		}
		out = append(out, rankedMove{Move: rules.Move(c), Score: score + k - len(out)})
	}
	return out
}

func legalNear(b *rules.Board) rules.Move {
	for c := 0; c < config.BoardCells; c++ {
		if b.IsLegal(rules.Cell(c)) {
			return rules.Move(c)
		}
	}
	return 0
}

func secondLegal(b *rules.Board) rules.Move {
	found := 0
	for c := config.BoardCells - 1; c >= 0; c-- {
		if b.IsLegal(rules.Cell(c)) {
			found++
			if found == 1 {
				return rules.Move(c)
			}
		}
	}
	return 0
}

type stubSolver struct {
	proof bool
	pvLen int
	nodes uint64
	calls int
}

func (s *stubSolver) probe(b *rules.Board, budget int) (rules.Move, uint64, string, int, bool) {
	s.calls++
	if !s.proof {
		return 0, s.nodes, "", 0, false
	}
	return legalNear(b), s.nodes, "vcf", s.pvLen, true
}

func tinyParams() treeParams {
	p := paramsFromConfig(config.TierMaster.Name)
	p.leaf = 100
	p.critical = 200
	p.total = 100_000
	p.solver = 50
	return p
}

func newTestTree(t *testing.T, analyst analystPort, solver solverPort) (*bookTree, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tree.txt")
	bt, err := newBookTree(tinyParams(), 4, priorsTable{}, analyst, solver, path)
	if err != nil {
		t.Fatal(err)
	}
	return bt, path
}

func rootOpeningRanking() []rankedMove {
	var rm []rankedMove
	for i, c := range config.BookOpeningCells {
		rm = append(rm, rankedMove{Move: rules.Move(c), Score: 5000 - i})
	}
	return rm
}

func TestTreeRootExpandsAllProtocolOpenings(t *testing.T) {
	sa := &stubAnalyst{score: 5000, rankings: map[int][]rankedMove{0: rootOpeningRanking()}}
	bt, _ := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	ex, ok := bt.log.expansions[bt.root]
	if !ok {
		t.Fatal("root never expanded")
	}
	want := make(map[[config.BookKeyBytes]byte]struct{}, len(config.BookOpeningCells))
	for _, c := range config.BookOpeningCells {
		b := rules.NewBoard()
		b.Make(rules.Cell(c))
		ck, _ := canonicalEntry(keyOf(b), 0)
		want[ck] = struct{}{}
	}
	got := make(map[[config.BookKeyBytes]byte]struct{}, len(ex.children))
	for _, c := range ex.children {
		got[c.child] = struct{}{}
	}
	if len(got) != len(ex.children) {
		t.Fatalf("root children %d hold %d distinct keys, orbit-mate duplicates must not enqueue", len(ex.children), len(got))
	}
	if len(got) != len(want) {
		t.Fatalf("distinct canonical root children %d, want %d covering every protocol opening", len(got), len(want))
	}
	for ck := range want {
		if _, ok := got[ck]; !ok {
			t.Fatalf("protocol opening orbit %s missing from the root expansion", hexKey(ck[:])[:12])
		}
	}
	if !ex.own {
		t.Error("root must be an owner node, red moves first")
	}
	if len(bt.frontier) != 0 {
		t.Errorf("full drain leaves frontier %d, want 0", len(bt.frontier))
	}
	if len(bt.log.analyses) < len(got)+1 {
		t.Errorf("analyses %d, want at least root plus one per canonical child", len(bt.log.analyses))
	}
}

func TestTreeOwnerExpandsBestOnlyOpponentExpandsTopK(t *testing.T) {
	sa := &stubAnalyst{score: 200, rankings: map[int][]rankedMove{0: rootOpeningRanking()}}
	bt, _ := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	owners, opponents := 0, 0
	for _, ex := range bt.log.expansions {
		if ex.ply == 0 {
			continue
		}
		if ex.own {
			owners++
			if len(ex.children) != 1 {
				t.Errorf("owner node at ply %d expanded %d children, want 1", ex.ply, len(ex.children))
			}
		} else {
			opponents++
			if len(ex.children) != bt.params.opponentK {
				t.Errorf("opponent node at ply %d expanded %d children, want %d", ex.ply, len(ex.children), bt.params.opponentK)
			}
		}
	}
	if owners == 0 || opponents == 0 {
		t.Fatalf("expected both node kinds, owners %d opponents %d", owners, opponents)
	}
}

func TestTreeSolverProofFreezesWinAndParentLoses(t *testing.T) {
	sa := &stubAnalyst{score: 200}
	ss := &stubSolver{nodes: 10}
	ss.proof = true
	ss.pvLen = 3
	bt, _ := newTestTree(t, sa.analyze, ss.probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	if ss.calls == 0 {
		t.Fatal("solver never probed")
	}
	wins := 0
	for _, f := range bt.log.frozen {
		if f.reason == "win" {
			wins++
		}
	}
	if wins == 0 {
		t.Fatal("no proof froze a win")
	}
	for _, f := range bt.log.frozen {
		if f.reason == "win" {
			if f.score != mateScore(3) {
				t.Errorf("proof score = %d, want mate score at distance 3", f.score)
			}
			break
		}
	}
}

func TestTreeStabilityFreezesQuietLopsided(t *testing.T) {
	sa := &stubAnalyst{score: 5000}
	bt, _ := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	quiet := 0
	for _, f := range bt.log.frozen {
		if f.reason == "quiet" {
			quiet++
		}
	}
	if quiet == 0 {
		t.Fatal("no stable lopsided leaf froze quiet")
	}
	nonBound := 0
	for key, recs := range bt.log.analyses {
		if f, ok := bt.log.frozen[key]; ok && f.reason == "quiet" && len(recs) >= bt.params.stableIters {
			nonBound++
		}
	}
	if nonBound == 0 {
		t.Error("quiet freezes must carry the full stability ladder of analyses")
	}
}

func TestTreeUnstableLeafExpandsInsteadOfFreezing(t *testing.T) {
	scores := []int{5000, 4800, 4600}
	idx := 0
	analyst := func(b *rules.Board, budget, k int) (leafRec, []rankedMove) {
		s := scores[idx%len(scores)]
		idx++
		rank := topLegal(b, 6, s)
		if b.MoveCount == 0 {
			rank = rootOpeningRanking()
		}
		best := rank[0]
		return leafRec{budget: budget, score: best.Score, best: best.Move, nodes: uint64(budget)}, rank
	}
	bt, _ := newTestTree(t, analyst, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	wandered := false
	for key, f := range bt.log.frozen {
		if f.reason != "quiet" {
			continue
		}
		recs := bt.log.analyses[key]
		if len(recs) < bt.params.stableIters {
			t.Errorf("quiet freeze at %s without the stability ladder", hexKey(key[:])[:12])
			continue
		}
		lo, hi := recs[0].score, recs[0].score
		for _, r := range recs {
			if r.score < lo {
				lo = r.score
			}
			if r.score > hi {
				hi = r.score
			}
		}
		if hi-lo > bt.params.stableMarg || absInt(f.score) < bt.params.lopsided {
			t.Errorf("quiet freeze at %s violates the stability law: spread %d score %d", hexKey(key[:])[:12], hi-lo, f.score)
		}
	}
	for key, recs := range bt.log.analyses {
		lo, hi := recs[0].score, recs[0].score
		for _, r := range recs {
			if r.score < lo {
				lo = r.score
			}
			if r.score > hi {
				hi = r.score
			}
		}
		if len(recs) >= bt.params.stableIters && hi-lo > bt.params.stableMarg {
			wandered = true
			if f, ok := bt.log.frozen[key]; ok && f.reason == "quiet" {
				t.Errorf("wandering node %s froze quiet", hexKey(key[:])[:12])
			}
		}
	}
	if !wandered {
		t.Fatal("fixture never produced a wandering ladder")
	}
}

func TestTreePlyBoundLeavesFreezeAndNeverExpand(t *testing.T) {
	sa := &stubAnalyst{score: 200}
	bt, _ := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	for key, f := range bt.log.frozen {
		if f.reason != "bound" {
			continue
		}
		if _, expanded := bt.log.expansions[key]; expanded {
			t.Fatal("a ply-bound leaf must never expand")
		}
		break
	}
	found := false
	for _, f := range bt.log.frozen {
		if f.reason == "bound" {
			found = true
		}
	}
	if !found {
		t.Fatal("no bound leaf recorded")
	}
}

func TestTreeBudgetCapStopsTheRun(t *testing.T) {
	sa := &stubAnalyst{score: 200}
	p := tinyParams()
	p.total = 120
	path := filepath.Join(t.TempDir(), "tree.txt")
	bt, err := newBookTree(p, 4, priorsTable{}, sa.analyze, (&stubSolver{nodes: 10}).probe, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	if bt.spent > uint64(p.total)+uint64(p.critical) {
		t.Errorf("spent %d exceeds cap %d by more than one analysis", bt.spent, p.total)
	}
	if len(bt.frontier) == 0 {
		t.Error("tiny budget should leave frontier residue")
	}
}

func TestTreeResumeReplaysLogWithoutReanalysis(t *testing.T) {
	sa := &stubAnalyst{score: 5000, rankings: map[int][]rankedMove{0: rootOpeningRanking()}}
	path := filepath.Join(t.TempDir(), "tree.txt")
	bt, err := newBookTree(tinyParams(), 2, priorsTable{}, sa.analyze, (&stubSolver{nodes: 10}).probe, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	callsFirst := sa.calls
	bt2, err := newBookTree(tinyParams(), 2, priorsTable{}, sa.analyze, (&stubSolver{nodes: 10}).probe, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := bt2.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	if sa.calls != callsFirst {
		t.Errorf("resume ran %d extra analyses, want zero on a completed log", sa.calls-callsFirst)
	}
	if len(bt2.log.frozen) != len(bt.log.frozen) {
		t.Error("replayed freeze set differs from the original run")
	}
}

func TestTreeFrontierPrefersContestedNodes(t *testing.T) {
	sa := &stubAnalyst{score: 200}
	p := tinyParams()
	p.total = 3000
	path := filepath.Join(t.TempDir(), "tree.txt")
	bt, err := newBookTree(p, 4, priorsTable{}, sa.analyze, (&stubSolver{nodes: 10}).probe, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(bt.frontier) < 2 {
		t.Fatalf("frontier %d after a capped run, want residue to order", len(bt.frontier))
	}
	bt.sortFrontier(bt.principalVariation())
	for i := 1; i < len(bt.frontier); i++ {
		prev, cur := bt.frontier[i-1], bt.frontier[i]
		pw := absInt(bt.provisional(prev.key)) + prev.attempts*bt.params.critMargin
		cw := absInt(bt.provisional(cur.key)) + cur.attempts*bt.params.critMargin
		if pw > cw {
			t.Fatalf("frontier order violated at %d: %d then %d", i, pw, cw)
		}
	}
}

func TestTreeCriticalChildrenFromParentGap(t *testing.T) {
	rank := make([]rankedMove, len(config.BookOpeningCells))
	for i, c := range config.BookOpeningCells {
		rank[i] = rankedMove{Move: rules.Move(c), Score: 4000 - i*10}
	}
	rank[0].Score = 5000
	rank[1].Score = 4900
	rank[2].Score = 4800
	sa := &stubAnalyst{rankings: map[int][]rankedMove{0: rank}, score: 200}
	bt, _ := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	ex, ok := bt.log.expansions[bt.root]
	if !ok {
		t.Fatal("root not expanded")
	}
	if ex.bestGap != 100 {
		t.Fatalf("root gap = %d, want 100", ex.bestGap)
	}
	criticalBudgets := 0
	for _, recs := range bt.log.analyses {
		for _, r := range recs {
			if r.budget == bt.params.critical {
				criticalBudgets++
			}
		}
	}
	if criticalBudgets == 0 {
		t.Error("gap 100 under critMargin 300 must fund at least one critical-budget analysis")
	}
}

func TestTreePriorsTieBreakFavorsRealMasterPlay(t *testing.T) {
	a := boardWithMoves(t, rules.Cell(119), rules.Cell(72))
	path := writePriorsCorpus(t, []posRec{priorsRec(a, rules.Move(33))})
	pt, err := loadPriors([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	canon, playedMv := canonicalEntry(keyOf(a), rules.Move(33))
	rank := []rankedMove{{Move: 10, Score: 100}, {Move: playedMv, Score: 100}}
	got := rankedByScoreMoves(rank, canon, pt)
	if got[0].Move != playedMv {
		t.Error("master-played move must win the tie at an opponent node")
	}
}

func TestTreeResumeSpendCountsLoggedNodes(t *testing.T) {
	sa := &stubAnalyst{score: 200}
	path := treeLogPath(t)
	p := tinyParams()
	p.total = 600
	clean, err := writeSidecarHeader(path, p.fingerprint(4))
	if err != nil {
		t.Fatal(err)
	}
	k := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(72)))
	clean, err = appendTreeLine(path, clean, formatLeaf(k, leafRec{budget: 100, score: 5, best: 7, nodes: 500}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appendTreeLine(path, clean, formatNoProof(k, 100)); err != nil {
		t.Fatal(err)
	}
	bt, err := newBookTree(p, 4, priorsTable{}, sa.analyze, (&stubSolver{nodes: 10}).probe, path)
	if err != nil {
		t.Fatal(err)
	}
	if bt.spent != 600 {
		t.Fatalf("resumed spend = %d, want the logged 600", bt.spent)
	}
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	if sa.calls != 0 {
		t.Fatalf("resume ran %d analyses, want 0 under an exhausted cap", sa.calls)
	}
	if bt.stopped != "budget spent" {
		t.Fatalf("stop reason %q, want budget spent", bt.stopped)
	}
}

func TestTreeSolverRecordWithoutFreezeRepairsOnLoad(t *testing.T) {
	path := treeLogPath(t)
	p := tinyParams()
	clean, err := writeSidecarHeader(path, p.fingerprint(4))
	if err != nil {
		t.Fatal(err)
	}
	k := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(72)))
	if _, err := appendTreeLine(path, clean, formatSolver(k, 33, "vcf", 10, 3)); err != nil {
		t.Fatal(err)
	}
	bt, err := newBookTree(p, 4, priorsTable{}, (&stubAnalyst{score: 200}).analyze, (&stubSolver{nodes: 10}).probe, path)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := bt.log.frozen[k]
	if !ok {
		t.Fatal("crash-window solver record was not re-frozen on load")
	}
	if f.reason != "win" || f.score != mateScore(3) {
		t.Fatalf("repaired freeze = %+v, want win at mate distance 3", f)
	}
	if bt.spent != 10 {
		t.Fatalf("repaired spend = %d, want the logged solver nodes 10", bt.spent)
	}
}

func TestTreeExpandConsumesSolverProofMove(t *testing.T) {
	bt, _ := newTestTree(t, (&stubAnalyst{score: 200}).analyze, (&stubSolver{nodes: 10}).probe)
	k := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(72)))
	b, err := boardFromKey(k)
	if err != nil {
		t.Fatal(err)
	}
	proof := legalNear(b)
	bt.log.solver[k] = solverRec{move: proof, kind: "vcf", pvLen: 2}
	ranking := topLegal(b, 6, 200)
	if err := bt.expand(k, b, 1, ranking); err != nil {
		t.Fatal(err)
	}
	ex, ok := bt.log.expansions[k]
	if !ok {
		t.Fatal("expansion missing")
	}
	if len(ex.children) != 1 || ex.children[0].move != proof || ex.best != proof {
		t.Fatalf("solver node expanded %+v, want the single proof move %d", ex, proof)
	}
}

func TestTreeHoldDefersOffPVNodesAndStopsWhenAllHeld(t *testing.T) {
	sa := &stubAnalyst{score: 200}
	path := treeLogPath(t)
	p := tinyParams()
	clean, err := writeSidecarHeader(path, p.fingerprint(4))
	if err != nil {
		t.Fatal(err)
	}
	root := canonicalKey(corpusZeroKey)
	c1 := [config.BookKeyBytes]byte{1}
	c2 := [config.BookKeyBytes]byte{2}
	c3 := [config.BookKeyBytes]byte{3}
	clean, err = appendTreeLine(path, clean, formatExpansion(root, treeExpansion{ply: 0, own: true, children: []childRec{{move: 9, child: c1}, {move: 8, child: c2}}}))
	if err != nil {
		t.Fatal(err)
	}
	clean, err = appendTreeLine(path, clean, formatFreeze(c1, "win", mateScore(2)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appendTreeLine(path, clean, formatExpansion(c2, treeExpansion{ply: 1, own: false, children: []childRec{{move: 7, child: c3}}})); err != nil {
		t.Fatal(err)
	}
	bt, err := newBookTree(p, 4, priorsTable{}, sa.analyze, (&stubSolver{nodes: 10}).probe, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(bt.frontier) != 1 || bt.frontier[0].key != c3 {
		t.Fatalf("construction frontier = %+v, want only the open child", bt.frontier)
	}
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	if bt.stopped != "frontier held" {
		t.Fatalf("stop reason %q, want frontier held", bt.stopped)
	}
	if bt.frontierHeld() != 1 {
		t.Fatalf("held = %d, want the single off-PV entry deferred once", bt.frontierHeld())
	}
	if sa.calls != 0 {
		t.Fatalf("held node ran %d analyses, deferral must precede analysis", sa.calls)
	}
	if _, expanded := bt.log.expansions[c3]; expanded {
		t.Fatal("held node expanded despite the all-held stop")
	}
}

func TestTreeFrontierAgingSortsDeferredLast(t *testing.T) {
	bt, _ := newTestTree(t, (&stubAnalyst{score: 200}).analyze, (&stubSolver{nodes: 10}).probe)
	a := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(72)))
	b := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(73)))
	bt.frontier = []frontierEntry{{key: a, ply: 2}, {key: b, ply: 2, attempts: 1}}
	bt.sortFrontier(bt.principalVariation())
	if bt.frontier[0].key != a || bt.frontier[1].key != b {
		t.Fatal("deferred entry must age behind an equal-weight fresh entry")
	}
}

func TestTreeStabilityLadderIsGeometric(t *testing.T) {
	sa := &stubAnalyst{score: 5000, rankings: map[int][]rankedMove{0: rootOpeningRanking()}}
	bt, _ := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	laddered := 0
	for key, f := range bt.log.frozen {
		if f.reason != "quiet" {
			continue
		}
		recs := bt.log.analyses[key]
		if len(recs) != bt.params.stableIters {
			t.Fatalf("quiet freeze without the full ladder: %d recs", len(recs))
		}
		base := recs[0].budget
		for i, want := range []int{base, base * 2, base * 4} {
			if recs[i].budget != want {
				t.Fatalf("ladder budgets %v, want geometric base 2x 4x from %d", recs, base)
			}
		}
		laddered++
	}
	if laddered == 0 {
		t.Fatal("no quiet freeze to inspect")
	}
}

func TestTreeBackUpStepsMateDistances(t *testing.T) {
	if got, want := backUp(mateScore(1)), -mateScore(2); got != want {
		t.Fatalf("backUp of a win in 1 = %d, want %d", got, want)
	}
	if got, want := backUp(-mateScore(2)), mateScore(3); got != want {
		t.Fatalf("backUp of a loss in 2 = %d, want %d", got, want)
	}
	if got, want := backUp(750), -750; got != want {
		t.Fatalf("backUp of a quiet score = %d, want %d", got, want)
	}
	path := treeLogPath(t)
	p := tinyParams()
	clean, err := writeSidecarHeader(path, p.fingerprint(4))
	if err != nil {
		t.Fatal(err)
	}
	k1 := [config.BookKeyBytes]byte{1}
	k2 := [config.BookKeyBytes]byte{2}
	k3 := [config.BookKeyBytes]byte{3}
	clean, err = appendTreeLine(path, clean, formatExpansion(k1, treeExpansion{ply: 0, own: true, children: []childRec{{move: 5, child: k2}}}))
	if err != nil {
		t.Fatal(err)
	}
	clean, err = appendTreeLine(path, clean, formatExpansion(k2, treeExpansion{ply: 1, own: false, children: []childRec{{move: 6, child: k3}}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appendTreeLine(path, clean, formatFreeze(k3, "win", mateScore(1))); err != nil {
		t.Fatal(err)
	}
	bt, err := newBookTree(p, 4, priorsTable{}, (&stubAnalyst{score: 200}).analyze, (&stubSolver{nodes: 10}).probe, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := bt.cascade(); err != nil {
		t.Fatal(err)
	}
	if got := bt.log.frozen[k2]; got.reason != "loss" || got.score != -mateScore(2) {
		t.Fatalf("mid freeze = %+v, want loss at distance 2", got)
	}
	if got := bt.log.frozen[k1]; got.score != mateScore(3) {
		t.Fatalf("root freeze score = %d, want a backed win at distance 3", got.score)
	}
}

func TestTreeReverifyPassIsIdempotentAndKeyCounted(t *testing.T) {
	rank := make([]rankedMove, len(config.BookOpeningCells))
	for i, c := range config.BookOpeningCells {
		rank[i] = rankedMove{Move: rules.Move(c), Score: 4000 - i*10}
	}
	sa := &stubAnalyst{rankings: map[int][]rankedMove{0: rank}, score: 200}
	bt, path := newTestTree(t, sa.analyze, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	calls := 0
	port := func(b *rules.Board, budget int) (rules.Move, int, uint64) {
		calls++
		ck := canonicalBoardKey(b)
		recs := bt.log.analyses[ck]
		if len(recs) == 0 {
			return legalNear(b), 0, uint64(budget)
		}
		last := recs[len(recs)-1]
		return last.best, last.score, uint64(budget)
	}
	if err := bt.reverifyPass(port, io.Discard); err != nil {
		t.Fatal(err)
	}
	firstCalls := calls
	firstRecords := len(bt.log.reverify)
	sizeFirst, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if firstRecords == 0 {
		t.Fatal("no contested owner node was ever reverified")
	}
	if err := bt.reverifyPass(port, io.Discard); err != nil {
		t.Fatal(err)
	}
	if calls != firstCalls {
		t.Fatalf("second pass ran %d extra probes, want 0", calls-firstCalls)
	}
	sizeSecond, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if sizeSecond.Size() != sizeFirst.Size() {
		t.Fatal("second pass appended records, the done-guard failed")
	}
	if bt.flipCount() != 0 {
		t.Fatalf("flipCount = %d, want 0 with agreeing probes", bt.flipCount())
	}
}

func TestTreeEmittedMoveMatchesExpandedBest(t *testing.T) {
	analyst := func(b *rules.Board, budget, k int) (leafRec, []rankedMove) {
		rank := topLegal(b, 6, 200)
		if b.MoveCount == 0 {
			rank = rootOpeningRanking()
		}
		second := rank[0]
		if len(rank) > 1 {
			second = rank[1]
		}
		return leafRec{budget: budget, score: second.Score, best: second.Move, nodes: uint64(budget)}, rank
	}
	bt, _ := newTestTree(t, analyst, (&stubSolver{nodes: 10}).probe)
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	entries, _, err := bt.emitEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no entries emitted")
	}
	index := make(map[[config.BookKeyBytes]byte]bookEntry, len(entries))
	for _, ent := range entries {
		index[ent.key] = ent
	}
	checked := 0
	for key, ex := range bt.log.expansions {
		if !ex.own {
			continue
		}
		ent, ok := index[key]
		if !ok {
			continue
		}
		if ent.move != ex.best {
			t.Fatalf("emitted move %d differs from expanded best %d at %s", ent.move, ex.best, hexKey(key[:])[:12])
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no own expansion ever verified against its emitted entry")
	}
}
