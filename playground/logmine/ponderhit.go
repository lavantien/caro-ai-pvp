package main

import (
	"fmt"
	"sort"
	"strings"
)

type hitKey struct {
	tier, tc string
}

type hitAgg struct {
	predictions   int
	hits          int
	adoptGivenHit int
	missDist      map[string]int
}

func hitStats(rows []moveRow) map[hitKey]hitAgg {
	capable := ponderCapableTiers()
	active := map[hitKey]bool{}
	for _, r := range rows {
		if r.tag == "[PONDER]" && capable[r.tier] {
			active[hitKey{tier: r.tier, tc: r.tc}] = true
		}
	}
	type gameID struct {
		src  string
		game int64
	}
	games := map[gameID][]moveRow{}
	order := []gameID{}
	for _, r := range rows {
		g := gameID{src: r.srcFile, game: r.gameID}
		if _, seen := games[g]; !seen {
			order = append(order, g)
		}
		games[g] = append(games[g], r)
	}
	agg := map[hitKey]*hitAgg{}
	for _, g := range order {
		seq := games[g]
		for i := 0; i+1 < len(seq); i++ {
			cur := seq[i]
			next := seq[i+1]
			k := hitKey{tier: cur.tier, tc: cur.tc}
			if !capable[cur.tier] || !active[k] || len(cur.pvCells) < 2 {
				continue
			}
			if agg[k] == nil {
				agg[k] = &hitAgg{missDist: map[string]int{}}
			}
			a := agg[k]
			a.predictions++
			if cur.pvCells[1] == next.cell {
				a.hits++
				if i+2 < len(seq) && seq[i+2].tier == cur.tier && seq[i+2].tag == "[PONDER]" {
					a.adoptGivenHit++
				}
				continue
			}
			pos := 0
			for j, c := range cur.pvCells {
				if j > 1 && c == next.cell {
					pos = j + 1
					break
				}
			}
			switch {
			case pos == 3:
				a.missDist["pv3"]++
			case pos == 4:
				a.missDist["pv4"]++
			case pos >= 5:
				a.missDist["pv5+"]++
			default:
				a.missDist["not-in-pv"]++
			}
		}
	}
	out := map[hitKey]hitAgg{}
	for k, a := range agg {
		out[k] = *a
	}
	return out
}

func renderHitReport(source string, files int, agg map[hitKey]hitAgg) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# ponder hit rate: %s\n\nsource: %s\nseries log files: %d\n\n", source, source, files)
	b.WriteString("method: within each game, compare pv[1] of turn n (the predicted reply) against the played cell of turn n+1. Opportunities are turns of a ponder-capable tier (config PonderThreads > 0) with a pv of at least 2 cells, in a series where that tier shows at least one adopted [PONDER] answer. Adopted-given-hit requires the ponderer's next move in the same game to carry [PONDER]. Caveats: the pv truncates near the search horizon so not-in-pv overstates true distance, a wrong prediction can still warm the tree, and turns where the ponder lane was idle by admission are indistinguishable from gate refusals in the logs.\n\n")
	keys := make([]hitKey, 0, len(agg))
	for k := range agg {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].tier != keys[j].tier {
			return keys[i].tier < keys[j].tier
		}
		return keys[i].tc < keys[j].tc
	})
	b.WriteString("## hit rate and conditional adoption\n\n")
	b.WriteString("| tier | tc | predictions | hits | hit rate | adopted after hit | adopted / hits |\n|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		a := agg[k]
		rate := 0.0
		if a.predictions > 0 {
			rate = float64(a.hits) / float64(a.predictions)
		}
		cond := 0.0
		if a.hits > 0 {
			cond = float64(a.adoptGivenHit) / float64(a.hits)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %.1f%% | %d | %.1f%% |\n",
			k.tier, k.tc, a.predictions, a.hits, rate*100, a.adoptGivenHit, cond*100)
	}
	b.WriteString("\n## miss distance, share of misses by where the actual reply sat in the predicting pv\n\n")
	b.WriteString("| tier | tc | misses | pv3 | pv4 | pv5+ | not-in-pv |\n|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		a := agg[k]
		misses := a.predictions - a.hits
		share := func(bucket string) string {
			if misses == 0 {
				return "-"
			}
			return fmt.Sprintf("%.1f%%", float64(a.missDist[bucket])/float64(misses)*100)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %s | %s | %s |\n",
			k.tier, k.tc, misses, share("pv3"), share("pv4"), share("pv5+"), share("not-in-pv"))
	}
	return b.String()
}
