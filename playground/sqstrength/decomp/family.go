package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type sideCensus struct {
	classN  [config.PatternClassCount]int
	cleanN  [config.PatternClassCount]int
	eng2N   [config.PatternClassCount]int
	farCont [config.PatternClassCount]int
	score   int
	clean   int
}

func censusSide(g *geom.Geometry, b *rules.Board, side rules.Color) sideCensus {
	var c sideCensus
	for d := range g.Dirs {
		for cell := range g.Cells {
			if !g.Contains(cell) {
				continue
			}
			idx := pattern.Index(b, rules.Cell(cell), d, side)
			cls := pattern.Lookup(idx).Class
			c.classN[cls]++
			c.score += config.PatternClassWeights[cls]
			states := pattern.Unpack(idx)
			own, opp, eng := false, false, false
			for i, s := range states {
				if s == config.PatternStateOwn {
					own = true
					for j, t := range states {
						if t == config.PatternStateOpp && j >= i-2 && j <= i+2 {
							eng = true
						}
					}
				}
				if s == config.PatternStateOpp {
					opp = true
				}
			}
			if own && !opp {
				c.cleanN[cls]++
				c.clean++
			} else if own && opp {
				if eng {
					c.eng2N[cls]++
				} else {
					c.farCont[cls]++
				}
			}
		}
	}
	return c
}

type childLine struct {
	name   string
	net    int
	cutNet [3]int
}

func boardNet(g *geom.Geometry, b *rules.Board, mover rules.Color, cuts [3]int) (int, [3]int) {
	me := censusSide(g, b, mover)
	opp := censusSide(g, b, mover^1)
	net := me.score - opp.score +
		config.EvalCleanWindowWeightSquare*(me.clean-opp.clean) -
		config.EvalBoxedCutThreeSquare*(me.eng2N[config.PatternClassThree]-opp.eng2N[config.PatternClassThree])
	var cutNet [3]int
	for k, c := range cuts {
		delta := (me.farCont[config.PatternClassOpenTwo] + me.eng2N[config.PatternClassOpenTwo]) - (opp.farCont[config.PatternClassOpenTwo] + opp.eng2N[config.PatternClassOpenTwo])
		cutNet[k] = net - c*delta
	}
	return net, cutNet
}

type poolAgg struct {
	positions  int
	playedTop3 int
	demoted    [3]int
	rankSum    [3]int
}

func runCutSweep(g *geom.Geometry, moves, candidates []string) {
	b := rules.NewBoardFor(g)
	for _, name := range moves {
		cell, err := g.ParseCell(name)
		if err != nil {
			fail("cutSweep: cell %s: %v", name, err)
		}
		b.Make(rules.Cell(cell))
	}
	mover := b.Side
	levels := []int{0, 300, 500, 600, 700, 750, 800, 850}
	meP := censusSide(g, b, mover)
	oppP := censusSide(g, b, mover^1)
	fmt.Printf("cutSweep parent mover=%v classesMe=%v classesOpp=%v clean=%d,%d eng3=%d,%d\n",
		mover, meP.classN, oppP.classN, meP.clean, oppP.clean,
		meP.eng2N[config.PatternClassThree], oppP.eng2N[config.PatternClassThree])
	for _, cand := range candidates {
		cell, err := g.ParseCell(cand)
		if err != nil || !b.IsLegal(rules.Cell(cell)) {
			fmt.Printf("  cand %s illegal\n", cand)
			continue
		}
		b.Make(rules.Cell(cell))
		me := censusSide(g, b, mover)
		opp := censusSide(g, b, mover^1)
		base := me.score - opp.score + config.EvalCleanWindowWeightSquare*(me.clean-opp.clean)
		eng := me.eng2N[config.PatternClassThree] - opp.eng2N[config.PatternClassThree]
		nets := make([]string, 0, len(levels))
		for _, lv := range levels {
			nets = append(nets, fmt.Sprintf("%d:%+d", lv, base-lv*eng))
		}
		fmt.Printf("  cand %s base=%+d eng3diff=%+d twoMe(c,e,f)=%d,%d,%d nets=[%s]\n",
			cand, base, eng,
			me.cleanN[config.PatternClassOpenTwo], me.eng2N[config.PatternClassOpenTwo], me.farCont[config.PatternClassOpenTwo],
			strings.Join(nets, " "))
		b.Unmake()
	}
}

func runFamilyProbes(g *geom.Geometry, positionsPath, outPath string) {
	raw, err := os.ReadFile(positionsPath)
	if err != nil {
		fail("read %s: %v", positionsPath, err)
	}
	var pf positionFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		fail("parse %s: %v", positionsPath, err)
	}
	cuts := [3]int{40, 60, 80}
	out, err := os.Create(outPath)
	if err != nil {
		fail("create %s: %v", outPath, err)
	}
	defer out.Close()
	agg := map[string]*poolAgg{}
	for _, pos := range pf.Positions {
		b := rules.NewBoardFor(g)
		for _, name := range pos.Moves {
			cell, err := g.ParseCell(name)
			if err != nil {
				fail("position %s g%d m%d: cell %s: %v", pos.Leg, pos.Game, pos.Move, name, err)
			}
			b.Make(rules.Cell(cell))
		}
		mover := b.Side
		playedCell, err := g.ParseCell(pos.Cell)
		if err != nil || !b.IsLegal(rules.Cell(playedCell)) {
			continue
		}
		me := censusSide(g, b, mover)
		opp := censusSide(g, b, mover^1)
		lines := make([]childLine, 0, 64)
		for cell := range g.Cells {
			if !g.Contains(cell) || !b.IsLegal(rules.Cell(cell)) {
				continue
			}
			b.Make(rules.Cell(cell))
			net, cutNet := boardNet(g, b, mover, cuts)
			b.Unmake()
			lines = append(lines, childLine{name: string(g.AppendName(nil, cell)), net: net, cutNet: cutNet})
		}
		byNet := append([]childLine(nil), lines...)
		sort.Slice(byNet, func(i, j int) bool { return byNet[i].net > byNet[j].net })
		rankIn := func(sorted []childLine, name string) int {
			for i, l := range sorted {
				if l.name == name {
					return i + 1
				}
			}
			return 0
		}
		played := string(g.AppendName(nil, playedCell))
		pr := rankIn(byNet, played)
		cutRanks := [3]int{}
		for k := range cuts {
			byCut := append([]childLine(nil), lines...)
			sort.Slice(byCut, func(i, j int) bool { return byCut[i].cutNet[k] > byCut[j].cutNet[k] })
			cutRanks[k] = rankIn(byCut, played)
		}
		fmt.Fprintf(out, "pos %s g%d m%d pool=%s played=%s rank=%d/%d cutRanks=%v meTwo(c,e,f)=%d,%d,%d oppTwo=%d,%d,%d\n",
			pos.Leg, pos.Game, pos.Move, pos.Pool, played, pr, len(lines), cutRanks,
			me.cleanN[config.PatternClassOpenTwo], me.eng2N[config.PatternClassOpenTwo], me.farCont[config.PatternClassOpenTwo],
			opp.cleanN[config.PatternClassOpenTwo], opp.eng2N[config.PatternClassOpenTwo], opp.farCont[config.PatternClassOpenTwo])
		for i := 0; i < 3 && i < len(byNet); i++ {
			l := byNet[i]
			fmt.Fprintf(out, "  top%d %s net=%d cutNets=%v\n", i+1, l.name, l.net, l.cutNet)
		}
		a := agg[pos.Pool]
		if a == nil {
			a = &poolAgg{}
			agg[pos.Pool] = a
		}
		a.positions++
		if pr <= 3 {
			a.playedTop3++
		}
		for k := range cuts {
			if pr <= 3 && cutRanks[k] > 3 {
				a.demoted[k]++
			}
			a.rankSum[k] += cutRanks[k] - pr
		}
	}
	keys := make([]string, 0, len(agg))
	for k := range agg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := agg[k]
		fmt.Fprintf(out, "agg %s positions=%d playedTop3=%d demotedByCut(40,60,80)=%v rankShiftSum=%v\n", k, a.positions, a.playedTop3, a.demoted, a.rankSum)
	}
}
