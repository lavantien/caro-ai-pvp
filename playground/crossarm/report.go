package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func fmtScore(s int) string {
	if s >= mateFloor {
		return fmt.Sprintf("M%d", (config.EvalMateMax-s)/config.EvalMateScoreStep)
	}
	if s <= -mateFloor {
		return fmt.Sprintf("-M%d", (config.EvalMateMax+s)/config.EvalMateScoreStep)
	}
	return fmt.Sprintf("%d", s)
}

func renderArm(a *armData, focus []string) string {
	var b strings.Builder
	decisive, draws := a.totals()
	zero := "holds"
	if !a.zeroSumHolds() {
		zero = "BROKEN"
	}
	fmt.Fprintf(&b, "## arm %s\n\n", a.label)
	fmt.Fprintf(&b, "dir %s, files %d, series %d, games %d (%d decisive, %d drawn), zero-sum %s, bad lines %d, unfinished %d, fold mismatches %d\n\n",
		a.dir, a.files, a.series, len(a.games), decisive, draws, zero, a.badLines, len(a.unfinished), len(a.foldMismatch))
	for _, m := range a.foldMismatch {
		fmt.Fprintf(&b, "fold mismatch: %s\n", m)
	}
	for _, m := range a.unfinished {
		fmt.Fprintf(&b, "unfinished: %s\n", m)
	}
	for _, s := range a.badSamples {
		fmt.Fprintf(&b, "bad line: %s\n", s)
	}
	b.WriteString("\n### standings\n\n")
	seats := a.seatList()
	b.WriteString("| seat | tier | series | games | red | blue |\n|---|---|---|---|---|---|\n")
	for _, s := range seats {
		fmt.Fprintf(&b, "| %s | %s | %d-%d | %s | %s | %s |\n",
			s.name, s.tier, s.seriesW, s.seriesL, s.games, s.red, s.blue)
	}
	tierCols := armTierColumns(a)
	b.WriteString("\n### per-seat records by opponent tier\n\n| seat |")
	for _, t := range tierCols {
		fmt.Fprintf(&b, " vs %s |", t)
	}
	b.WriteString("\n|")
	for range tierCols {
		b.WriteString("---|")
	}
	b.WriteString("---|\n")
	for _, s := range seats {
		fmt.Fprintf(&b, "| %s |", s.name)
		for _, t := range tierCols {
			fmt.Fprintf(&b, " %s |", s.vsTier[t])
		}
		b.WriteString("\n")
	}
	b.WriteString("\n### series matrix, row winner over column\n\n")
	renderMatrix(&b, seats, func(row, col *seatStats) string {
		r := row.seriesBy[col.name]
		if r.empty() {
			return ""
		}
		return fmt.Sprintf("%d-%d", r.w, r.l)
	})
	b.WriteString("\n### games matrix, row over column\n\n")
	renderMatrix(&b, seats, func(row, col *seatStats) string {
		return row.vsSeat[col.name].String()
	})
	b.WriteString("\n### per-seat telemetry, untagged searches\n\n")
	b.WriteString("| seat | moves | tagged | d mean | d med | t mean | alloc mean | t/alloc | nps mean | n mean |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, s := range seats {
		fmt.Fprintf(&b, "| %s | %d | %d | %.1f | %.1f | %.2f | %.2f | %.2f | %.2fm | %.2fm |\n",
			s.name, s.tel.lines, s.tel.tagged, s.tel.depthMean(), s.tel.depthMedian(),
			s.tel.tMean(), s.tel.allocMean(), s.tel.ratioMean(), s.tel.npsMean()/1e6, s.tel.nodesMean()/1e6)
	}
	for _, name := range focus {
		s, ok := a.seats[name]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "\n### %s phase telemetry\n\n| phase | moves | d mean | t mean | alloc mean | t/alloc |\n|---|---|---|---|---|---|\n", name)
		for p, label := range []string{"early", "mid", "late"} {
			t := s.phase[p]
			fmt.Fprintf(&b, "| %s | %d | %.1f | %.2f | %.2f | %.2f |\n",
				label, t.lines, t.depthMean(), t.tMean(), t.allocMean(), t.ratioMean())
		}
		fmt.Fprintf(&b, "\n### %s losses\n\n| series | game | opponent | tier | color | moves | won by | own final s | opp final s |\n|---|---|---|---|---|---|---|---|---|\n", name)
		for _, g := range a.games {
			loser, _, decisive := g.loser()
			if !decisive || loser != name {
				continue
			}
			opp := g.blue
			if name == g.blue {
				opp = g.red
			}
			oppLast := g.blueLast
			if name == g.blue {
				oppLast = g.redLast
			}
			wb := g.wonBy
			if wb == "" {
				wb = "-"
			}
			fmt.Fprintf(&b, "| s%02d | %d | %s | %s | %s | %d | %s | %s | %s |\n",
				g.series, g.gameNo, opp, tierOfGame(g, opp), g.colorOf(name), g.moves, wb,
				fmtScore(g.lastScoreOf(name)), fmtScore(oppLast))
		}
	}
	dp := a.drawsByTierPair()
	if len(dp) > 0 {
		b.WriteString("\n### draws by tier pair\n\n")
		for _, t := range tierCols {
			for _, u := range tierCols {
				key := t + "-" + u
				if n, ok := dp[key]; ok {
					fmt.Fprintf(&b, "%s: %d\n", key, n)
				}
			}
		}
	}
	b.WriteString("\n")
	return b.String()
}

func tierOfGame(g gameRec, name string) string {
	if name == g.red {
		return g.redTier
	}
	return g.blueTier
}

func armTierColumns(a *armData) []string {
	present := map[string]bool{}
	for _, s := range a.seats {
		for t := range s.vsTier {
			present[t] = true
		}
		present[s.tier] = true
	}
	var cols []string
	for i := range config.Tiers {
		if present[config.Tiers[i].Name] {
			cols = append(cols, config.Tiers[i].Name)
		}
	}
	return cols
}

func renderMatrix(b *strings.Builder, seats []*seatStats, cell func(row, col *seatStats) string) {
	b.WriteString("| seat |")
	for _, s := range seats {
		fmt.Fprintf(b, " %s |", s.name)
	}
	b.WriteString("\n|---|")
	for range seats {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, row := range seats {
		fmt.Fprintf(b, "| %s |", row.name)
		for _, col := range seats {
			c := cell(row, col)
			if c == "0-0-0" || c == "0-0" {
				c = ""
			}
			fmt.Fprintf(b, " %s |", c)
		}
		b.WriteString("\n")
	}
}

func renderSynthesis(arms []*armData) string {
	var b strings.Builder
	b.WriteString("## cross-arm synthesis\n\n")
	labels := make([]string, len(arms))
	byLabel := map[string]*armData{}
	for i, a := range arms {
		labels[i] = a.label
		byLabel[a.label] = a
	}
	b.WriteString("### same-tier mutuals, first seat's perspective\n\n| pair |")
	for _, l := range labels {
		fmt.Fprintf(&b, " %s |", l)
	}
	b.WriteString("\n|")
	for range labels {
		b.WriteString("---|")
	}
	b.WriteString("---|\n")
	for _, t := range synthesisTiers(arms) {
		var seats []string
		for _, a := range arms {
			for _, s := range a.seatList() {
				if s.tier == t && !slices.Contains(seats, s.name) {
					seats = append(seats, s.name)
				}
			}
		}
		for i := 0; i < len(seats); i++ {
			for j := i + 1; j < len(seats); j++ {
				fmt.Fprintf(&b, "| %s v %s |", seats[i], seats[j])
				for _, l := range labels {
					a := byLabel[l]
					s := a.seats[seats[i]]
					if s == nil {
						fmt.Fprintf(&b, " |")
						continue
					}
					sr := s.seriesBy[seats[j]]
					fmt.Fprintf(&b, " s%d-%d g%s |", sr.w, sr.l, s.vsSeat[seats[j]])
				}
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("\n### adjacent-tier aggregates, upper tier's perspective\n\n| pair |")
	for _, l := range labels {
		fmt.Fprintf(&b, " %s |", l)
	}
	b.WriteString("\n|")
	for range labels {
		b.WriteString("---|")
	}
	b.WriteString("---|\n")
	tiers := synthesisTiers(arms)
	for i := 0; i < len(tiers)-1; i++ {
		upper, lower := tiers[i], tiers[i+1]
		fmt.Fprintf(&b, "| %s v %s |", upper, lower)
		for _, l := range labels {
			a := byLabel[l]
			var games wld
			var series wld
			for _, s := range a.seatList() {
				if s.tier != upper {
					continue
				}
				for _, o := range a.seatList() {
					if o.tier != lower {
						continue
					}
					games = games.add(s.vsSeat[o.name])
					series = series.add(s.seriesBy[o.name])
				}
			}
			fmt.Fprintf(&b, " s%d-%d g%s |", series.w, series.l, games)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func synthesisTiers(arms []*armData) []string {
	present := map[string]bool{}
	for _, a := range arms {
		for _, s := range a.seats {
			present[s.tier] = true
		}
	}
	var out []string
	for i := range config.Tiers {
		if present[config.Tiers[i].Name] {
			out = append(out, config.Tiers[i].Name)
		}
	}
	return out
}
