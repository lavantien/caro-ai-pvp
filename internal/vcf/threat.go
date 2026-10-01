package vcf

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const wordBits = 64

// windowHalf is the center offset inside a packed window; windowCenterShift
// is its bit offset in the packed index. A candidate square always sits at
// the window center, per the taxonomy's center-bit contract.
const (
	windowHalf        = (config.PatternWindowLen - 1) / 2
	windowCenterShift = config.PatternCellBits * windowHalf
)

type bb [config.BoardWordsPerColor]uint64

var fileMaskFirst, fileMaskLast uint64

func init() {
	for row := range config.BoardWordsPerColor {
		fileMaskFirst |= 1 << (row * config.BoardStride)
		fileMaskLast |= 1 << (row*config.BoardStride + config.BoardStride - 1)
	}
}

func stones(b *rules.Board, mover rules.Color) bb {
	if mover == rules.Red {
		return bb(b.Red)
	}
	return bb(b.Blue)
}

func cellOf(r, c int) rules.Cell { return rules.Cell(r*config.BoardStride + c) }

// fwdClear is the file mask of wrapped bits after shifting one cell along a
// direction with column delta dc.
func fwdClear(dc int) uint64 {
	switch dc {
	case 1:
		return fileMaskFirst
	case -1:
		return fileMaskLast
	}
	return 0
}

// shiftLine steps a bitboard one cell along a direction, forward or
// backward, with cross-word carries and file clears so lines never wrap
// into neighbor columns.
func shiftLine(s bb, dir int, forward bool) bb {
	dr, dc := config.PatternDirs[dir][0], config.PatternDirs[dir][1]
	step := dr*config.BoardStride + dc
	clear := fwdClear(dc)
	if !forward {
		step, clear = -step, fwdClear(-dc)
	}
	var r bb
	if step >= 0 {
		us := uint(step)
		for w := config.BoardWordsPerColor - 1; w >= 0; w-- {
			r[w] = s[w] << us
			if w > 0 {
				r[w] |= s[w-1] >> (wordBits - us)
			}
			r[w] &^= clear
		}
		return r
	}
	us := uint(-step)
	for w := range config.BoardWordsPerColor {
		r[w] = s[w] >> us
		if w < config.BoardWordsPerColor-1 {
			r[w] |= s[w+1] << (wordBits - us)
		}
		r[w] &^= clear
	}
	return r
}

func orBB(a, b bb) bb {
	for w := range a {
		a[w] |= b[w]
	}
	return a
}

// dirReach returns, per direction, the cells holding at least one own stone
// within the window line through them: a one-bit pre-filter before the full
// window lookup, since every threat class needs own stones in the window.
func dirReach(b *rules.Board, mover rules.Color) [config.PatternDirections]bb {
	var out [config.PatternDirections]bb
	for d := range config.PatternDirs {
		up, down := stones(b, mover), stones(b, mover)
		m := up
		for range windowHalf {
			up = shiftLine(up, d, true)
			down = shiftLine(down, d, false)
			m = orBB(m, orBB(up, down))
		}
		out[d] = m
	}
	return out
}

// dilate grows a stone set by radius Chebyshev rings; Chebyshev dilation
// composes, so radius r is r single steps.
func dilate(s bb, radius int) bb {
	for range radius {
		var vert, out bb
		for w := range s {
			up := s[w] >> config.BoardStride
			if w+1 < len(s) {
				up |= s[w+1] << (wordBits - config.BoardStride)
			}
			down := s[w] << config.BoardStride
			if w > 0 {
				down |= s[w-1] >> (wordBits - config.BoardStride)
			}
			vert[w] = up | down
			out[w] = s[w] | vert[w] | (s[w]>>1)&^fileMaskLast | (s[w]<<1)&^fileMaskFirst |
				(vert[w]>>1)&^fileMaskLast | (vert[w]<<1)&^fileMaskFirst
		}
		s = out
	}
	return s
}

// threatRank classifies placing the mover's stone at cell: the best class,
// over the four directions, of the post-placement window centered on the
// cell. Window classes are over-approximations of the real board; the
// search verifies everything that matters through rules.FastLastMoveWin.
func threatRank(b *rules.Board, cell rules.Cell, mover rules.Color, reach *[config.PatternDirections]bb) uint8 {
	best := config.PatternClassNone
	for d := range config.PatternDirs {
		w, m := int(cell)/wordBits, uint64(1)<<(uint(cell)%wordBits)
		if reach[d][w]&m == 0 {
			continue
		}
		idx := pattern.Index(b, cell, d, mover)
		cls := pattern.Lookup(d, idx|uint32(config.PatternStateOwn)<<windowCenterShift).Class
		if cls > best {
			best = cls
		}
	}
	return best
}

// threatScore packs a threat move ordering key: class rank in the high
// bits, then the number of directions reaching the floor, so double
// threats through one square come before single line threats of the same
// class.
func (s *Solver) threatScore(b *rules.Board, cell rules.Cell, mover rules.Color, reach *[config.PatternDirections]bb) uint8 {
	floor := s.kindFloor()
	rank, dirs := config.PatternClassNone, 0
	for d := range config.PatternDirs {
		w, m := int(cell)/wordBits, uint64(1)<<(uint(cell)%wordBits)
		if reach[d][w]&m == 0 {
			continue
		}
		idx := pattern.Index(b, cell, d, mover)
		cls := pattern.Lookup(d, idx|uint32(config.PatternStateOwn)<<windowCenterShift).Class
		if cls > rank {
			rank = cls
		}
		if cls >= floor {
			dirs++
		}
	}
	return rank<<2 | uint8(min(dirs, 3))
}

// probeWins finds every empty in-region cell where mover placing a stone
// wins right now, each verified on the real board at move time. out stores
// up to its length cells, the count is real either way.
func (s *Solver) probeWins(b *rules.Board, mover rules.Color, out []rules.Cell) (int, rules.Cell) {
	n := 0
	var first rules.Cell
	cands := dilate(stones(b, mover), config.SolverCandidateRadius)
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

// cellAt reports a stone of the color at row, col; off-board never matches.
func cellAt(b *rules.Board, r, c int, color rules.Color) bool {
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

// playable reports an empty in-region cell.
func playable(b *rules.Board, r, c int) bool {
	if r < 0 || r >= config.BoardSize || c < 0 || c >= config.BoardSize {
		return false
	}
	cell := r*config.BoardStride + c
	w, m := cell/wordBits, uint64(1)<<(uint(cell)%wordBits)
	return b.Region[w]&m != 0 && b.Full[w]&m == 0
}

// windowCell maps window position i of the line through cell to a board
// cell. Callers only ask for positions the window held Empty.
func windowCell(cell rules.Cell, dir, i int) rules.Cell {
	k := i - windowHalf
	dr, dc := config.PatternDirs[dir][0], config.PatternDirs[dir][1]
	r := int(cell)/config.BoardStride + k*dr
	c := int(cell)%config.BoardStride + k*dc
	return cellOf(r, c)
}

// fiveFrame holds the two neighbor cells one step past an exact five run:
// before (br,bc) behind the run, after (fr,fc) past its front.
type fiveFrame struct{ br, bc, fr, fc int }

// fiveFrames reports every direction whose maximal contiguous run of color
// stones through the empty cell is exactly WinLength, at most one frame per
// direction, writing into out and returning the real count. A frame with
// both neighbors holding the opponent's stones is dead and never a win
// witness, so callers reading far ends only ever see frames whose one
// defender end leaves the other end playable.
func fiveFrames(b *rules.Board, color rules.Color, cell rules.Cell, out []fiveFrame) int {
	r, c := int(cell)/config.BoardStride, int(cell)%config.BoardStride
	n := 0
	for d := range config.PatternDirs {
		dr, dc := config.PatternDirs[d][0], config.PatternDirs[d][1]
		back := 0
		xr, xc := r-dr, c-dc
		for back < config.WinLength && cellAt(b, xr, xc, color) {
			back++
			xr -= dr
			xc -= dc
		}
		fwd := 0
		yr, yc := r+dr, c+dc
		for fwd < config.WinLength && cellAt(b, yr, yc, color) {
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

// defusing fills the defender's exact defusing set for the attacker win
// cells s.winA[:nA]: each win cell itself, plus the opposite end of every
// winning line whose one end already holds a defender stone. One win cell
// can complete exact fives in several directions at once, so every frame
// through it contributes; dead frames (both ends defender stones) pass
// neither playability test and add nothing. Under the exact-5 both-ends
// rule that far end closes the five, a defense gomoku does not have. Every
// other defender move is dominated: it cannot five (W_D was empty) and
// cannot defuse, so the attacker completes at a live win cell.
func (s *Solver) defusing(b *rules.Board, attacker rules.Color, nA, ply int) int {
	defender := attacker.Opponent()
	n := 0
	var frames [config.PatternDirections]fiveFrame
	for i := 0; i < nA; i++ {
		w := s.winA[i]
		n = addCell(s.defends[ply][:], n, w)
		for j, nf := 0, fiveFrames(b, attacker, w, frames[:]); j < nf; j++ {
			f := frames[j]
			if cellAt(b, f.br, f.bc, defender) && playable(b, f.fr, f.fc) {
				n = addCell(s.defends[ply][:], n, cellOf(f.fr, f.fc))
			}
			if cellAt(b, f.fr, f.fc, defender) && playable(b, f.br, f.bc) {
				n = addCell(s.defends[ply][:], n, cellOf(f.br, f.bc))
			}
		}
	}
	return n
}

// addCell appends a cell to the reply buffer unless already present.
func addCell(buf []rules.Cell, n int, c rules.Cell) int {
	for i := 0; i < n; i++ {
		if buf[i] == c {
			return n
		}
	}
	buf[n] = c
	return n + 1
}

// realThree gates a screened three-move before the defender is treated as
// forced: some in-window conversion must yield a win cell that verifies on
// the real board, else the window threat is phantom and the line fails.
func realThree(b *rules.Board, attacker rules.Color, last rules.Cell) bool {
	for d := range config.PatternDirs {
		idx := pattern.Index(b, last, d, attacker)
		switch pattern.Lookup(d, idx).Class {
		case config.PatternClassThree, config.PatternClassBrokenThree:
		default:
			continue
		}
		w := pattern.Unpack(idx)
		for i, st := range w {
			if st != config.PatternStateEmpty {
				continue
			}
			child := pattern.Lookup(d, idx|uint32(config.PatternStateOwn)<<(uint(i)*config.PatternCellBits)).Win1
			if child == 0 {
				continue
			}
			a := windowCell(last, d, i)
			for j := range config.PatternWindowLen {
				if child&(uint16(1)<<uint(j)) == 0 {
					continue
				}
				v := windowCell(last, d, j)
				cur := b.Side
				b.Side = attacker
				b.Make(a)
				b.Side = attacker
				b.Make(v)
				won := b.FastLastMoveWin(attacker, v)
				b.Unmake()
				b.Unmake()
				b.Side = cur
				if won {
					return true
				}
			}
		}
	}
	return false
}
