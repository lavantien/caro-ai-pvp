package server

import (
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const (
	compactKilo = 1_000
	compactMega = 1_000_000
)

const mLineCap = 160 + config.SearchMaxPly*4

func AppendMLine(dst []byte, g *geom.Geometry, moveNumber int, side rules.Color, move rules.Move, st *engine.SearchStats, tag string) []byte {
	dst = append(dst, 'M')
	dst = strconv.AppendInt(dst, int64(moveNumber), 10)
	dst = append(dst, ", "...)
	dst = append(dst, sideName(side)...)
	dst = append(dst, ", "...)
	dst = g.AppendName(dst, int(move))
	dst = append(dst, ", d="...)
	dst = strconv.AppendInt(dst, int64(st.Depth), 10)
	dst = append(dst, ", n="...)
	dst = appendCompact(dst, st.Nodes)
	dst = append(dst, ", nps="...)
	dst = appendCompact(dst, st.Nps)
	dst = append(dst, ", ebf="...)
	dst = strconv.AppendFloat(dst, float64(st.EBFMilli)/config.EvalMilliUnit, 'f', 1, 64)
	dst = append(dst, ", tt="...)
	dst = strconv.AppendInt(dst, int64(permillePercent(st.TTHitPermille)), 10)
	dst = append(dst, "%, hf="...)
	dst = strconv.AppendInt(dst, int64(permillePercent(st.HashFullPermille)), 10)
	dst = append(dst, "%, fh1="...)
	dst = strconv.AppendInt(dst, int64(permillePercent(st.FirstMoveFailHighPermille)), 10)
	dst = append(dst, "%, s="...)
	dst = appendScore(dst, st.Score)
	dst = append(dst, ", thr="...)
	dst = strconv.AppendInt(dst, int64(st.Threads), 10)
	dst = append(dst, ", t="...)
	dst = strconv.AppendFloat(dst, float64(st.ElapsedNs)/float64(time.Second), 'f', 2, 64)
	dst = append(dst, ", alloc="...)
	dst = strconv.AppendFloat(dst, float64(st.AllocNs)/float64(time.Second), 'f', 2, 64)
	dst = append(dst, tag...)
	dst = append(dst, ", pv="...)
	return st.AppendPV(dst)
}

func MLine(moveNumber int, side rules.Color, move rules.Move, st *engine.SearchStats, tag string) string {
	return string(AppendMLine(make([]byte, 0, mLineCap), geom.Default(), moveNumber, side, move, st, tag))
}

func sideName(c rules.Color) string {
	if c == rules.Blue {
		return "Blue"
	}
	return "Red"
}

func appendCellName(dst []byte, m rules.Move) []byte {
	return rules.AppendCellName(dst, rules.Cell(m))
}

func appendCompact(dst []byte, n uint64) []byte {
	if n < compactKilo {
		return strconv.AppendUint(dst, n, 10)
	}
	scale, unit := uint64(compactKilo), byte('k')
	if n >= compactMega {
		scale, unit = compactMega, 'm'
	}
	whole, frac := compactParts(n, scale)
	if whole >= compactKilo && scale == compactKilo {
		whole, frac = compactParts(n, compactMega)
		unit = 'm'
	}
	dst = strconv.AppendUint(dst, whole, 10)
	if frac != 0 {
		dst = append(dst, '.')
		if frac%10 == 0 {
			frac /= 10
		} else if frac < 10 {
			dst = append(dst, '0')
		}
		dst = strconv.AppendUint(dst, frac, 10)
	}
	return append(dst, unit)
}

func compactParts(n, scale uint64) (whole, frac uint64) {
	whole = n / scale
	frac = (n%scale*100 + scale/2) / scale
	if frac >= 100 {
		whole++
		frac = 0
	}
	return whole, frac
}

func appendScore(dst []byte, score int) []byte {
	floor := config.EvalMateMax - config.SearchMaxPly*config.EvalMateScoreStep
	switch {
	case score >= floor:
		dst = append(dst, 'M')
		return strconv.AppendInt(dst, int64((config.EvalMateMax-score)/config.EvalMateScoreStep), 10)
	case score <= -floor:
		dst = append(dst, '-', 'M')
		return strconv.AppendInt(dst, int64((config.EvalMateMax+score)/config.EvalMateScoreStep), 10)
	}
	if score >= 0 {
		dst = append(dst, '+')
	}
	return strconv.AppendInt(dst, int64(score), 10)
}

func permillePercent(permille int) int {
	return (permille + 5) / 10
}
