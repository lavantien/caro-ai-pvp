package engine

import (
	"math"
	"math/bits"
	"sync/atomic"

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

// ttMaxDepth keeps the stored depth inside its byte even if a caller passes
// a maxDepth beyond SearchMaxPly through the exported SearchDepth.
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

// ttTable is the lockless direct-mapped table shared by every worker of one
// instance. One slot is two uint64 words, 16 bytes: the data word and the
// signature word. The data word packs the node-relative score in bits 0..31,
// the best move in 32..47, the searched depth in 48..55, and the gen byte
// (bound in the low 2 bits, generation in the upper 6) in 56..63. The
// signature word holds key ^ data.
//
// Consistency argument, the Hyatt-Mann lockless scheme: a reader loads both
// words with atomic loads and accepts the entry only when signature ^ data
// equals its key. An all-old or all-new pair satisfies that check and is a
// coherent entry. A pair torn by a concurrent writer mixes old signature
// with new data, and oldSig ^ newData can only reproduce the key when
// oldKey ^ oldData ^ newData == newKey ^ newData, that is on a 64-bit key
// collision between the two occupants, the same residual risk any
// zobrist-gated table already accepts. A torn read is therefore seen as a
// miss, never as a wrong hit, without any lock. Writers store data then
// signature; a writer losing a race merely decides which coherent entry
// lands in the slot. Replacement stays depth preferred inside the current
// generation and always replaces older generations; the 6-bit generation
// wraps after 63 searches, which can only skew replacement priority, never
// correctness, since the signature still gates every hit. Stones only
// accumulate in this game and no repetition draw exists, so entries stay
// sound across searches of the same instance. The table belongs to one
// instance and one board kind: the zobrist key does not encode the region.
type ttTable struct {
	entries []uint64
	mask    uint64
	gen     uint8
}

func newTT(bytes int64) *ttTable {
	t := &ttTable{}
	if bytes < ttEntryBytes {
		return t
	}
	n := uint64(bytes) / ttEntryBytes
	n = 1 << (bits.Len64(n) - 1)
	t.entries = make([]uint64, 2*n)
	t.mask = n - 1
	return t
}

func (t *ttTable) enabled() bool { return t.entries != nil }

// bumpGen advances the replacement generation once per search, called before
// workers start so no worker ever races it.
func (t *ttTable) bumpGen() {
	t.gen++
	if t.gen > 63 {
		t.gen = 1
	}
}

func ttScore(data uint64) int { return int(int32(uint32(data))) }

func (t *ttTable) probe(hash uint64, depth int, alpha, beta int, ply int) (int, int, bool) {
	if !t.enabled() {
		return 0, -1, false
	}
	i := (hash & t.mask) * 2
	data := atomic.LoadUint64(&t.entries[i])
	sig := atomic.LoadUint64(&t.entries[i+1])
	if sig^data != hash {
		return 0, -1, false
	}
	move := int(uint16(data >> 32))
	if int(uint8(data>>48)) < depth {
		return 0, move, false
	}
	score := scoreFromTT(ttScore(data), ply)
	switch uint8(data>>56) & 3 {
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
	i := (hash & t.mask) * 2
	data := atomic.LoadUint64(&t.entries[i])
	sig := atomic.LoadUint64(&t.entries[i+1])
	if sig^data != hash {
		return moveNone
	}
	return rules.Move(uint16(data >> 32))
}

func (t *ttTable) store(hash uint64, score int, move rules.Move, depth int, bound uint8, ply int) {
	if !t.enabled() || hash == 0 {
		return
	}
	if depth > ttMaxDepth {
		depth = ttMaxDepth
	}
	i := (hash & t.mask) * 2
	cur := atomic.LoadUint64(&t.entries[i])
	if cur != 0 && uint8(cur>>56)>>2 == t.gen && int(uint8(cur>>48)) > depth {
		return
	}
	data := uint64(uint32(scoreToTT(score, ply))) |
		uint64(move)<<32 |
		uint64(uint8(depth))<<48 |
		uint64(t.gen<<2|bound)<<56
	atomic.StoreUint64(&t.entries[i], data)
	atomic.StoreUint64(&t.entries[i+1], hash^data)
}

// hashFullPermille samples SearchHashFullSample evenly spaced slots and
// reports the share holding current generation data.
func (t *ttTable) hashFullPermille() int {
	if !t.enabled() {
		return 0
	}
	slots := len(t.entries) / 2
	n := slots
	if n > config.SearchHashFullSample {
		n = config.SearchHashFullSample
	}
	used := 0
	for i := range n {
		data := atomic.LoadUint64(&t.entries[2*(i*slots/n)])
		if data != 0 && uint8(data>>56)>>2 == t.gen {
			used++
		}
	}
	return used * 1000 / n
}
