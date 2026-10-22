package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type priorsTable struct {
	games  int
	counts map[[config.BookKeyBytes]byte]map[rules.Move]int
}

type rankedMove struct {
	Move  rules.Move
	Score int
}

func canonicalEntry(k [config.BookKeyBytes]byte, mv rules.Move) ([config.BookKeyBytes]byte, rules.Move) {
	best := 0
	bestKey := k
	for t := 1; t < dihedralOrder; t++ {
		if c := transformKey(t, k); keyBefore(c, bestKey) {
			bestKey = c
			best = t
		}
	}
	return bestKey, transformMove(best, mv)
}

func loadPriors(paths []string) (priorsTable, error) {
	pt := priorsTable{counts: make(map[[config.BookKeyBytes]byte]map[rules.Move]int)}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			return priorsTable{}, fmt.Errorf("bookgen: priors %s: %w", p, err)
		}
		sc, err := loadCorpus(p)
		if err != nil {
			return priorsTable{}, err
		}
		pt.games += sc.games
		for _, r := range sc.recs {
			ck, cmv := canonicalEntry(r.key, r.move)
			counts, ok := pt.counts[ck]
			if !ok {
				counts = make(map[rules.Move]int)
				pt.counts[ck] = counts
			}
			counts[cmv]++
		}
	}
	return pt, nil
}

func (pt priorsTable) lookup(canon [config.BookKeyBytes]byte) map[rules.Move]int {
	return pt.counts[canon]
}

func (pt priorsTable) bonus(canon [config.BookKeyBytes]byte, mv rules.Move) int {
	counts := pt.lookup(canon)
	if counts == nil {
		return 0
	}
	return counts[mv]
}

func rankedByScoreMoves(rm []rankedMove, canon [config.BookKeyBytes]byte, pt priorsTable) []rankedMove {
	out := append([]rankedMove(nil), rm...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		bi := pt.bonus(canon, out[i].Move)
		bj := pt.bonus(canon, out[j].Move)
		if bi != bj {
			return bi > bj
		}
		return out[i].Move < out[j].Move
	})
	return out
}
