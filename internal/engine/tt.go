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

const ttMaxDepth = math.MaxInt8

func scoreToTT(score int, ply int) int {
	if score >= config.EvalMateFloor {
		return score + ply*config.EvalMateScoreStep
	}
	if score <= -config.EvalMateFloor {
		return score - ply*config.EvalMateScoreStep
	}
	return score
}

func scoreFromTT(score int, ply int) int {
	if score >= config.EvalMateFloor {
		return score - ply*config.EvalMateScoreStep
	}
	if score <= -config.EvalMateFloor {
		return score + ply*config.EvalMateScoreStep
	}
	return score
}

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
	if uint8(data>>56)>>2 != t.gen && (score >= config.EvalMateFloor || score <= -config.EvalMateFloor) {
		return 0, move, false
	}
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
