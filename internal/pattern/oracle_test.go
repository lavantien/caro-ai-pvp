package pattern

import (
	"math/bits"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Oracle path: independent recomputation of win-in-1 and the classification
// chain through rules.NaiveBoard, sharing no code with the generator. The
// emulation geometry is the horizontal anchor line, which doubles as a check
// of direction freeness for the other direction tables.

func oracleCell(i int) rules.Cell {
	return genCell(config.BoardSize/2, &config.PatternDirs[0], i)
}

func naiveBuild(t *testing.T, w [config.PatternWindowLen]uint8) *rules.NaiveBoard {
	t.Helper()
	nb := rules.NewNaiveBoard()
	for i, s := range w {
		switch s {
		case config.PatternStateOwn:
			nb.Set(oracleCell(i), rules.Red)
		case config.PatternStateOpp:
			nb.Set(oracleCell(i), rules.Blue)
		}
	}
	return nb
}

func naiveWin1(t *testing.T, w [config.PatternWindowLen]uint8) uint16 {
	t.Helper()
	var mask uint16
	for i, s := range w {
		if s != config.PatternStateEmpty {
			continue
		}
		probe := naiveBuild(t, w)
		probe.Set(oracleCell(i), rules.Red)
		if probe.WinsThrough(rules.Red, oracleCell(i)) {
			mask |= 1 << uint(i)
		}
	}
	return mask
}

func withOwn(w [config.PatternWindowLen]uint8, i int) [config.PatternWindowLen]uint8 {
	w[i] = config.PatternStateOwn
	return w
}

func naiveClass(t *testing.T, w [config.PatternWindowLen]uint8) uint8 {
	t.Helper()
	if n1 := bits.OnesCount16(naiveWin1(t, w)); n1 >= 2 {
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
		childN1[i] = uint8(bits.OnesCount16(naiveWin1(t, withOwn(w, i))))
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
			if bits.OnesCount16(naiveWin1(t, withOwn(child, j))) >= 2 {
				return config.PatternClassOpenTwo
			}
		}
	}
	return config.PatternClassNone
}

// win parses a 9 character witness: '.' empty, 'o' own, 'x' opp, ' ' off.
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
