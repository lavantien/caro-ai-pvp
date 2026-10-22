package main

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func runSearchProbe() error {
	b, err := midgameBoard()
	if err != nil {
		return err
	}
	fmt.Printf("search probe: fixed midgame, fresh engine per rep, %d reps, TT %d MiB\n", searchReps, searchTTMiB)
	fmt.Println("depth   median_nodes   median_ms   median_knps   last_score")
	for _, depth := range searchDepths {
		var nodes, elapsed []float64
		var last engine.SearchStats
		for range searchReps {
			e := engine.New(int64(searchTTMiB) << 20)
			dl := engine.NewStoppable()
			mv, stats := e.SearchDepth(b, dl, depth)
			if !b.IsLegal(rules.Cell(mv)) {
				return fmt.Errorf("depth %d: illegal move %d", depth, mv)
			}
			nodes = append(nodes, float64(stats.Nodes))
			elapsed = append(elapsed, float64(stats.ElapsedNs)/1e6)
			last = stats
		}
		knps := medianF(nodes) / (medianF(elapsed) / 1000) / 1000
		fmt.Printf("%5d   %12.0f   %9.1f   %11.1f   %10d\n", depth, medianF(nodes), medianF(elapsed), knps, last.Score)
	}
	return nil
}
