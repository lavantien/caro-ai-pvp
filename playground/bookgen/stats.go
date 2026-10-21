package main

import (
	"fmt"
	"io"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type plyCount struct {
	unique int
	pass   int
}

type statsResult struct {
	games  int
	unique int
	trunks int
	plies  [config.BookMaxPly]plyCount
}

func runStats(paths []string, floor int, progress io.Writer) (statsResult, error) {
	var res statsResult
	agg := make(map[[config.BookKeyBytes]byte]map[rules.Move]int)
	plyOf := make(map[[config.BookKeyBytes]byte]int)
	trunks := make(map[rules.Move]struct{})
	for _, path := range paths {
		cf, err := parseCorpusFile(path)
		if err != nil {
			return res, err
		}
		if cf.fp == "" {
			return res, fmt.Errorf("bookgen: stats corpus %s holds no corpus fingerprint, regenerate it with -phase corpus", path)
		}
		if cf.torn {
			return res, fmt.Errorf("bookgen: stats corpus %s ends torn, let its writer finish or repair it", path)
		}
		res.games += cf.games
		fmt.Fprintf(progress, "stats %s: %d games\n", path, cf.games)
		for _, blk := range cf.blocks {
			if len(blk.recs) > 0 {
				trunks[blk.recs[0].move] = struct{}{}
			}
			for _, rec := range blk.recs {
				if counts, ok := agg[rec.key]; ok {
					counts[rec.move]++
					continue
				}
				b, err := boardFromKey(rec.key)
				if err != nil {
					return res, fmt.Errorf("bookgen: stats position in %s: %w", path, err)
				}
				ply := b.MoveCount
				if ply >= config.BookMaxPly {
					return res, fmt.Errorf("bookgen: stats position in %s sits at ply %d past the BookMaxPly %d bound", path, ply, config.BookMaxPly)
				}
				plyOf[rec.key] = ply
				agg[rec.key] = map[rules.Move]int{rec.move: 1}
			}
		}
	}
	res.unique = len(agg)
	res.trunks = len(trunks)
	for key, counts := range agg {
		p := plyOf[key]
		res.plies[p].unique++
		if _, n := majorityOf(counts); n >= floor {
			res.plies[p].pass++
		}
	}
	fmt.Fprintf(progress, "stats done: games %d unique %d trunks %d floor %d\n", res.games, res.unique, res.trunks, floor)
	for p := range config.BookMaxPly {
		fmt.Fprintf(progress, "ply %d unique %d pass %d\n", p, res.plies[p].unique, res.plies[p].pass)
	}
	return res, nil
}
