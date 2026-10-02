package server

import (
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// The M-line emitter renders one engine.SearchStats into the Implication 1.5
// bot-log line. config.BotLogFormat owns the byte-level shape; this file
// rebuilds it with strconv appends only, and TestMLineMatchesBotLogFormat
// holds the two definitions identical so they cannot drift apart.

// Compact-number scales for n= and nps=: k below a million, m at and above.
// The spec's goldens show no billion scale, so counts past 1e9 stay on m with
// a wider mantissa (2500m) instead of inventing a suffix the format never
// displays.
const (
	compactKilo = 1_000
	compactMega = 1_000_000
)

// mLineCap sizes MLine's buffer: the fixed fields plus one 4-byte cell name
// per PV slot, P16 being the widest coordinate the 16x16 board can name.
const mLineCap = 160 + config.SearchMaxPly*4

// AppendMLine appends the Implication 1.5 bot-log line of one completed bot
// move to dst and returns the extended slice. The result is byte-identical
// to fmt.Sprintf(config.BotLogFormat, moveNumber, side name, move name,
// depth, nodes, nps, ebf, tt, hf, fh1, score, threads, t, alloc, tag, pv)
// with every field derived from st, but built from strconv appends so a
// caller-sized dst keeps the hot path allocation free. tag is one of
// config.BotLogTagVCF, config.BotLogTagVCT, config.BotLogTagPonder, or the
// empty string for an untagged line. An empty PV leaves a bare trailing
// "pv=" per the format's final %s.
func AppendMLine(dst []byte, moveNumber int, side rules.Color, move rules.Move, st *engine.SearchStats, tag string) []byte {
	dst = append(dst, 'M')
	dst = strconv.AppendInt(dst, int64(moveNumber), 10)
	dst = append(dst, ", "...)
	dst = append(dst, sideName(side)...)
	dst = append(dst, ", "...)
	dst = appendCellName(dst, move)
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

// MLine is the allocating wrapper of AppendMLine for cold paths and tests;
// the realtime emitter reuses a buffer through AppendMLine instead.
func MLine(moveNumber int, side rules.Color, move rules.Move, st *engine.SearchStats, tag string) string {
	return string(AppendMLine(make([]byte, 0, mLineCap), moveNumber, side, move, st, tag))
}

// sideName gives the two sides the display names of the Implication 1.5
// goldens; rules.Color is a bare uint8 with no String method.
func sideName(c rules.Color) string {
	if c == rules.Blue {
		return "Blue"
	}
	return "Red"
}

// appendCellName names a move in the rules codec's notation, column letter
// plus 1-based row, mirroring rules.CellName without its allocation. The
// emitter only names engine-produced legal moves, so the codec's range error
// cannot fire here; TestStatsCellNameMatchesCodec walks every board cell to
// hold the mirror to the canonical implementation.
func appendCellName(dst []byte, m rules.Move) []byte {
	dst = append(dst, byte('A'+m%config.BoardStride))
	return strconv.AppendUint(dst, uint64(m/config.BoardStride)+1, 10)
}

// appendCompact renders n the way the Implication 1.5 goldens write node
// counts and node rates (6.25m, 2.5m, 45k, 18m, 1.2m): plain digits below a
// thousand, then a mantissa of at most three significant digits, that is two
// decimals, with trailing zeros trimmed, suffixed k below a million and m at
// and above. The second decimal rounds half up, so 1.2m means [1.195m,
// 1.205m) and 1_050_000 renders 1.05m. A k-scale mantissa that rounds up to
// 1000 promotes to 1m (999_995 -> 1m); on the m scale the same rounding
// widens the mantissa instead (999_995_000 -> 1000m) because no higher
// scale exists in the spec.
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

// compactParts splits n over scale into a whole mantissa and a hundredths
// fraction rounded half up, carrying a fraction of 100 into the whole part.
func compactParts(n, scale uint64) (whole, frac uint64) {
	whole = n / scale
	frac = (n%scale*100 + scale/2) / scale
	if frac >= 100 {
		whole++
		frac = 0
	}
	return whole, frac
}

// appendScore renders s=: scores strictly inside the mate band print as
// signed integer milliunits (+150, -25; zero keeps the plus so the sign
// column never varies), mate scores as M-distance over the engine's
// mateWin lattice. The drivers return mateWin(ply) = EvalMateMax -
// (ply+1)*EvalMateScoreStep when the side that just moved at ply wins on
// that stone (searchRoot: mateWin(0), negamax: mateWin(ply)), so the winner
// is the (ply+1)-th move from the mover's root and the distance shown is
// (EvalMateMax - |score|) / EvalMateScoreStep. Mate detection uses the
// engine's own band floor, EvalMateMax - SearchMaxPly*EvalMateScoreStep
// (engine's evalMateScoreMin): the early-break threshold of both drivers,
// EvalMateMax - EvalMateScoreStep = mateWin(0), is the band's near edge and
// renders M1, the mover's own winning stone. Negated lattice points mirror
// to -M-distance, mate against the mover.
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

// permillePercent converts a permille field (0..1000) to the integer percent
// the goldens display, rounding half up so the display error stays within
// half a percent (the goldens 380, 450, 930 divide exactly and are
// unaffected).
func permillePercent(permille int) int {
	return (permille + 5) / 10
}
