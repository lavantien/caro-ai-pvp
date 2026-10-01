package engine

import (
	"bytes"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestNpsReport(t *testing.T) {
	cases := [...]struct {
		nodes     uint64
		elapsedNs int64
		want      uint64
	}{
		{0, 0, 0},
		{0, 5000, 0},
		{1000, 0, 1000 * uint64(time.Second)},
		{1000, 1, 1000 * uint64(time.Second)},
		{1, int64(time.Second), 1},
	}
	for _, c := range cases {
		if got := npsReport(c.nodes, c.elapsedNs); got != c.want {
			t.Errorf("npsReport(%d, %d) = %d, want %d", c.nodes, c.elapsedNs, got, c.want)
		}
	}
}

func TestEBFMilli(t *testing.T) {
	cases := [...]struct {
		nodes uint64
		depth int
		want  int
	}{
		{0, 5, 0},
		{1, 0, 0},
		{1, 5, 0},
		{3, 1, 2999},
		{6, 1, 5999},
	}
	for _, c := range cases {
		if got := ebfMilli(c.nodes, c.depth); got != c.want {
			t.Errorf("ebfMilli(%d, %d) = %d, want %d", c.nodes, c.depth, got, c.want)
		}
	}
	// The estimate is the maximal b whose geometric node sum fits.
	for _, c := range [...]struct {
		nodes uint64
		depth int
	}{{7, 2}, {121, 4}, {1 << 20, 8}, {1 << 40, 20}} {
		b := ebfMilli(c.nodes, c.depth)
		if geoSum(b, c.depth) > c.nodes {
			t.Errorf("ebf %d overruns %d nodes at depth %d", b, c.nodes, c.depth)
		}
		if b < 32000 && geoSum(b+1, c.depth) <= c.nodes {
			t.Errorf("ebf %d leaves room: b+1 also fits %d nodes at depth %d", b, c.nodes, c.depth)
		}
	}
	if got := ebfMilli(1<<62, 60); got != 32000 {
		t.Errorf("saturated ebf = %d, want the 32.0 ceiling", got)
	}
}

func TestGeoSumSaturates(t *testing.T) {
	if geoSum(32000, 200) != 1<<62 {
		t.Errorf("geoSum overflow did not saturate: %d", geoSum(32000, 200))
	}
	if got := geoSum(1000, 9); got != 10 {
		t.Errorf("geoSum b=1 depth 9 = %d, want 10", got)
	}
	if got := geoSum(2000, 3); got != 15 {
		t.Errorf("geoSum b=2 depth 3 = %d, want 15", got)
	}
}

func TestAppendPVZeroAlloc(t *testing.T) {
	var s SearchStats
	s.PVLen = 6
	names := [...]string{"H8", "I9", "J10", "A1", "P16", "E5"}
	for i, name := range names {
		cell, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("cell %q: %v", name, err)
		}
		s.PV[i] = rules.Move(cell)
	}
	buf := make([]byte, 0, 256)
	buf = s.AppendPV(buf)
	if want := "H8 I9 J10 A1 P16 E5"; string(buf) != want {
		t.Errorf("pv = %q, want %q", buf, want)
	}
	var out bytes.Buffer
	if _, err := out.Write(buf); err != nil {
		t.Fatalf("write: %v", err)
	}
}
