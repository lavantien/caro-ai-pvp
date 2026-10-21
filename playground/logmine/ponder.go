package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type ponderKey struct {
	tier, tc string
}

type ponderAgg struct {
	opportunities int
	adopted       int
	adoptMate     int
	adoptT        spread
	adoptedDepth  spread
	untaggedDepth spread
	adoptedNodes  spread
	untaggedNodes spread
	clause1Pass   int
	clause1Total  int
}

type ponderRaw struct {
	adoptElapsed  []float64
	adoptDepth    []float64
	adoptNodes    []float64
	untaggedDepth []float64
	untaggedNodes []float64
}

func ponderCapableTiers() map[string]bool {
	out := map[string]bool{}
	for i := range config.Tiers {
		if config.Tiers[i].PonderThreads > 0 {
			out[config.Tiers[i].Name] = true
		}
	}
	return out
}

func isMateScore(txt string) bool {
	rest := strings.TrimPrefix(strings.TrimPrefix(txt, "+"), "-")
	if strings.HasPrefix(rest, "M") {
		return true
	}
	var v int
	if _, err := fmt.Sscanf(txt, "%d", &v); err != nil {
		return false
	}
	band := config.EvalMateMax - config.SearchMaxPly*config.EvalMateScoreStep
	return v > band || v < -band
}

func ponderStats(rows []moveRow) map[ponderKey]ponderAgg {
	capable := ponderCapableTiers()
	adoptedBy := map[ponderKey]bool{}
	for _, r := range rows {
		if r.tag == "[PONDER]" && capable[r.tier] {
			adoptedBy[ponderKey{tier: r.tier, tc: r.tc}] = true
		}
	}
	raw := map[ponderKey]*ponderRaw{}
	baseline := map[ponderKey]map[string][]float64{}
	for _, r := range rows {
		if !capable[r.tier] {
			continue
		}
		k := ponderKey{tier: r.tier, tc: r.tc}
		if !adoptedBy[k] {
			continue
		}
		if raw[k] == nil {
			raw[k] = &ponderRaw{}
			baseline[k] = map[string][]float64{}
		}
		w := raw[k]
		if r.tag == "[PONDER]" {
			w.adoptElapsed = append(w.adoptElapsed, float64(r.elapsedNS)/1e9)
			w.adoptDepth = append(w.adoptDepth, float64(r.depth))
			w.adoptNodes = append(w.adoptNodes, float64(r.nodes))
			continue
		}
		w.untaggedDepth = append(w.untaggedDepth, float64(r.depth))
		w.untaggedNodes = append(w.untaggedNodes, float64(r.nodes))
		b := allocBucketLabel(r.allocNS)
		baseline[k][b] = append(baseline[k][b], float64(r.depth))
	}
	out := map[ponderKey]ponderAgg{}
	for k, w := range raw {
		adopted := len(w.adoptDepth)
		agg := ponderAgg{
			opportunities: adopted + len(w.untaggedDepth),
			adopted:       adopted,
			adoptT:        quantiles(w.adoptElapsed),
			adoptedDepth:  quantiles(w.adoptDepth),
			untaggedDepth: quantiles(w.untaggedDepth),
			adoptedNodes:  quantiles(w.adoptNodes),
			untaggedNodes: quantiles(w.untaggedNodes),
		}
		for _, r := range rows {
			if r.tier != k.tier || r.tc != k.tc || r.tag != "[PONDER]" {
				continue
			}
			agg.clause1Total++
			if isMateScore(r.scoreTxt) {
				agg.adoptMate++
			}
			ds := baseline[k][allocBucketLabel(r.allocNS)]
			if len(ds) == 0 {
				continue
			}
			med := quantiles(ds).med
			if float64(r.depth) >= med {
				agg.clause1Pass++
			}
		}
		out[k] = agg
	}
	return out
}

func renderPonderReport(source string, files int, stats map[ponderKey]ponderAgg) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# ponder adoption: %s\n\nsource: %s\nseries log files: %d\n\n", source, source, files)
	b.WriteString("definition: opportunities are all moves by a ponder-capable tier (config PonderThreads > 0) in a series where that tier shows at least one adopted [PONDER] answer. The logs carry no lane start or stop record, so a tier with ponder disabled all series would not appear here, and the count includes the seat's first move of a game where no ponder ran, bounded by the game count.\n\n")
	keys := make([]ponderKey, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].tier != keys[j].tier {
			return keys[i].tier < keys[j].tier
		}
		return keys[i].tc < keys[j].tc
	})
	b.WriteString("## adoption rate\n\n")
	b.WriteString("| tier | tc | opportunities | adopted | rate | adopted t q1/med/q3 s |\n|---|---|---|---|---|---|\n")
	for _, k := range keys {
		a := stats[k]
		rate := 0.0
		if a.opportunities > 0 {
			rate = float64(a.adopted) / float64(a.opportunities)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %.1f%% | %s |\n",
			k.tier, k.tc, a.opportunities, a.adopted, rate*100, fmtSpread(a.adoptT, 1, 2))
	}
	b.WriteString("\n## adopted vs normal searches\n\n")
	b.WriteString("| tier | tc | depth adopted | depth normal | nodes adopted | nodes normal | adopted mate share |\n|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		a := stats[k]
		mateShare := 0.0
		if a.adopted > 0 {
			mateShare = float64(a.adoptMate) / float64(a.adopted)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %.1f%% |\n",
			k.tier, k.tc, fmtCount(a.adoptedDepth), fmtCount(a.untaggedDepth),
			fmtCount(a.adoptedNodes), fmtCount(a.untaggedNodes), mateShare*100)
	}
	b.WriteString("\n## adoption law clauses against PonderAdoptFraction 0.5, PonderStableIters 3, PonderScoreDropMargin 300\n\n")
	b.WriteString("clause 1 (depth adequacy, form A: adopted depth at or above the normal-search median depth for the same tier, tc, and grant bucket) is the only clause the logs can decide. Form B (ponder elapsed at or above half the grant) needs ponder elapsed, which an adopted line does not carry: t is turn wall.\n\n")
	b.WriteString("| tier | tc | adopted evaluable | passes form A |\n|---|---|---|---|\n")
	for _, k := range keys {
		a := stats[k]
		fmt.Fprintf(&b, "| %s | %s | %d | %d |\n", k.tier, k.tc, a.clause1Total, a.clause1Pass)
	}
	b.WriteString("\nclause 2 (best move stable over the last 3 completed iterations), clause 3 (score drop within 300 milliunits across recent iterations), and clause 4 (no best-move change in the final iteration) need the root-iteration history ring that the v0.25 engine layer introduces. Not decidable from logs.\n")
	return b.String()
}
