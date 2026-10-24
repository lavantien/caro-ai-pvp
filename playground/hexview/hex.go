package main

import (
	"math"
	"strconv"
)

const (
	hexSide     = 12
	hexWidth    = 2*hexSide - 1
	hexCenter   = hexSide - 1
	unitR       = 10.0
	hexGapFrac  = 0.045
	labelSize   = 7.5
	stoneR      = 6.3
	latestRingW = 0.55
	strokeCellW = 0.22
	viewBoxPad  = 2.5
)

var sqrt3 = math.Sqrt(3)

const (
	colBackground = "#f9e8f1"
	colSurface    = "#fff7fb"
	colBorder     = "#c48ba1"
	colTextMuted  = "#6f5f75"
	colRed        = "#c2185b"
	colBlue       = "#1565c0"
	colMark       = "#b26a00"
)

func playable(q, r int) bool {
	dq, dr := q-hexCenter, r-hexCenter
	return abs(dq) <= hexCenter && abs(dr) <= hexCenter && abs(dq+dr) <= hexCenter
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func cellName(q, r int) string {
	return string(rune('A'+r)) + strconv.Itoa(q+1)
}

func parseCell(name string) (int, int, bool) {
	if len(name) < 2 || len(name) > 3 {
		return 0, 0, false
	}
	r := int(name[0]) - 'A'
	if r < 0 || r >= hexWidth {
		return 0, 0, false
	}
	n, err := strconv.Atoi(name[1:])
	if err != nil || n < 1 || n > hexWidth {
		return 0, 0, false
	}
	q := n - 1
	if !playable(q, r) {
		return 0, 0, false
	}
	return q, r, true
}

func cellCenter(q, r int) (float64, float64) {
	return sqrt3 * unitR * (float64(r) + float64(q)/2), 1.5 * unitR * float64(q)
}

func polygonPoints(cx, cy float64) [6][2]float64 {
	var pts [6][2]float64
	rad := unitR * (1 - hexGapFrac)
	for i := range pts {
		a := math.Pi / 180 * (60*float64(i) - 30)
		pts[i][0] = cx + rad*math.Cos(a)
		pts[i][1] = cy + rad*math.Sin(a)
	}
	return pts
}

type cellView struct {
	Q, R   int
	Name   string
	X, Y   float64
	Points [6][2]float64
}

type labelView struct {
	Text   string
	X, Y   float64
	Anchor string
}

type stoneView struct {
	Name   string
	X, Y   float64
	Fill   string
	Latest bool
}

type boardModel struct {
	Cells     []cellView
	ColLabels []labelView
	RowLabels []labelView
	Stones    []stoneView
	ViewBox   [4]float64
}

func buildBoard() boardModel {
	m := boardModel{}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	track := func(x, y float64) {
		minX = math.Min(minX, x)
		minY = math.Min(minY, y)
		maxX = math.Max(maxX, x)
		maxY = math.Max(maxY, y)
	}
	for q := range hexWidth {
		for r := range hexWidth {
			if !playable(q, r) {
				continue
			}
			x, y := cellCenter(q, r)
			pts := polygonPoints(x, y)
			for _, p := range pts {
				track(p[0], p[1])
			}
			m.Cells = append(m.Cells, cellView{Q: q, R: r, Name: cellName(q, r), X: x, Y: y, Points: pts})
		}
	}
	for r := range hexWidth {
		q := max(0, hexCenter-r)
		if !playable(q, r) {
			continue
		}
		x, y := cellCenter(q, r)
		lx, ly := x, y-unitR*1.42
		m.ColLabels = append(m.ColLabels, labelView{Text: string(rune('A' + r)), X: lx, Y: ly, Anchor: "middle"})
		track(lx-labelSize*0.4, ly-labelSize*0.8)
	}
	for q := range hexWidth {
		r := max(0, hexCenter-q)
		if !playable(q, r) {
			continue
		}
		x, y := cellCenter(q, r)
		lx, ly := x-sqrt3*unitR*1.18, y+labelSize*0.35
		m.RowLabels = append(m.RowLabels, labelView{Text: strconv.Itoa(q + 1), X: lx, Y: ly, Anchor: "end"})
		track(lx-labelSize*0.8, ly)
	}
	m.ViewBox = [4]float64{minX - viewBoxPad, minY - viewBoxPad, maxX + viewBoxPad, maxY + viewBoxPad}
	m.Stones = []stoneView{{Name: "L12", Fill: colRed, Latest: true}, {Name: "M14", Fill: colBlue}}
	for i := range m.Stones {
		q, r, _ := parseCell(m.Stones[i].Name)
		m.Stones[i].X, m.Stones[i].Y = cellCenter(q, r)
	}
	return m
}
