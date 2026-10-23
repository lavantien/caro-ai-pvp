package main

import (
	"math"
	"testing"
)

func TestPlayableCountIs397(t *testing.T) {
	n := 0
	for q := range hexWidth {
		for r := range hexWidth {
			if playable(q, r) {
				n++
			}
		}
	}
	if n != 397 {
		t.Fatalf("playable cells = %d, want 397", n)
	}
}

func TestPlayableMaskBoundary(t *testing.T) {
	yes := [][2]int{{11, 11}, {0, 11}, {11, 0}, {22, 11}, {11, 22}, {0, 22}, {22, 0}, {1, 21}}
	no := [][2]int{{0, 0}, {0, 10}, {10, 0}, {22, 22}, {21, 22}, {22, 21}, {5, 5}}
	for _, c := range yes {
		if !playable(c[0], c[1]) {
			t.Errorf("playable(%d,%d) = false, want true", c[0], c[1])
		}
	}
	for _, c := range no {
		if playable(c[0], c[1]) {
			t.Errorf("playable(%d,%d) = true, want false", c[0], c[1])
		}
	}
}

func TestMaxPlayableIndexIs517(t *testing.T) {
	max := -1
	for q := range hexWidth {
		for r := range hexWidth {
			if playable(q, r) && q*hexWidth+r > max {
				max = q*hexWidth + r
			}
		}
	}
	if max != 517 {
		t.Fatalf("max playable index = %d, want 517", max)
	}
}

func TestCellNameSamples(t *testing.T) {
	cases := [][3]any{{11, 11, "L12"}, {0, 11, "L1"}, {7, 7, "H8"}, {22, 11, "L23"}, {0, 22, "W1"}, {22, 0, "A23"}, {5, 20, "U6"}}
	for _, c := range cases {
		if got := cellName(c[0].(int), c[1].(int)); got != c[2].(string) {
			t.Errorf("cellName(%d,%d) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestCellNameRoundTripOverRegion(t *testing.T) {
	for q := range hexWidth {
		for r := range hexWidth {
			if !playable(q, r) {
				continue
			}
			pq, pr, ok := parseCell(cellName(q, r))
			if !ok || pq != q || pr != r {
				t.Fatalf("parseCell(cellName(%d,%d)) = %d,%d,%v", q, r, pq, pr, ok)
			}
		}
	}
}

func TestCellNamesUnique(t *testing.T) {
	seen := make(map[string]bool)
	for q := range hexWidth {
		for r := range hexWidth {
			if playable(q, r) {
				seen[cellName(q, r)] = true
			}
		}
	}
	if len(seen) != 397 {
		t.Fatalf("unique cell names = %d, want 397", len(seen))
	}
}

func TestParseCellRejects(t *testing.T) {
	for _, s := range []string{"", "A", "A1", "A0", "A24", "X5", "Z1", "a12", "AA1", "L 2", "L-1", "L023"} {
		if _, _, ok := parseCell(s); ok {
			t.Errorf("parseCell(%q) accepted, want reject", s)
		}
	}
}

func TestNeighborCenterDistance(t *testing.T) {
	tolerance := 1e-9
	dirs := [6][2]int{{0, 1}, {1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}}
	for q := range hexWidth {
		for r := range hexWidth {
			if !playable(q, r) {
				continue
			}
			for _, d := range dirs {
				nq, nr := q+d[0], r+d[1]
				if nq < 0 || nr < 0 || nq >= hexWidth || nr >= hexWidth || !playable(nq, nr) {
					continue
				}
				x1, y1 := cellCenter(q, r)
				x2, y2 := cellCenter(nq, nr)
				dist := math.Hypot(x2-x1, y2-y1)
				if math.Abs(dist-sqrt3*unitR) > tolerance {
					t.Fatalf("centers (%d,%d)-(%d,%d) distance %g, want %g", q, r, nq, nr, dist, sqrt3*unitR)
				}
			}
		}
	}
}

func TestPolygonShape(t *testing.T) {
	cx, cy := cellCenter(11, 11)
	pts := polygonPoints(cx, cy)
	if len(pts) != 6 {
		t.Fatalf("polygon corners = %d, want 6", len(pts))
	}
	minR, maxR := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		d := math.Hypot(p[0]-cx, p[1]-cy)
		minR = math.Min(minR, d)
		maxR = math.Max(maxR, d)
	}
	if minR < unitR*(1-hexGapFrac)-1e-9 || maxR > unitR+1e-9 {
		t.Fatalf("polygon radii [%g,%g] outside [%g,%g]", minR, maxR, unitR*(1-hexGapFrac), unitR)
	}
	vertical := false
	for _, p := range pts {
		if math.Abs(p[0]-cx) < 1e-9 {
			vertical = true
		}
	}
	if !vertical {
		t.Fatalf("no pointy-top vertical corner found for cell at (%g,%g)", cx, cy)
	}
}

func TestBoardModelContract(t *testing.T) {
	m := buildBoard()
	if len(m.Cells) != 397 {
		t.Fatalf("model cells = %d, want 397", len(m.Cells))
	}
	ids := make(map[string]bool)
	for _, c := range m.Cells {
		if c.Name == "" || ids[c.Name] {
			t.Fatalf("bad or duplicate model name %q", c.Name)
		}
		ids[c.Name] = true
	}
	if !ids["L12"] {
		t.Fatalf("center cell L12 missing from model")
	}
	w := m.ViewBox[2] - m.ViewBox[0]
	h := m.ViewBox[3] - m.ViewBox[1]
	if w <= 0 || h <= 0 || w/h > 1.2 {
		t.Fatalf("viewBox %v not board-shaped", m.ViewBox)
	}
}
