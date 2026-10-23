package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
)

const tblWindow = config.PatternWindowLen

func tblRealizable(w *[tblWindow]uint8) bool {
	first, last := -1, -1
	for i, s := range w {
		if s != config.PatternStateOff {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	for i := first + 1; i < last; i++ {
		if w[i] == config.PatternStateOff {
			return false
		}
	}
	return true
}

func tblRunFive(w *[tblWindow]uint8, at int) bool {
	own := config.PatternStateOwn
	opp := config.PatternStateOpp
	back := 0
	for i := at - 1; i >= 0 && back < winLen && w[i] == own; i-- {
		back++
	}
	fwd := 0
	for i := at + 1; i < tblWindow && fwd < winLen && w[i] == own; i++ {
		fwd++
	}
	if back+fwd+1 != winLen {
		return false
	}
	blockedBack := at-back-1 >= 0 && w[at-back-1] == opp
	blockedFwd := at+fwd+1 < tblWindow && w[at+fwd+1] == opp
	return !blockedBack || !blockedFwd
}

func tblWinMask(w *[tblWindow]uint8) uint16 {
	var mask uint16
	for i, s := range w {
		if s != config.PatternStateEmpty {
			continue
		}
		w[i] = config.PatternStateOwn
		if tblRunFive(w, i) {
			mask |= 1 << uint(i)
		}
		w[i] = config.PatternStateEmpty
	}
	return mask
}

func tblFill(w *[tblWindow]uint8, i int) uint32 {
	idx := uint32(0)
	for j, s := range w {
		st := s
		if j == i {
			st = config.PatternStateOwn
		}
		idx |= uint32(st) << uint(j*config.PatternCellBits)
	}
	return idx
}

func tblN1(idx uint32) int {
	w := pattern.Unpack(idx)
	return int(popcount16(tblWinMask(&w)))
}

func popcount16(m uint16) int {
	n := 0
	for m != 0 {
		m &= m - 1
		n++
	}
	return n
}

func tblThreeFlag(idx uint32) bool {
	w := pattern.Unpack(idx)
	for i, s := range w {
		if s != config.PatternStateEmpty {
			continue
		}
		if tblN1(tblFill(&w, i)) >= 2 {
			return true
		}
	}
	return false
}

func tblBrokenFlag(idx uint32) bool {
	w := pattern.Unpack(idx)
	for i, s := range w {
		if s != config.PatternStateEmpty {
			continue
		}
		if tblN1(tblFill(&w, i)) == 1 {
			return true
		}
	}
	return false
}

func tblClass(idx uint32) uint8 {
	n1 := tblN1(idx)
	switch n1 {
	case 0:
		if tblThreeFlag(idx) {
			return config.PatternClassThree
		}
		if tblBrokenFlag(idx) {
			return config.PatternClassBrokenThree
		}
		w := pattern.Unpack(idx)
		for i, s := range w {
			if s == config.PatternStateEmpty && tblThreeFlag(tblFill(&w, i)) {
				return config.PatternClassOpenTwo
			}
		}
		return config.PatternClassNone
	case 1:
		return config.PatternClassFour
	}
	return config.PatternClassOpenFour
}

func tblIdxToWindow(idx uint32) [tblWindow]uint8 {
	return pattern.Unpack(idx)
}

func runTable(outDir string) {
	path := filepath.Join(outDir, "pattern-table-rederivation.txt")
	fh, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	defer func() { _ = fh.Close() }()
	p := func(format string, args ...any) {
		s := fmt.Sprintf(format, args...)
		fmt.Print(s)
		_, _ = fmt.Fprint(fh, s)
	}
	maskBad, classBad, checked, realizable := 0, 0, 0, 0
	var firstBad [10]string
	for idx := range config.PatternTableEntries {
		w := tblIdxToWindow(uint32(idx))
		if !tblRealizable(&w) {
			continue
		}
		realizable++
		got := pattern.Lookup(uint32(idx))
		wantMask := tblWinMask(&w)
		wantClass := tblClass(uint32(idx))
		checked++
		if got.Win1 != wantMask || got.Class != wantClass {
			if maskBad+classBad < len(firstBad) {
				firstBad[maskBad+classBad] = fmt.Sprintf("idx=%#x win1=%#x/%#x class=%d/%d", idx, got.Win1, wantMask, got.Class, wantClass)
			}
			if got.Win1 != wantMask {
				maskBad++
			}
			if got.Class != wantClass {
				classBad++
			}
		}
	}
	p("pattern table: checked %d realizable states of %d total, mask mismatches %d, class mismatches %d\n", checked, config.PatternTableEntries, maskBad, classBad)
	for i := 0; i < maskBad+classBad && i < len(firstBad) && firstBad[i] != ""; i++ {
		p("  mismatch %s\n", firstBad[i])
	}
	if maskBad == 0 && classBad == 0 {
		p("pattern table: independent line-oracle re-derivation agrees on every realizable entry\n")
	}
	unrealizableOffInterior := 0
	for idx := range config.PatternTableEntries {
		w := tblIdxToWindow(uint32(idx))
		if !tblRealizable(&w) {
			unrealizableOffInterior++
		}
	}
	p("pattern table: %d states carry interior Off and are unrealizable by construction\n", unrealizableOffInterior)
	hexWindowScan(p)
}

func hexWindowScan(p func(string, ...any)) {
	g, err := geom.ByName("hex")
	if err != nil {
		fail(err)
	}
	enum, scored, offWindows, interiorOff := 0, 0, 0, 0
	for cell := range g.Cells {
		if !g.Contains(cell) {
			continue
		}
		for d := range g.Dirs {
			enum++
			states := pattern.Index(newHexRulesBoard(g), rulesCell(cell), d, redRules)
			w := pattern.Unpack(states)
			allIn := true
			for _, s := range w {
				if s == config.PatternStateOff {
					allIn = false
				}
			}
			if allIn {
				scored++
			} else {
				offWindows++
			}
			if !tblRealizable(&w) {
				interiorOff++
			}
		}
	}
	p("hex windows: enumerated %d (center in region), fully in-region %d, rim windows with Off %d, windows with INTERIOR Off %d\n", enum, scored, offWindows, interiorOff)
	if interiorOff == 0 {
		p("hex windows: Off cells form prefix and suffix runs only, interior-Off table states are unreachable\n")
	}
}
