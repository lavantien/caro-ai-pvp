package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

type ponderRow struct {
	series   string
	redName  string
	blueName string
	game     int
	move     int
	side     string
	seat     string
	opponent string
	nodes    uint64
	nps      uint64
	treeSec  float64
	ownT     float64
	oppPrevT float64
	alloc    float64
	inflated bool
}

type scanData struct {
	dir        string
	files      int
	games      int
	ponderRows int
	rows       []ponderRow
}

const (
	inflateFactor = 2
	inflateSlack  = 1.0
	ownTSlowSec   = 5.0
)

func scanDir(dir string) (*scanData, error) {
	paths, err := readSeriesLogs(dir)
	if err != nil {
		return nil, err
	}
	s := &scanData{dir: dir, files: len(paths)}
	for _, path := range paths {
		f, err := parseFile(path)
		if err != nil {
			return nil, err
		}
		base := strings.TrimSuffix(filepath.Base(path), ".txt")
		game := 0
		redIsFirst := true
		prevT := 0.0
		for _, r := range f.records {
			if r.ml != nil {
				if r.ml.tag == "" {
					prevT = r.ml.t
				}
				if r.ml.tag != ponderBody {
					continue
				}
				redSeat, blueSeat := f.redName, f.blueName
				if !redIsFirst {
					redSeat, blueSeat = blueSeat, redSeat
				}
				seat, opp := redSeat, blueSeat
				if r.ml.side == "Blue" {
					seat, opp = blueSeat, redSeat
				}
				tree := 0.0
				if r.ml.nps > 0 {
					tree = float64(r.ml.nodes) / float64(r.ml.nps)
				}
				bound := math.Max(r.ml.t, prevT)
				row := ponderRow{
					series: base, redName: f.redName, blueName: f.blueName,
					game: game, move: r.ml.move, side: r.ml.side, seat: seat, opponent: opp,
					nodes: r.ml.nodes, nps: r.ml.nps, treeSec: tree,
					ownT: r.ml.t, oppPrevT: prevT, alloc: r.ml.alloc,
					inflated: tree > inflateFactor*bound+inflateSlack || r.ml.t > ownTSlowSec,
				}
				s.rows = append(s.rows, row)
				s.ponderRows++
				continue
			}
			s.games++
			game = r.gl.game
			redIsFirst = !redIsFirst
			prevT = 0
		}
	}
	return s, nil
}

func renderScan(s *scanData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\nfiles %d, games %d, ponder lines %d\n\n", s.dir, s.files, s.games, s.ponderRows)
	b.WriteString("| series | game | move | seat | tree s | own t | opp t | alloc | inflated |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range s.rows {
		fmt.Fprintf(&b, "| %s | %d | %d | %s | %.1f | %.2f | %.2f | %.2f | %s |\n",
			r.series, r.game, r.move, r.seat, r.treeSec, r.ownT, r.oppPrevT, r.alloc, mark(r.inflated))
	}
	n := 0
	for _, r := range s.rows {
		if r.inflated {
			n++
		}
	}
	fmt.Fprintf(&b, "\ninflated %d of %d\n\n", n, s.ponderRows)
	return b.String()
}

func mark(v bool) string {
	if v {
		return "YES"
	}
	return ""
}
