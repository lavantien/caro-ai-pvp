package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type position struct {
	Leg   string   `json:"leg"`
	Game  int      `json:"game"`
	Move  int      `json:"move"`
	Pool  string   `json:"pool"`
	Cell  string   `json:"cell"`
	Score int      `json:"score"`
	Depth int      `json:"depth"`
	Tag   string   `json:"tag"`
	Moves []string `json:"moves"`
}

type positionFile struct {
	Positions []position `json:"positions"`
}

type windowHit struct {
	Cell  string
	Dir   int
	Class uint8
}

type sideDecomp struct {
	Total     int
	ClassN    [config.PatternClassCount]int
	Own1      int
	Own1Score int
	Own2      int
	Own2Score int
	Own3      int
	Own3Score int
	CleanOwn  int
	Windows   []windowHit
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "sqdecomp: "+format+"\n", args...)
	os.Exit(1)
}

func buildBoard(g *geom.Geometry, pos position, withCell bool) *rules.Board {
	b := rules.NewBoardFor(g)
	for _, name := range pos.Moves {
		cell, err := g.ParseCell(name)
		if err != nil {
			fail("position %s g%d m%d: cell %s: %v", pos.Leg, pos.Game, pos.Move, name, err)
		}
		b.Make(rules.Cell(cell))
	}
	if withCell {
		cell, err := g.ParseCell(pos.Cell)
		if err != nil {
			fail("position %s g%d m%d: cell %s: %v", pos.Leg, pos.Game, pos.Move, pos.Cell, err)
		}
		b.Make(rules.Cell(cell))
	}
	return b
}

func decompose(g *geom.Geometry, b *rules.Board) (red, blue sideDecomp) {
	w := config.PatternClassWeights
	for d := range g.Dirs {
		for cell := range g.Cells {
			if !g.Contains(cell) {
				continue
			}
			for side, idx := range map[rules.Color]uint32{
				rules.Red:  pattern.Index(b, rules.Cell(cell), d, rules.Red),
				rules.Blue: pattern.Index(b, rules.Cell(cell), d, rules.Blue),
			} {
				entry := pattern.Lookup(idx)
				cls := entry.Class
				sd := &red
				if side == rules.Blue {
					sd = &blue
				}
				sd.Total += w[cls]
				sd.ClassN[cls]++
				if cls != config.PatternClassNone {
					name := string(g.AppendName(nil, cell))
					sd.Windows = append(sd.Windows, windowHit{Cell: name, Dir: d, Class: cls})
				}
				states := pattern.Unpack(idx)
				own := 0
				opposed := false
				for _, s := range states {
					switch s {
					case config.PatternStateOwn:
						own++
					case config.PatternStateOpp:
						opposed = true
					}
				}
				if own == 0 || opposed {
					continue
				}
				sd.CleanOwn++
				score := w[cls]
				switch own {
				case 1:
					sd.Own1++
					sd.Own1Score += score
				case 2:
					sd.Own2++
					sd.Own2Score += score
				default:
					sd.Own3++
					sd.Own3Score += score
				}
			}
		}
	}
	return red, blue
}

func sortedWindows(sd sideDecomp, limit int) string {
	hits := append([]windowHit(nil), sd.Windows...)
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Class != hits[j].Class {
			return hits[i].Class > hits[j].Class
		}
		if hits[i].Cell != hits[j].Cell {
			return hits[i].Cell < hits[j].Cell
		}
		return hits[i].Dir < hits[j].Dir
	})
	parts := make([]string, 0, limit)
	for i, h := range hits {
		if i >= limit {
			break
		}
		parts = append(parts, fmt.Sprintf("%s/d%d=c%d", h.Cell, h.Dir, h.Class))
	}
	return strings.Join(parts, " ")
}

func main() {
	positionsPath := flag.String("positions", "playground/sqstrength/out/discriminator-positions.json", "position list json")
	outPath := flag.String("out", "playground/sqstrength/out/decomp-l.txt", "decomposition output")
	withCell := flag.Bool("afterMove", true, "decompose the board after the recorded move")
	variant := flag.String("variant", "square", "board geometry variant")
	branch := flag.Bool("branch", false, "run the divergence-point branch probes instead of the pool decomposition")
	signal := flag.Bool("signal", false, "run the candidate-signal probes on the divergence boards")
	phase := flag.Bool("phase", false, "run the opening and midgame board probes through the corrected engine")
	family := flag.Bool("family", false, "run the regate2 loss and win family census over all legal children")
	familyOut := flag.String("familyOut", "playground/sqstrength/out/family-probe.txt", "family probe output")
	cutSweepMoves := flag.String("cutSweep", "", "comma list of board moves, then print the boxed-cut sweep for -cutCands")
	cutCands := flag.String("cutCands", "", "comma list of candidate cells for the cut sweep")
	flag.Parse()

	g0, err := geom.ByName(*variant)
	if err != nil {
		fail("geometry %s: %v", *variant, err)
	}
	if *branch {
		runBranchProbes(g0)
		return
	}
	if *signal {
		runSignalProbes(g0)
		return
	}
	if *phase {
		runPhaseProbes(g0)
		return
	}
	if *family {
		runFamilyProbes(g0, *positionsPath, *familyOut)
		return
	}
	if *cutSweepMoves != "" {
		runCutSweep(g0, strings.Split(*cutSweepMoves, ","), strings.Split(*cutCands, ","))
		return
	}

	g, err := geom.ByName(*variant)
	if err != nil {
		fail("geometry %s: %v", *variant, err)
	}
	raw, err := os.ReadFile(*positionsPath)
	if err != nil {
		fail("read %s: %v", *positionsPath, err)
	}
	var pf positionFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		fail("parse %s: %v", *positionsPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		fail("create out dir: %v", err)
	}
	var sb strings.Builder
	for _, pos := range pf.Positions {
		if pos.Move < 6 {
			continue
		}
		b := buildBoard(g, pos, *withCell)
		red, blue := decompose(g, b)
		net := blue.Total - red.Total
		fmt.Fprintf(&sb, "pos %s g%d m%d pool=%s cell=%s inGameScore=%d\n", pos.Leg, pos.Game, pos.Move, pos.Pool, pos.Cell, pos.Score)
		fmt.Fprintf(&sb, "  blueTotal=%d redTotal=%d netBlue=%d\n", blue.Total, red.Total, net)
		fmt.Fprintf(&sb, "  blueClasses=%v redClasses=%v\n", blue.ClassN, red.ClassN)
		fmt.Fprintf(&sb, "  blueClean own1=%d(%d) own2=%d(%d) own3+%d(%d)\n", blue.Own1, blue.Own1Score, blue.Own2, blue.Own2Score, blue.Own3, blue.Own3Score)
		fmt.Fprintf(&sb, "  redClean  own1=%d(%d) own2=%d(%d) own3+%d(%d)\n", red.Own1, red.Own1Score, red.Own2, red.Own2Score, red.Own3, red.Own3Score)
		fmt.Fprintf(&sb, "  blueWin: %s\n", sortedWindows(blue, 24))
		fmt.Fprintf(&sb, "  redWin:  %s\n", sortedWindows(red, 24))
	}
	if err := os.WriteFile(*outPath, []byte(sb.String()), 0o644); err != nil {
		fail("write %s: %v", *outPath, err)
	}
	fmt.Printf("wrote %s\n", *outPath)
}
