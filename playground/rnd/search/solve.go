package main

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type res uint8

const (
	resFail res = iota
	resWin
	resAbort
)

type orderMode uint8

const (
	orderShipped orderMode = iota
	orderWaveTie
	orderWaveFirst
	orderPrune
)

type solver struct {
	maxPly     int
	plyCap     int
	moves      [solveMaxPly][config.BoardCells]rules.Move
	ranks      [solveMaxPly][config.BoardCells]uint16
	defends    [solveMaxPly][config.BoardCells]rules.Cell
	line       [solveMaxPly]rules.Move
	winA       [maxWinCells]rules.Cell
	ttKeys     [1 << solverTTBits]uint64
	ttVerdicts [1 << solverTTBits]uint8
	nodes      int
	budget     int
	plies      int
	aborted    bool
	order      orderMode
	pruneLimit uint8
	wave       []uint8
}

const maxWinCells = 48

func newSolver() *solver {
	return &solver{maxPly: solveMaxPly}
}

func (s *solver) ok() bool {
	if s.aborted {
		return false
	}
	s.nodes++
	if s.nodes > s.budget {
		s.aborted = true
		return false
	}
	return true
}

func (s *solver) ttProbe(hash uint64) (res, bool) {
	slot := hash & uint64(len(s.ttKeys)-1)
	if s.ttVerdicts[slot] != 0 && s.ttKeys[slot] == hash {
		return resFail, true
	}
	return resFail, false
}

func (s *solver) ttStore(hash uint64, r res) {
	if r != resFail {
		return
	}
	slot := hash & uint64(len(s.ttKeys)-1)
	s.ttKeys[slot] = hash
	s.ttVerdicts[slot] = 2
}

func (s *solver) solve(b *rules.Board, budget int, order orderMode, pruneLimit uint8, wave []uint8) (bool, int, int) {
	s.nodes, s.budget = 0, budget
	s.plies, s.aborted = 0, false
	s.order = order
	s.pruneLimit = pruneLimit
	s.wave = wave
	found := false
	for depth := 1; depth <= s.maxPly && !s.aborted; depth += 2 {
		s.plyCap = depth
		if s.attack(b, 0) == resWin {
			found = true
			break
		}
	}
	clear(s.ttKeys[:])
	clear(s.ttVerdicts[:])
	return found, s.nodes, s.plies
}

func (s *solver) attack(b *rules.Board, ply int) res {
	if ply >= s.plyCap || !s.ok() {
		return resAbort
	}
	attacker := b.Side
	if ply > 0 {
		if r, hit := s.ttProbe(b.Hash); hit {
			return r
		}
	}
	if n, first := s.probeWins(b, attacker); n > 0 {
		s.line[ply] = rules.Move(first)
		s.plies = ply + 1
		return resWin
	}
	if b.IsFull() {
		s.ttStore(b.Hash, resFail)
		return resFail
	}
	if nD, defCell := s.probeWinsCount(b, attacker.Opponent()); nD > 0 {
		if nD > 1 {
			s.ttStore(b.Hash, resFail)
			return resFail
		}
		return s.forcedBlock(b, attacker, defCell, ply)
	}
	return s.threatLine(b, attacker, ply)
}

func (s *solver) forcedBlock(b *rules.Board, attacker rules.Color, u rules.Cell, ply int) res {
	var cands [1 + config.PatternDirections]rules.Cell
	cands[0] = u
	n := 1
	defender := attacker.Opponent()
	var frames [config.PatternDirections]fiveFrame
	for j, nf := 0, fiveFramesThrough(b, defender, u, frames[:]); j < nf; j++ {
		f := frames[j]
		switch {
		case cellHas(b, f.br, f.bc, attacker) && playableRC(b, f.fr, f.fc):
			n = addCellUnique(cands[:], n, cellOfRC(f.fr, f.fc))
		case cellHas(b, f.fr, f.fc, attacker) && playableRC(b, f.br, f.bc):
			n = addCellUnique(cands[:], n, cellOfRC(f.br, f.bc))
		}
	}
	reach := dirReachBB(b, attacker)
	overall := resFail
	for i := 0; i < n; i++ {
		if threatRank(b, cands[i], attacker, &reach) < config.PatternClassFour {
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
	s.ttStore(b.Hash, overall)
	return overall
}

func (s *solver) threatLine(b *rules.Board, attacker rules.Color, ply int) res {
	n := s.threatMoves(b, attacker, ply)
	overall := resFail
	for i := 0; i < n; i++ {
		pickBestKey(s.moves[ply][:], s.ranks[ply][:], i, n)
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
	s.ttStore(b.Hash, overall)
	return overall
}

func (s *solver) defend(b *rules.Board, last rules.Cell, ply int) res {
	if ply >= s.plyCap || !s.ok() {
		return resAbort
	}
	undecided := false
	attacker := b.Side.Opponent()
	if n, _ := s.probeWinsCount(b, b.Side); n > 0 {
		return resFail
	}
	if nA, _ := s.probeWinsInto(b, attacker, s.winA[:]); nA > 0 {
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
	return resFail
}

func (s *solver) threatMoves(b *rules.Board, attacker rules.Color, ply int) int {
	reach := dirReachBB(b, attacker)
	cands := dilateBB(stonesOf(b, attacker), candRadiusSolver)
	n := 0
	for w := range cands {
		free := cands[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			c := rules.Cell(w*wordBits + bits.TrailingZeros64(bit))
			if s.order == orderPrune && s.wave[c] > s.pruneLimit {
				continue
			}
			score := threatScore(b, c, attacker, &reach, config.PatternClassFour)
			if score < config.PatternClassFour<<2 {
				continue
			}
			s.moves[ply][n] = rules.Move(c)
			s.ranks[ply][n] = s.orderKey(score, c)
			n++
		}
	}
	return n
}

func (s *solver) orderKey(score uint8, c rules.Cell) uint16 {
	switch s.order {
	case orderWaveTie:
		return uint16(score)<<8 | uint16(waveUnreachable-s.wave[c])
	case orderWaveFirst:
		return uint16(waveUnreachable-s.wave[c])<<4 | uint16(score)
	default:
		return uint16(score)
	}
}

func pickBestKey(moves []rules.Move, ranks []uint16, i, n int) {
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

func (s *solver) probeWins(b *rules.Board, mover rules.Color) (int, rules.Cell) {
	return s.probeWinsInto(b, mover, s.winA[:])
}

func (s *solver) probeWinsCount(b *rules.Board, mover rules.Color) (int, rules.Cell) {
	return s.probeWinsInto(b, mover, nil)
}

func (s *solver) probeWinsInto(b *rules.Board, mover rules.Color, out []rules.Cell) (int, rules.Cell) {
	n := 0
	var first rules.Cell
	cands := dilateBB(stonesOf(b, mover), candRadiusSolver)
	cur := b.Side
	b.Side = mover
	for w := range cands {
		free := cands[w] & b.Region[w] &^ b.Full[w]
		for free != 0 {
			bit := free & (^free + 1)
			free ^= bit
			c := rules.Cell(w*wordBits + bits.TrailingZeros64(bit))
			b.Make(c)
			won := b.FastLastMoveWin(mover, c)
			b.Unmake()
			if won {
				if n == 0 {
					first = c
				}
				if n < len(out) {
					out[n] = c
				}
				n++
			}
		}
	}
	b.Side = cur
	return n, first
}

type fiveFrame struct{ br, bc, fr, fc int }

func cellHas(b *rules.Board, r, c int, color rules.Color) bool {
	if r < 0 || r >= config.BoardSize || c < 0 || c >= config.BoardSize {
		return false
	}
	s := &b.Red
	if color == rules.Blue {
		s = &b.Blue
	}
	cell := r*config.BoardStride + c
	return s[cell/wordBits]&(1<<(uint(cell)%wordBits)) != 0
}

func playableRC(b *rules.Board, r, c int) bool {
	if r < 0 || r >= config.BoardSize || c < 0 || c >= config.BoardSize {
		return false
	}
	cell := r*config.BoardStride + c
	w, m := cell/wordBits, uint64(1)<<(uint(cell)%wordBits)
	return b.Region[w]&m != 0 && b.Full[w]&m == 0
}

func fiveFramesThrough(b *rules.Board, color rules.Color, cell rules.Cell, out []fiveFrame) int {
	r, c := int(cell)/config.BoardStride, int(cell)%config.BoardStride
	n := 0
	for d := range config.PatternDirs {
		dr, dc := config.PatternDirs[d][0], config.PatternDirs[d][1]
		back := 0
		xr, xc := r-dr, c-dc
		for back < config.WinLength && cellHas(b, xr, xc, color) {
			back++
			xr -= dr
			xc -= dc
		}
		fwd := 0
		yr, yc := r+dr, c+dc
		for fwd < config.WinLength && cellHas(b, yr, yc, color) {
			fwd++
			yr += dr
			yc += dc
		}
		if back+fwd+1 == config.WinLength {
			if n < len(out) {
				out[n] = fiveFrame{xr, xc, yr, yc}
			}
			n++
		}
	}
	return n
}

func (s *solver) defusing(b *rules.Board, attacker rules.Color, nA, ply int) int {
	defender := attacker.Opponent()
	n := 0
	var frames [config.PatternDirections]fiveFrame
	for i := 0; i < nA; i++ {
		w := s.winA[i]
		n = addCellUnique(s.defends[ply][:], n, w)
		for j, nf := 0, fiveFramesThrough(b, attacker, w, frames[:]); j < nf; j++ {
			f := frames[j]
			if cellHas(b, f.br, f.bc, defender) && playableRC(b, f.fr, f.fc) {
				n = addCellUnique(s.defends[ply][:], n, cellOfRC(f.fr, f.fc))
			}
			if cellHas(b, f.fr, f.fc, defender) && playableRC(b, f.br, f.bc) {
				n = addCellUnique(s.defends[ply][:], n, cellOfRC(f.br, f.bc))
			}
		}
	}
	return n
}

func addCellUnique(buf []rules.Cell, n int, c rules.Cell) int {
	for i := 0; i < n; i++ {
		if buf[i] == c {
			return n
		}
	}
	buf[n] = c
	return n + 1
}
