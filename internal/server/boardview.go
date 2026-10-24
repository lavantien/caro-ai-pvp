package server

import (
	"math"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
)

const (
	hexUnitRadius          = 10.0
	hexGapFraction         = 0.045
	hexViewPad             = 2.5
	hexStoneRadiusFraction = 0.63
)

var hexSqrt3 = math.Sqrt(3)

type hexCellView struct {
	Name      string
	Transform string
	Stone     string
	Glyph     string
	Latest    bool
	Idx       int
	Off       bool
}

type hexBoardView struct {
	ViewBox string
	Aspect  string
	Points  string
	StoneR  string
	Cells   []hexCellView
}

func hexFmt(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func hexPolygonPoints() string {
	rad := hexUnitRadius * (1 - hexGapFraction)
	pts := make([]string, 0, 6)
	for i := range 6 {
		a := math.Pi / 180 * (60*float64(i) - 30)
		pts = append(pts, hexFmt(rad*math.Cos(a))+" "+hexFmt(rad*math.Sin(a)))
	}
	return strings.Join(pts, " ")
}

func hexCellCenter(q, r int) (float64, float64) {
	return hexSqrt3 * hexUnitRadius * (float64(r) + float64(q)/2), 1.5 * hexUnitRadius * float64(q)
}

func hexPlayableCount(g *geom.Geometry) int {
	n := 0
	for flat := range g.Cells {
		if g.Contains(flat) {
			n++
		}
	}
	return n
}

func hexBoardOf(g *geom.Geometry, moves []string, playable map[string]bool) hexBoardView {
	at := make(map[string]int, len(moves))
	for i, name := range moves {
		at[name] = i
	}
	view := hexBoardView{Cells: make([]hexCellView, 0, hexPlayableCount(g))}
	view.Points = hexPolygonPoints()
	view.StoneR = hexFmt(hexUnitRadius * hexStoneRadiusFraction)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	rad := hexUnitRadius * (1 - hexGapFraction)
	track := func(x, y float64) {
		minX = math.Min(minX, x)
		minY = math.Min(minY, y)
		maxX = math.Max(maxX, x)
		maxY = math.Max(maxY, y)
	}
	for flat := range g.Cells {
		if !g.Contains(flat) {
			continue
		}
		q, r := flat/g.Stride, flat%g.Stride
		x, y := hexCellCenter(q, r)
		track(x-rad*hexSqrt3/2, y-rad)
		track(x+rad*hexSqrt3/2, y+rad)
		name := string(g.AppendName(nil, flat))
		cv := hexCellView{Name: name, Transform: "translate(" + hexFmt(x) + " " + hexFmt(y) + ")"}
		if i, ok := at[name]; ok {
			cv.Idx = i
			cv.Stone, cv.Glyph = stoneOf(i)
			cv.Latest = i == len(moves)-1
		} else if playable != nil && !playable[name] {
			cv.Off = true
		}
		view.Cells = append(view.Cells, cv)
	}
	x0, y0 := minX-hexViewPad, minY-hexViewPad
	w, h := maxX+hexViewPad-x0, maxY+hexViewPad-y0
	view.ViewBox = strings.Join([]string{hexFmt(x0), hexFmt(y0), hexFmt(w), hexFmt(h)}, " ")
	view.Aspect = strconv.FormatFloat(w/h, 'f', 4, 64)
	return view
}

func boardDataFor(variant string, moves []string, playable map[string]bool) boardData {
	if g, err := geom.ByName(variant); err == nil && g.Kind == geom.KindHex {
		return boardData{BoardSize: config.BoardSize, Hex: true, HexBoard: hexBoardOf(g, moves, playable)}
	}
	return boardData{Cells: boardCells(moves, playable), BoardSize: config.BoardSize}
}
