package main

import (
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func probeBestMove(b *rules.Board) (rules.Move, engine.SearchStats) {
	e := engine.New(1 << 22)
	dl := engine.NewStoppable()
	mv, stats := e.SearchDepth(b, dl, 4)
	return mv, stats
}
