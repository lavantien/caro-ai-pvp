package main

import (
	"fmt"
	"io"

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
	var out vcf.SolverStats
	if fours.Solve(b, budget, nil, &out) {
		return rules.Move(out.PV[0]), out.Nodes, "vcf", len(out.PV), true
	}
	if threes.Solve(b, budget, nil, &out) {
		return rules.Move(out.PV[0]), out.Nodes, "vct", len(out.PV), true
	}
	return 0, out.Nodes, "", 0, false
}

func (bt *bookTree) reverifyPass(progress io.Writer) error {
	gm := config.TierGrandmaster
	gm.Book = false
	checked := 0
	for key, ex := range bt.log.expansions {
		if !ex.own || ex.ply > bt.params.opponentK || ex.bestGap >= bt.params.critMargin {
			continue
		}
		recs := bt.log.analyses[key]
		if len(recs) == 0 {
			continue
		}
		latest := recs[len(recs)-1]
		b, err := boardFromKey(key)
		if err != nil {
			return fmt.Errorf("bookgen: reverify position: %w", err)
		}
		s := engine.NewTiered(gm)
		mv, st := s.SearchDepth(b, engine.NewNodeBudget(uint64(bt.params.critical)), config.SearchMaxPly)
		s.Close()
		delta := st.Score - latest.score
		flip := mv != latest.best && absInt(delta) > bt.params.prune
		bt.log.reverify = append(bt.log.reverify, reverifyRec{key: key, smp: st.Score, delta: delta, flip: flip})
		if err := bt.append(formatReverify(reverifyRec{key: key, smp: st.Score, delta: delta, flip: flip})); err != nil {
			return err
		}
		bt.spent += st.Nodes
		checked++
	}
	fmt.Fprintf(progress, "reverify pass: %d contested owner nodes checked, %d flips\n", checked, bt.flipCount())
	return nil
}

func (bt *bookTree) flipCount() int {
	n := 0
	for _, r := range bt.log.reverify {
		if r.flip {
			n++
		}
	}
	return n
}
