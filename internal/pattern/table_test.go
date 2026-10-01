package pattern

import (
	"strconv"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestGeneratorTruthExhaustive(t *testing.T) {
	stride := 1
	if testing.Short() {
		stride = 97
	}
	for dir := range config.PatternDirections {
		t.Run(strconv.Itoa(dir), func(t *testing.T) {
			t.Parallel()
			checked := 0
			for idx := range config.PatternTableEntries {
				if idx%stride != 0 {
					continue
				}
				w := Unpack(uint32(idx))
				if !isRealizable(&w) {
					if entry := tables[dir][idx]; entry != (Entry{}) {
						t.Fatalf("idx %#x: non-realizable entry %+v want zero", idx, entry)
					}
					continue
				}
				if got, want := tables[dir][idx].Win1, naiveWin1(t, w); got != want {
					t.Fatalf("idx %#x %v: Win1 = %#x want %#x", idx, w, got, want)
				}
				checked++
			}
			if checked == 0 {
				t.Fatal("no realizable windows checked")
			}
		})
	}
}

func TestClassTruth(t *testing.T) {
	strides := [config.PatternDirections]int{1, 149, 149, 149}
	if testing.Short() {
		strides = [config.PatternDirections]int{997, 997, 997, 997}
	}
	for dir := range config.PatternDirections {
		t.Run(strconv.Itoa(dir), func(t *testing.T) {
			t.Parallel()
			for idx := range config.PatternTableEntries {
				if idx%strides[dir] != 0 {
					continue
				}
				w := Unpack(uint32(idx))
				if !isRealizable(&w) {
					continue
				}
				if got, want := tables[dir][idx].Class, naiveClass(t, w); got != want {
					t.Fatalf("idx %#x %v: Class = %d want %d", idx, w, got, want)
				}
			}
		})
	}
}

func TestTaxonomyWitnesses(t *testing.T) {
	cases := []struct {
		name     string
		witness  string
		class    uint8
		win1Bits []int
	}{
		{"four", "xoooo..xx", config.PatternClassFour, []int{5}},
		{"open four", ".oooo.xxx", config.PatternClassOpenFour, []int{0, 5}},
		{"three", "..ooo..xx", config.PatternClassThree, nil},
		{"broken three", "xooo...xx", config.PatternClassBrokenThree, nil},
		{"open two", "..oo...xx", config.PatternClassOpenTwo, nil},
		{"none", "oxoxoxo..", config.PatternClassNone, nil},
		{"split four", "oo.oo....", config.PatternClassFour, []int{2}},
	}
	for _, c := range cases {
		w := win(t, c.witness)
		if got, want := naiveClass(t, w), c.class; got != want {
			t.Fatalf("%s: naive class = %d want %d", c.name, got, want)
		}
		var wantMask uint16
		for _, bit := range c.win1Bits {
			wantMask |= 1 << uint(bit)
		}
		for dir := range config.PatternDirections {
			entry := Lookup(dir, Pack(w))
			if entry.Class != c.class {
				t.Fatalf("%s dir %d: class = %d want %d", c.name, dir, entry.Class, c.class)
			}
			if entry.Win1 != wantMask {
				t.Fatalf("%s dir %d: Win1 = %#x want %#x", c.name, dir, entry.Win1, wantMask)
			}
		}
	}
}

func TestOverlineExclusion(t *testing.T) {
	cases := []string{
		".ooooo...", // extending either end makes a 6 run
		"oo.ooo...", // gap fill makes a 6 run
		".ooooox..", // live existing five, extension overlines
		"xooooox..", // fully blocked existing five
		".oooooo..", // existing overline
		"  .ooooo.", // wall-flanked five, extension overlines
	}
	for _, s := range cases {
		w := win(t, s)
		for dir := range config.PatternDirections {
			entry := Lookup(dir, Pack(w))
			if entry.Win1 != 0 {
				t.Fatalf("%q dir %d: Win1 = %#x want 0, overline completions never win", s, dir, entry.Win1)
			}
			if got, want := entry.Class, naiveClass(t, w); got != want {
				t.Fatalf("%q dir %d: class = %d want %d", s, dir, got, want)
			}
		}
	}
}

func TestRealizableCount(t *testing.T) {
	want := 0
	for idx := range config.PatternTableEntries {
		w := Unpack(uint32(idx))
		if isRealizable(&w) {
			want++
		}
	}
	live := 0
	for idx := range config.PatternTableEntries {
		if flagsScratch[idx]&flagRealizable != 0 {
			live++
		}
	}
	if live != want {
		t.Fatalf("generated realizable entries = %d want %d", live, want)
	}
}

func TestPatternConfigInvariants(t *testing.T) {
	if config.PatternWindowLen > 16 || config.PatternWindowLen%2 == 0 {
		t.Fatalf("PatternWindowLen = %d, want odd and at most 16 for the uint16 mask", config.PatternWindowLen)
	}
	wantDirs := [config.PatternDirections][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
	if config.PatternDirs != wantDirs {
		t.Fatalf("PatternDirs = %v want %v", config.PatternDirs, wantDirs)
	}
	if len(config.PatternClassWeights) != config.PatternClassCount {
		t.Fatalf("len(PatternClassWeights) = %d want %d", len(config.PatternClassWeights), config.PatternClassCount)
	}
	for i := 1; i < len(config.PatternClassWeights); i++ {
		if config.PatternClassWeights[i] < config.PatternClassWeights[i-1] {
			t.Fatalf("PatternClassWeights must be nondecreasing up the forcing order: %v", config.PatternClassWeights)
		}
	}
}
