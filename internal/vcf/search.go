package vcf

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// attack is the OR node: the attacker to move. Its value is a pure function
// of board and side, so completely evaluated results are memoized.
func (s *Solver) attack(b *rules.Board, ply int) result {
	if ply >= s.plyCap || !s.ok() {
		return resAbort
	}
	attacker := b.Side
	if ply > 0 {
		if r, hit := s.ttProbe(b.Hash); hit {
			return r
		}
	}
	if n, first := s.probeWins(b, attacker, s.winA[:]); n > 0 {
		s.line[ply] = rules.Move(first)
		s.plies = ply + 1
		s.ttStore(b.Hash, resWin)
		return resWin
	}
	if b.IsFull() {
		s.ttStore(b.Hash, resFail)
		return resFail
	}
	if nD, defCell := s.probeWins(b, attacker.Opponent(), nil); nD > 0 {
		if nD > 1 {
			s.ttStore(b.Hash, resFail)
			return resFail
		}
		return s.forcedBlock(b, attacker, defCell, ply)
	}
	return s.threatLine(b, attacker, ply)
}

// forcedBlock handles the single defender win cell: the attacker must
// occupy it, or close its far end when one end of the defender's five
// already holds an attacker stone, with a threat move of the solver's
// kind, else the defender fives first.
func (s *Solver) forcedBlock(b *rules.Board, attacker rules.Color, u rules.Cell, ply int) result {
	cands := [2]rules.Cell{u, u}
	n := 1
	defender := attacker.Opponent()
	if br, bc, fr, fc, ok := walkFive(b, defender, u); ok {
		switch {
		case cellAt(b, br, bc, attacker) && playable(b, fr, fc):
			cands[1] = cellOf(fr, fc)
			n = 2
		case cellAt(b, fr, fc, attacker) && playable(b, br, bc):
			cands[1] = cellOf(br, bc)
			n = 2
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
			s.ttStore(b.Hash, resWin)
			return resWin
		}
		if r == resAbort {
			overall = resAbort
		}
	}
	s.ttStore(b.Hash, overall)
	return overall
}

// threatLine tries every screened threat move in class rank order, so open
// four conversions go before simple fours and threes. A depth-aborted move
// does not stop the OR: a later move's complete proof is a valid win.
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
			s.ttStore(b.Hash, resWin)
			return resWin
		}
		if r == resAbort {
			overall = resAbort
		}
	}
	s.ttStore(b.Hash, overall)
	return overall
}

// defend is the AND node: the defender to move right after the attacker's
// threat move last. Defender model, see vcf.md: with live attacker win
// cells the replies are the exact defusing set; after a gated three they
// are the threat window cells plus every counter-four; a defender win in 1
// fails the line outright.
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

// threatMoves screens the attacker's candidate moves: every empty cell in
// the Chebyshev SolverCandidateRadius ring of the attacker's stones whose
// centered post-placement window reaches the kind's class floor.
func (s *Solver) threatMoves(b *rules.Board, attacker rules.Color, ply int) int {
	reach := dirReach(b, attacker)
	floor := s.kindFloor()
	cands := dilate(stones(b, attacker), config.SolverCandidateRadius)
	n := 0
	for w := range cands {
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

// threatReplies assembles the defender model after a gated three: every
// defender move whose centered post-placement window reaches Four (the
// counter-fours that force the attacker back), then every empty cell of
// the threat windows through the last move, the blocks of the three.
// Counter-fours come first: they are the tries most likely to refute, and
// AND nodes exit on the first refutation.
func (s *Solver) threatReplies(b *rules.Board, attacker rules.Color, last rules.Cell, ply int) int {
	defender := attacker.Opponent()
	n := 0
	reach := dirReach(b, defender)
	cands := dilate(stones(b, defender), config.SolverCandidateRadius)
	for w := range cands {
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
	for d := range config.PatternDirs {
		idx := pattern.Index(b, last, d, attacker)
		switch pattern.Lookup(d, idx).Class {
		case config.PatternClassThree, config.PatternClassBrokenThree:
		default:
			continue
		}
		for i, st := range pattern.Unpack(idx) {
			if st != config.PatternStateEmpty {
				continue
			}
			n = addCell(s.defends[ply][:], n, windowCell(last, d, i))
		}
	}
	return n
}

// pickBest swaps the best remaining move into position i by class rank,
// stable on equal ranks, matching the deterministic cell order.
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
