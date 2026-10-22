package main

import (
	"math/bits"
	"testing"
)

var (
	sinkBB   bb
	sinkInt  int
	sinkBool []bool
)

func benchPositions(b *testing.B) []benchPos {
	path, err := resolveCorpus("")
	if err != nil {
		b.Skip(err.Error())
	}
	games, err := loadCorpus(path)
	if err != nil {
		b.Skip(err.Error())
	}
	ps := pickPositions(games, positionMin)
	if len(ps) == 0 {
		b.Skip("no positions")
	}
	return ps
}

func BenchmarkDilateCandidateSet(b *testing.B) {
	ps := benchPositions(b)
	p := ps[len(ps)/2]
	board := p.board
	for b.Loop() {
		near := dilateBB(bb(board.Full), candRadiusSearch)
		for w := range near {
			sinkInt += bits.OnesCount64(near[w] &^ board.Full[w])
		}
	}
}

func BenchmarkCandIncremental(b *testing.B) {
	ps := benchPositions(b)
	p := ps[len(ps)/2]
	board := p.board
	cell := freeInRegion(board, dilateBB(bb(board.Full), candRadiusSearch))[0]
	tr := &candTracker{}
	tr.make(cell)
	for b.Loop() {
		tr.unmake(cell)
		tr.make(cell)
	}
	sinkInt += int(tr.set[0])
}

func BenchmarkWavefrontBuild(b *testing.B) {
	ps := benchPositions(b)
	board := ps[len(ps)/2].board
	var wave []uint8
	for b.Loop() {
		wave = wavefront(board, board.Side)
	}
	for _, d := range wave {
		sinkInt += int(d)
	}
}

func BenchmarkTableFoursBoard(b *testing.B) {
	ps := benchPositions(b)
	board := ps[len(ps)/2].board
	var out []bool
	for b.Loop() {
		out = tableFours(board, board.Side)
	}
	sinkBool = out
}

func BenchmarkConvFoursBoard(b *testing.B) {
	ps := benchPositions(b)
	board := ps[len(ps)/2].board
	var out []bool
	for b.Loop() {
		out = convFours(board, board.Side)
	}
	sinkBool = out
}

func BenchmarkVcfMirrorSolve(b *testing.B) {
	ps := benchPositions(b)
	board := ps[len(ps)/2].board
	s := newSolver()
	wave := wavefront(board, board.Side)
	for b.Loop() {
		_, nodes, _ := s.solve(board, solveBudget, orderShipped, 0, wave)
		sinkInt += nodes
	}
}
