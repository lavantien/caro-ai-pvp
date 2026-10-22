package main

import "github.com/lavantien/caro-ai-pvp/internal/config"

const (
	defaultCorpusRel = "playground/bookgen/out/book-corpus.txt"

	waveUnreachable = uint8(255)
	wavePruneDepth  = 3

	solveBudget      = config.SolverNodeBudget
	solveMaxPly      = config.SolverMaxPly
	solverTTBits     = config.SolverTTBits
	candRadiusSolver = config.SolverCandidateRadius
	candRadiusSearch = config.SearchRingRadius

	positionMin   = 30
	positionPickA = 12
	positionPickB = 18

	tacticalWant = 40
	tacticalCap  = 160
	batchRepeat  = 200

	convRepeat   = 20
	mctsPlayouts = 20000
	mctsSeed     = 20261022
	mctsRollover = 40

	outDir = "out"
)
