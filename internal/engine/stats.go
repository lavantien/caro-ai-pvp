package engine

import (
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// SearchStats carries one search's Implication 1.5 counters: depth, nodes,
// nodes per second, effective branching factor in milliunits (1500 = 1.5),
// transposition hit rate, hash full, first move fail high rate, all in
// permille where fractional, plus score, thread count, elapsed and granted
// budget in nanoseconds, and the principal variation. The M-line assembly
// itself belongs to the stats pipeline, not the engine.
type SearchStats struct {
	Depth                     int
	Nodes                     uint64
	Nps                       uint64
	EBFMilli                  int
	TTHitPermille             int
	HashFullPermille          int
	FirstMoveFailHighPermille int
	Score                     int
	Threads                   int
	ElapsedNs                 int64
	AllocNs                   int64
	PVLen                     int
	PV                        [config.SearchMaxPly]rules.Move
}

// npsReport converts a node count and elapsed nanoseconds into nodes per
// second, flooring a sub-nanosecond elapsed to 1ns so an instant search
// still reports a rate instead of a zero from clock granularity.
func npsReport(nodes uint64, elapsedNs int64) uint64 {
	if nodes == 0 {
		return 0
	}
	if elapsedNs < 1 {
		elapsedNs = 1
	}
	return nodes * uint64(time.Second) / uint64(elapsedNs)
}

// ebfMilli estimates the effective branching factor as the integer b, in
// milliunits, whose geometric node sum to the reached depth fits the node
// count. Integer binary search, outside the hot loop.
func ebfMilli(nodes uint64, depth int) int {
	if depth < 1 || nodes < 2 {
		return 0
	}
	lo, hi := 1000, 32000
	for lo < hi {
		mid := int(uint(lo+hi+1) >> 1)
		if geoSum(mid, depth) <= nodes {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// geoSum sums 1 + b + b^2 + ... + b^depth with b in milliunits, saturating
// instead of overflowing so the binary search can compare safely. The term
// guard subsumes the total: every admitted term is at most 2^62/1000 and at
// most SearchMaxPly terms accumulate.
func geoSum(bMilli int, depth int) uint64 {
	total, term := uint64(1), uint64(1)
	bm := uint64(bMilli)
	for range depth {
		if term > 1<<62/bm {
			return 1 << 62
		}
		term = term * bm / 1000
		total += term
	}
	return total
}

// AppendPV appends the principal variation in cell notation to dst with
// strconv only. The caller sized dst (PVLen * 4 bytes covers P16), so no
// allocation happens.
func (s *SearchStats) AppendPV(dst []byte) []byte {
	for i := 0; i < s.PVLen && i < len(s.PV); i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = append(dst, byte('A'+s.PV[i]%config.BoardStride))
		dst = strconv.AppendInt(dst, int64(s.PV[i]/config.BoardStride)+1, 10)
	}
	return dst
}
