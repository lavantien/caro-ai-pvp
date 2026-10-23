package pattern

import (
	"math/bits"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

var oracleSquare = geom.Default()

func oracleDir(g *geom.Geometry) [2]int {
	return [2]int{g.Dirs[0].Dr, g.Dirs[0].Dc}
}

func oracleCellFor(g *geom.Geometry, i int) rules.Cell {
	delta := oracleDir(g)
	return genCell(g, g.Rows/2, &delta, i)
}

func oracleCell(i int) rules.Cell {
	return oracleCellFor(oracleSquare, i)
}

func naiveBuildFor(t *testing.T, g *geom.Geometry, w [config.PatternWindowLen]uint8) *rules.NaiveBoard {
	t.Helper()
	nb := rules.NewNaiveBoardFor(g)
	for i, s := range w {
		switch s {
		case config.PatternStateOwn:
			nb.Set(oracleCellFor(g, i), rules.Red)
		case config.PatternStateOpp:
			nb.Set(oracleCellFor(g, i), rules.Blue)
		}
	}
	return nb
}

func naiveBuild(t *testing.T, w [config.PatternWindowLen]uint8) *rules.NaiveBoard {
	t.Helper()
	return naiveBuildFor(t, oracleSquare, w)
}

func naiveWin1For(t *testing.T, g *geom.Geometry, w [config.PatternWindowLen]uint8) uint16 {
	t.Helper()
	var mask uint16
	for i, s := range w {
		if s != config.PatternStateEmpty {
			continue
		}
		probe := naiveBuildFor(t, g, w)
		probe.Set(oracleCellFor(g, i), rules.Red)
		if probe.WinsThrough(rules.Red, oracleCellFor(g, i)) {
			mask |= 1 << uint(i)
		}
	}
	return mask
}

func naiveWin1(t *testing.T, w [config.PatternWindowLen]uint8) uint16 {
	t.Helper()
	return naiveWin1For(t, oracleSquare, w)
}

func withOwn(w [config.PatternWindowLen]uint8, i int) [config.PatternWindowLen]uint8 {
	w[i] = config.PatternStateOwn
	return w
}

func naiveClassFor(t *testing.T, g *geom.Geometry, w [config.PatternWindowLen]uint8) uint8 {
	t.Helper()
	if n1 := bits.OnesCount16(naiveWin1For(t, g, w)); n1 >= 2 {
		return config.PatternClassOpenFour
	} else if n1 == 1 {
		return config.PatternClassFour
	}
	var childN1 [config.PatternWindowLen]uint8
	three, broken := false, false
	for i, s := range w {
		if s != config.PatternStateEmpty {
			continue
		}
		childN1[i] = uint8(bits.OnesCount16(naiveWin1For(t, g, withOwn(w, i))))
		if childN1[i] >= 2 {
			three = true
		} else if childN1[i] == 1 {
			broken = true
		}
	}
	if three {
		return config.PatternClassThree
	}
	if broken {
		return config.PatternClassBrokenThree
	}
	for i, s := range w {
		if s != config.PatternStateEmpty || childN1[i] != 0 {
			continue
		}
		child := withOwn(w, i)
		for j, s2 := range child {
			if s2 != config.PatternStateEmpty {
				continue
			}
			if bits.OnesCount16(naiveWin1For(t, g, withOwn(child, j))) >= 2 {
				return config.PatternClassOpenTwo
			}
		}
	}
	return config.PatternClassNone
}

func naiveClass(t *testing.T, w [config.PatternWindowLen]uint8) uint8 {
	t.Helper()
	return naiveClassFor(t, oracleSquare, w)
}

func win(t *testing.T, s string) [config.PatternWindowLen]uint8 {
	t.Helper()
	if len(s) != config.PatternWindowLen {
		t.Fatalf("witness %q: want %d characters", s, config.PatternWindowLen)
	}
	var w [config.PatternWindowLen]uint8
	for i, ch := range s {
		switch ch {
		case '.':
			w[i] = config.PatternStateEmpty
		case 'o':
			w[i] = config.PatternStateOwn
		case 'x':
			w[i] = config.PatternStateOpp
		case ' ':
			w[i] = config.PatternStateOff
		default:
			t.Fatalf("witness %q: stray character %q", s, ch)
		}
	}
	return w
}
