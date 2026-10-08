package engine

import (
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

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
	RootMoves                 [config.PonderDepthHistory]rules.Move
	RootScores                [config.PonderDepthHistory]int
	RootIters                 int
}

func NpsReport(nodes uint64, elapsedNs int64) uint64 {
	if nodes == 0 {
		return 0
	}
	if elapsedNs < int64(time.Millisecond) {
		elapsedNs = int64(time.Millisecond)
	}
	return nodes * uint64(time.Second) / uint64(elapsedNs)
}
func EBFMilli(nodes uint64, depth int) int {
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
