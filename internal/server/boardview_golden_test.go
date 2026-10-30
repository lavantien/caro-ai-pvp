package server

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func goldenFmt2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func TestHexBoardViewGoldenGeometry(t *testing.T) {
	g := mustHexGeometry(t)
	view := hexBoardOf(g, nil, nil)

	sqrt3 := math.Sqrt(3)
	unit, rad, pad := 10.0, 10.0*(1-0.045), 2.5

	pts := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		a := (60*float64(i) - 30) * (math.Pi / 180)
		pts = append(pts, goldenFmt2(rad*math.Cos(a))+" "+goldenFmt2(rad*math.Sin(a)))
	}
	if want := strings.Join(pts, " "); view.Points != want {
		t.Errorf("hex polygon points:\n got %q\nwant %q", view.Points, want)
	}
	if want := goldenFmt2(unit * 0.63); view.StoneR != want {
		t.Errorf("hex stone radius = %q, want %q", view.StoneR, want)
	}

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	transforms := make(map[string]string, len(view.Cells))
	for flat := range g.Cells {
		if !g.Contains(flat) {
			continue
		}
		q, r := flat/g.Stride, flat%g.Stride
		x := sqrt3 * unit * (float64(r) + float64(q)/2)
		y := 1.5 * unit * float64(q)
		minX = math.Min(minX, x-rad*sqrt3/2)
		minY = math.Min(minY, y-rad)
		maxX = math.Max(maxX, x+rad*sqrt3/2)
		maxY = math.Max(maxY, y+rad)
		transforms[string(g.AppendName(nil, flat))] = "translate(" + goldenFmt2(x) + " " + goldenFmt2(y) + ")"
	}
	if len(view.Cells) != len(transforms) {
		t.Fatalf("hex view cells = %d, want the %d playable cells", len(view.Cells), len(transforms))
	}
	for _, c := range view.Cells {
		want, ok := transforms[c.Name]
		if !ok || c.Transform != want {
			t.Errorf("cell %s transform = %q, want %q", c.Name, c.Transform, want)
		}
	}

	x0, y0 := minX-pad, minY-pad
	w, h := maxX+pad-x0, maxY+pad-y0
	if want := strings.Join([]string{goldenFmt2(x0), goldenFmt2(y0), goldenFmt2(w), goldenFmt2(h)}, " "); view.ViewBox != want {
		t.Errorf("hex viewBox:\n got %q\nwant %q", view.ViewBox, want)
	}
	if want := strconv.FormatFloat(w/h, 'f', 4, 64); view.Aspect != want {
		t.Errorf("hex aspect = %q, want %q", view.Aspect, want)
	}
}
