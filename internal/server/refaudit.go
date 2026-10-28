package server

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

type excludedSearcher interface {
	SearchExcluded(b *rules.Board, dl engine.Deadline, exclude rules.Move) (rules.Move, engine.SearchStats)
}

func mateClaimPlies(score int) int {
	return (config.EvalMateMax - score) / config.EvalMateScoreStep
}

func opponentRefutes(b *rules.Board, mv rules.Move, score int, budget int, dl engine.Deadline) bool {
	if mv == rules.MoveNone || !b.IsLegal(rules.Cell(mv)) || score < config.EvalMateFloor {
		return false
	}
	claim := mateClaimPlies(score)
	if claim < 1 {
		return false
	}
	nb := *b
	nb.Make(rules.Cell(mv))
	for _, kind := range [...]vcf.Kind{vcf.KindVCF, vcf.KindVCT} {
		var out vcf.SolverStats
		if vcf.New(kind).Solve(&nb, budget, dl, &out) && out.Plies < claim {
			return true
		}
	}
	return false
}
