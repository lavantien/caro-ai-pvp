package vcf

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func (s *Solver) attack(b *rules.Board, ply int) result {
	if ply >= s.plyCap || !s.ok() {
		return resAbort
	}
	attacker := b.Side
	if ply > 0 {
		if s.ttProbe(b.Hash, s.plyCap-ply) {
			return resFail
		}
	}
	if n, first := s.probeWins(b, attacker, s.winA[:]); n > 0 {
		s.line[ply] = rules.Move(first)
		s.plies = ply + 1
		return resWin
	}
	if b.IsFull() {
		s.ttStoreFail(b.Hash, s.plyCap-ply)
		return resFail
	}
	if nD, defCell := s.probeWins(b, attacker.Opponent(), nil); nD > 0 {
		if nD > 1 {
			s.ttStoreFail(b.Hash, s.plyCap-ply)
			return resFail
		}
		return s.forcedBlock(b, attacker, defCell, ply)
	}
	return s.threatLine(b, attacker, ply)
}

func (s *Solver) forcedBlock(b *rules.Board, attacker rules.Color, u rules.Cell, ply int) result {
	var cands [1 + config.PatternDirections]rules.Cell
	cands[0] = u
	n := 1
	defender := attacker.Opponent()
	var frames [config.PatternDirections]fiveFrame
	for j, nf := 0, fiveFrames(b, defender, u, frames[:]); j < nf; j++ {
		f := frames[j]
		switch {
		case cellAt(b, f.br, f.bc, attacker) && playable(b, f.fr, f.fc):
			n = addCell(cands[:], n, cellOf(f.fr, f.fc, b.Geom))
		case cellAt(b, f.fr, f.fc, attacker) && playable(b, f.br, f.bc):
			n = addCell(cands[:], n, cellOf(f.br, f.bc, b.Geom))
		}
	}
	reach := dirReach(b, attacker)
	floor := s.kindFloor()
	overall := resFail
	for i := 0; i < n; i++ {
		if threatRank(b, cands[i], attacker, &reach) < floor {
			continue
		}
		s.line[ply] = rules.Move(cands[i])
		b.Make(cands[i])
		r := s.defend(b, cands[i], ply+1)
		b.Unmake()
		if r == resWin {
			return resWin
		}
		if r == resAbort {
			overall = resAbort
		}
	}
	if overall == resFail {
		s.ttStoreFail(b.Hash, s.plyCap-ply)
	}
	return overall
}

func (s *Solver) threatLine(b *rules.Board, attacker rules.Color, ply int) result {
	n := s.threatMoves(b, attacker, ply)
	overall := resFail
	for i := 0; i < n; i++ {
		pickBest(s.moves[ply][:], s.ranks[ply][:], i, n)
		m := s.moves[ply][i]
		s.line[ply] = m
		b.Make(rules.Cell(m))
		r := s.defend(b, rules.Cell(m), ply+1)
		b.Unmake()
		if r == resWin {
			return resWin
		}
		if r == resAbort {
			overall = resAbort
		}
	}
	if overall == resFail {
		s.ttStoreFail(b.Hash, s.plyCap-ply)
	}
	return overall
}

func (s *Solver) defend(b *rules.Board, last rules.Cell, ply int) result {
	if ply >= s.plyCap || !s.ok() {
		return resAbort
	}
	undecided := false
	attacker := b.Side.Opponent()
	if n, _ := s.probeWins(b, b.Side, nil); n > 0 {
		return resFail
	}
	if nA, _ := s.probeWins(b, attacker, s.winA[:]); nA > 0 {
		n := s.defusing(b, attacker, nA, ply)
		for i := 0; i < n; i++ {
			d := s.defends[ply][i]
			s.line[ply] = rules.Move(d)
			b.Make(d)
			r := s.attack(b, ply+1)
			b.Unmake()
			if r == resFail {
				return resFail
			}
			if r == resAbort {
				undecided = true
			}
		}
		if undecided {
			return resAbort
		}
		return resWin
	}
	if s.kind == KindVCF {
		return resFail
	}
	if !realThree(b, attacker, last) {
		return resFail
	}
	n := s.threatReplies(b, attacker, last, ply)
	for i := 0; i < n; i++ {
		d := s.defends[ply][i]
		s.line[ply] = rules.Move(d)
		b.Make(d)
		r := s.attack(b, ply+1)
		b.Unmake()
		if r == resFail {
			return resFail
		}
		if r == resAbort {
			undecided = true
		}
	}
	if undecided {
		return resAbort
	}
	return resWin
}

func (s *Solver) threatMoves(b *rules.Board, attacker rules.Color, ply int) int {
	reach := dirReach(b, attacker)
	floor := s.kindFloor()
	g := b.Geom
	cands := dilate(stones(b, attacker), config.SolverCandidateRadius, g)
	n := 0
	for w := range g.Words {
		free := cands[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			c := rules.Cell(w*wordBits + bits.TrailingZeros64(bit))
			if score := s.threatScore(b, c, attacker, &reach); score >= floor<<2 {
				s.moves[ply][n] = rules.Move(c)
				s.ranks[ply][n] = score
				n++
			}
		}
	}
	return n
}

func (s *Solver) threatReplies(b *rules.Board, attacker rules.Color, last rules.Cell, ply int) int {
	defender := attacker.Opponent()
	n := 0
	g := b.Geom
	reach := dirReach(b, defender)
	cands := dilate(stones(b, defender), config.SolverCandidateRadius, g)
	for w := range g.Words {
		free := cands[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			c := rules.Cell(w*wordBits + bits.TrailingZeros64(bit))
			if threatRank(b, c, defender, &reach) >= config.PatternClassFour {
				n = addCell(s.defends[ply][:], n, c)
			}
		}
	}
	for d := range g.Dirs {
		idx := pattern.Index(b, last, d, attacker)
		switch pattern.Lookup(idx).Class {
		case config.PatternClassThree, config.PatternClassBrokenThree:
		default:
			continue
		}
		for i, st := range pattern.Unpack(idx) {
			if st != config.PatternStateEmpty {
				continue
			}
			n = addCell(s.defends[ply][:], n, windowCell(last, g, d, i))
		}
	}
	return n
}

func pickBest(moves []rules.Move, ranks []uint8, i, n int) {
	best := i
	for j := i + 1; j < n; j++ {
		if ranks[j] > ranks[best] {
			best = j
		}
	}
	if best != i {
		moves[i], moves[best] = moves[best], moves[i]
		ranks[i], ranks[best] = ranks[best], ranks[i]
	}
}

func (s *Solver) kindFloor() uint8 {
	if s.kind == KindVCF {
		return config.PatternClassFour
	}
	return config.PatternClassBrokenThree
}
