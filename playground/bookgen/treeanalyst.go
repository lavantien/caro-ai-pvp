package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

const treeAnalystTTBytes = 32 << 20

type engineAnalyst struct {
	rankBuf []engine.RankedMove
}

func (a *engineAnalyst) analyze(b *rules.Board, budget, k int) (leafRec, []rankedMove) {
	e := engine.New(treeAnalystTTBytes)
	e.SetExactRootRanking(true)
	mv, st := e.SearchDepth(b, engine.NewNodeBudget(uint64(budget)), config.SearchMaxPly)
	if len(a.rankBuf) < k {
		a.rankBuf = make([]engine.RankedMove, k)
	}
	rank := e.RootRanking(a.rankBuf[:k], k)
	e.Close()
	out := make([]rankedMove, 0, len(rank))
	for _, rm := range rank {
		out = append(out, rankedMove{Move: rm.Move, Score: rm.Score})
	}
	return leafRec{budget: budget, score: st.Score, best: mv, nodes: st.Nodes}, out
}

func treeSolverPort(b *rules.Board, budget int) (rules.Move, uint64, string, int, bool) {
	fours := vcf.New(vcf.KindVCF)
	threes := vcf.New(vcf.KindVCT)
	var outF vcf.SolverStats
	if fours.Solve(b, budget, nil, &outF) {
		return rules.Move(outF.PV[0]), outF.Nodes, "vcf", len(outF.PV), true
	}
	var outT vcf.SolverStats
	if threes.Solve(b, budget, nil, &outT) {
		return rules.Move(outT.PV[0]), outF.Nodes + outT.Nodes, "vct", len(outT.PV), true
	}
	return 0, outF.Nodes + outT.Nodes, "", 0, false
}

type reverifyPort func(b *rules.Board, budget int) (rules.Move, int, uint64)

func grandmasterReverify(b *rules.Board, budget int) (rules.Move, int, uint64) {
	gm := config.TierGrandmaster
	gm.Book = false
	s := engine.NewTiered(gm)
	defer s.Close()
	mv, st := s.SearchDepth(b, engine.NewNodeBudget(uint64(budget)), config.SearchMaxPly)
	return mv, st.Score, st.Nodes
}

func (bt *bookTree) reverifyPass(port reverifyPort, progress io.Writer) error {
	done := make(map[[config.BookKeyBytes]byte]struct{}, len(bt.log.reverify))
	for _, r := range bt.log.reverify {
		done[r.key] = struct{}{}
	}
	keys := make([][config.BookKeyBytes]byte, 0, len(bt.log.expansions))
	for key, ex := range bt.log.expansions {
		if !ex.own || ex.ply > bt.params.opponentK || ex.bestGap >= bt.params.critMargin {
			continue
		}
		if _, ok := done[key]; ok {
			continue
		}
		if len(bt.log.analyses[key]) == 0 {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keyBefore(keys[i], keys[j]) })
	checked := 0
	for _, key := range keys {
		recs := bt.log.analyses[key]
		latest := recs[len(recs)-1]
		b, err := boardFromKey(key)
		if err != nil {
			return fmt.Errorf("bookgen: reverify position: %w", err)
		}
		mv, score, nodes := port(b, bt.params.reverify)
		delta := score - latest.score
		flip := mv != latest.best && absInt(delta) > bt.params.prune
		rec := reverifyRec{key: key, smp: score, delta: delta, flip: flip}
		bt.log.reverify = append(bt.log.reverify, rec)
		if err := bt.append(formatReverify(rec)); err != nil {
			return err
		}
		bt.spent += nodes
		checked++
	}
	fmt.Fprintf(progress, "reverify pass: %d contested owner nodes checked, %d flips\n", checked, bt.flipCount())
	return nil
}

func (bt *bookTree) flipCount() int {
	flipped := make(map[[config.BookKeyBytes]byte]struct{})
	for _, r := range bt.log.reverify {
		if r.flip {
			flipped[r.key] = struct{}{}
		}
	}
	return len(flipped)
}
