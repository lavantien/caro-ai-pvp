package engine

import (
	"math"
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const ttEntryBytes = 16

const (
	ttBoundExact uint8 = iota + 1
	ttBoundLower
	ttBoundUpper
)

// evalMateScoreMin is the fence of the mate band: scores at or beyond it are
// mate distances, everything below is a milliunit leaf score. Derived from
// the config hub, never a second literal.
const evalMateScoreMin = config.EvalMateMax - config.SearchMaxPly*config.EvalMateScoreStep

// ttMaxDepth keeps the stored depth inside int8 even if a caller passes a
// maxDepth beyond SearchMaxPly through the exported SearchDepth.
const ttMaxDepth = math.MaxInt8

// scoreToTT and scoreFromTT are the single encode/decode pair every stored
// and probed score passes through. In-search mate scores count plies from
// the root, stored scores count plies from the node, so any transposition
// path can reuse the entry. Mate scores step EvalMateScoreStep per ply, so
// the ply adjustment scales by the same step, keeping every stored and
// decoded mate value on the mateWin lattice.
func scoreToTT(score int, ply int) int {
	if score >= evalMateScoreMin {
		return score + ply*config.EvalMateScoreStep
	}
	if score <= -evalMateScoreMin {
		return score - ply*config.EvalMateScoreStep
	}
	return score
}

func scoreFromTT(score int, ply int) int {
	if score >= evalMateScoreMin {
		return score - ply*config.EvalMateScoreStep
	}
	if score <= -evalMateScoreMin {
		return score + ply*config.EvalMateScoreStep
	}
	return score
}

// ttEntry packs into 16 bytes: full zobrist key, node-relative score, best
// move, searched depth, and a gen byte holding the bound in the low 2 bits
// and the search generation in the upper 6.
type ttEntry struct {
	key   uint64
	score int32
	move  uint16
	depth int8
	gen   uint8
}

// ttTable is direct mapped, single threaded here, atomically shared once the
// SMP layer lands. Replacement is depth preferred inside the current
// generation and always replaces entries of older generations; the 6-bit
// generation wraps after 63 searches, which can only skew replacement
// priority, never correctness, since the full 64-bit key still gates every
// hit. Stones only accumulate in this game and no repetition draw exists,
// so entries stay sound across searches of the same instance.
type ttTable struct {
	entries []ttEntry
	mask    uint64
}

func (t *ttTable) init(bytes int64) {
	if bytes < ttEntryBytes {
		return
	}
	n := uint64(bytes) / ttEntryBytes
	n = 1 << (bits.Len64(n) - 1)
	t.entries = make([]ttEntry, n)
	t.mask = n - 1
}

func (t *ttTable) enabled() bool { return t.entries != nil }

func (t *ttTable) probe(hash uint64, depth int, alpha, beta int, ply int) (int, int, bool) {
	if !t.enabled() {
		return 0, -1, false
	}
	e := &t.entries[hash&t.mask]
	move := -1
	if e.key == hash && e.move != uint16(moveNone) {
		move = int(e.move)
	}
	if e.key != hash || e.depth < int8(depth) {
		return 0, move, false
	}
	score := scoreFromTT(int(e.score), ply)
	switch e.gen & 3 {
	case ttBoundExact:
		return score, move, true
	case ttBoundLower:
		if score >= beta {
			return score, move, true
		}
	case ttBoundUpper:
		if score <= alpha {
			return score, move, true
		}
	}
	return 0, move, false
}

func (t *ttTable) move(hash uint64) rules.Move {
	if !t.enabled() {
		return moveNone
	}
	e := &t.entries[hash&t.mask]
	if e.key != hash || e.move == uint16(moveNone) {
		return moveNone
	}
	return rules.Move(e.move)
}

func (t *ttTable) store(hash uint64, score int, move rules.Move, depth int, bound uint8, ply int, gen uint8) {
	if !t.enabled() || hash == 0 {
		return
	}
	if depth > ttMaxDepth {
		depth = ttMaxDepth
	}
	e := &t.entries[hash&t.mask]
	if e.key != 0 && e.gen>>2 == gen && e.depth > int8(depth) {
		return
	}
	e.key = hash
	e.score = int32(scoreToTT(score, ply))
	e.move = uint16(move)
	e.depth = int8(depth)
	e.gen = gen<<2 | bound
}

// hashFullPermille samples SearchHashFullSample evenly spaced slots and
// reports the share holding current generation data.
func (t *ttTable) hashFullPermille(gen uint8) int {
	if !t.enabled() {
		return 0
	}
	n := len(t.entries)
	if n > config.SearchHashFullSample {
		n = config.SearchHashFullSample
	}
	used := 0
	for i := range n {
		e := &t.entries[i*len(t.entries)/n]
		if e.key != 0 && e.gen>>2 == gen {
			used++
		}
	}
	return used * 1000 / n
}
