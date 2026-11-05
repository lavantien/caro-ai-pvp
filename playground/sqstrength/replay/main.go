package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

var game27Moves = [...]string{
	"H8", "H6", "H5", "I6", "J6", "I7", "J8", "J5", "H7", "K8", "I4", "G6",
	"I9", "K7", "J10", "K11", "J9", "J7", "K9", "L9", "M10", "H9", "L10", "K10",
	"M11", "N12", "F6", "G7", "M12", "M7", "L7", "M8", "M13", "M14",
}

func cellName(c rules.Cell) string {
	var buf [8]byte
	out := rules.AppendCellName(buf[:0], c)
	return string(out)
}

func build(moves []string, upto int) *rules.Board {
	b := rules.NewBoard()
	for i := 0; i < upto; i++ {
		cell, err := rules.ParseCell(moves[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "replay: cell %s: %v\n", moves[i], err)
			os.Exit(1)
		}
		b.Make(cell)
	}
	return b
}

func childScore(moves []string, upto int, extra string, depth int) (int, rules.Move, engine.SearchStats) {
	b := build(moves, upto)
	cell, err := rules.ParseCell(extra)
	if err != nil {
		fmt.Fprintf(os.Stderr, "replay: cell %s: %v\n", extra, err)
		os.Exit(1)
	}
	b.Make(cell)
	e := engine.New(1 << 22)
	mv, stats := e.SearchDepth(b, engine.NewFixedBudget(120*time.Second), depth)
	return stats.Score, mv, stats
}

var midgameMoves = [...]string{"H8", "H9", "I8", "I9", "C3", "C4", "D3", "M12", "M13", "N12"}

var game36Moves = [...]string{
	"H8", "H6", "H5", "G6", "F6", "G7", "F8", "F5", "H7", "E8", "G8", "I6", "J8", "I8", "J6", "I7",
	"I9", "I5", "I4", "H10", "J7", "J9", "K3", "J4", "M3", "L3", "L6", "M5", "M7", "N8", "M9", "M8",
	"L8", "L7", "N6", "K9", "K5", "K6", "N9", "K8", "K7", "N10", "I10", "F7", "N5", "N2", "D9", "P4",
	"L12", "K11", "K12", "J11", "L11", "M10", "J12", "I12", "L9", "L10", "O9", "P9", "O10", "H13", "O8", "G14",
	"F15",
}

func main() {
	depthFlag := flag.Int("depth", 8, "max search depth")
	midgame := flag.Bool("midgame", false, "measure the quiet midgame board instead of the recorded board")
	game := flag.Int("game", 27, "recorded game to replay, 27 or 36")
	flag.Parse()
	if *midgame {
		b := build(midgameMoves[:], len(midgameMoves))
		for depth := 6; depth <= *depthFlag; depth += 2 {
			e := engine.New(1 << 22)
			mv, stats := e.SearchDepth(b, engine.NewFixedBudget(120*time.Second), depth)
			fmt.Printf("midgame d=%d pick=%s score=%d nodes=%d\n", depth, cellName(rules.Cell(mv)), stats.Score, stats.Nodes)
		}
		return
	}
	var moves []string
	var played string
	var moverName string
	if *game == 36 {
		moves = game36Moves[:len(game36Moves)]
		played = "H11"
		moverName = "blue"
	} else {
		moves = game27Moves[:]
		played = "I11"
		moverName = "red"
	}
	b := build(moves, len(moves))
	fmt.Printf("game %d board at the flip, side to move %s, stones %d\n", *game, moverName, b.MoveCount)
	for depth := 4; depth <= *depthFlag; depth += 2 {
		e := engine.New(1 << 22)
		mv, stats := e.SearchDepth(b, engine.NewFixedBudget(120*time.Second), depth)
		pv := ""
		for i := 0; i < stats.PVLen; i++ {
			pv += " " + cellName(rules.Cell(stats.PV[i]))
		}
		fmt.Printf("root d=%d pick=%s score=%d nodes=%d pv=%s\n", depth, cellName(rules.Cell(mv)), stats.Score, stats.Nodes, pv)
	}
	for depth := 4; depth <= *depthFlag; depth += 2 {
		score, _, _ := childScore(moves, len(moves), played, depth)
		fmt.Printf("child after the recorded %s d=%d replySideScore=%d moverView=%d\n", played, depth, score, -score)
	}
	after := build(moves, len(moves))
	playedCell, _ := rules.ParseCell(played)
	after.Make(playedCell)
	s := vcf.New(vcf.KindVCF)
	var st vcf.SolverStats
	found := s.Solve(after, config.SolverNodeBudget, nil, &st)
	pv := ""
	for i := 0; i < st.Plies; i++ {
		pv += " " + cellName(rules.Cell(st.PV[i]))
	}
	fmt.Printf("vcf for the reply side after %s found=%v plies=%d nodes=%d pv=%s\n", played, found, st.Plies, st.Nodes, pv)
}
