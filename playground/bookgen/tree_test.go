package main

import (
	"io"
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
	if len(ex.children) != len(config.BookOpeningCells) {
		t.Fatalf("root children = %d, want all %d protocol openings", len(ex.children), len(config.BookOpeningCells))
	}
	if !ex.own {
		t.Error("root must be an owner node, red moves first")
	}
	if len(bt.frontier) != 0 {
		t.Errorf("full drain leaves frontier %d, want 0", len(bt.frontier))
	}
	distinct := make(map[[config.BookKeyBytes]byte]struct{})
	for _, c := range ex.children {
		distinct[c.child] = struct{}{}
	}
	if len(distinct) >= len(config.BookOpeningCells) {
		t.Errorf("distinct canonical children %d: orbit-mate openings must merge", len(distinct))
	}
	if len(bt.log.analyses) < len(distinct)+1 {
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
