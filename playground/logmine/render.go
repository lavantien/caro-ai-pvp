package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type mined struct {
	tournament []moveRow
	casual     []moveRow
	rejects    int
	files      int
	firstWins  []firstWins
	outcomes   []outcomeRow
}

func mine(db *sql.DB) mined {
	var m mined
	m.tournament, _ = loadTournamentMoves(db)
	m.firstWins, _ = loadSeriesFirstWins(db)
	m.outcomes, _ = loadGameOutcomes(db)
	m.casual, m.rejects, _ = loadCasualMoves(db)
	return m
}

func sanitizeName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".db")
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." {
		return base
	}
	return strings.ReplaceAll(dir, "/", "__") + "__" + base
}

func fmtSpread(s spread, scale float64, digits int) string {
	return fmt.Sprintf("%.*f/%.*f/%.*f", digits, s.q1*scale, digits, s.med*scale, digits, s.q3*scale)
}

func fmtCount(s spread) string {
	return fmtSpread(s, 1, 0)
}

func fmtPermille(s spread) string {
	return fmtSpread(s, 0.1, 1)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func telemetryTable(title string, rows []moveRow) string {
	if len(rows) == 0 {
		return ""
	}
	tele := tierTelemetry(rows)
	keys := make([]telemetryKey, 0, len(tele))
	for k := range tele {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].tier != keys[j].tier {
			return keys[i].tier < keys[j].tier
		}
		return keys[i].tc < keys[j].tc
	})
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", title)
	b.WriteString("| tier | tc | games | moves | depth q1/med/q3 | nodes med | nps med | tt% med | hf% med | fh1% med | ebf med |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		a := tele[k]
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %s | %s | %s | %s | %s | %s | %s |\n",
			k.tier, k.tc, a.games, a.moves,
			fmtCount(a.depth), fmtCount(a.nodes), fmtCount(a.nps),
			fmtPermille(a.tt), fmtPermille(a.hf), fmtPermille(a.fh1),
			fmtSpread(a.ebf, 0.001, 1))
	}
	b.WriteString("\n")
	return b.String()
}

func grantDepthTable(rows []moveRow) string {
	if len(rows) == 0 {
		return ""
	}
	gd := grantDepth(rows)
	keys := make([]grantKey, 0, len(gd))
	for k := range gd {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].tier != keys[j].tier {
			return keys[i].tier < keys[j].tier
		}
		return keys[i].bucket < keys[j].bucket
	})
	var b strings.Builder
	b.WriteString("## grant x depth\n\n")
	b.WriteString("| tier | grant s | moves | depth q1/med/q3 |\n|---|---|---|---|\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "| %s | %s | %d | %s |\n", k.tier, k.bucket, gd[k].moves, fmtCount(gd[k].depth))
	}
	b.WriteString("\n")
	return b.String()
}

func renderReport(name, source string, m mined) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# logmine: %s\n\nsource: %s\n\n", name, source)
	total := len(m.tournament) + len(m.casual)
	if total == 0 && len(m.outcomes) == 0 {
		b.WriteString("no move data\n")
		return b.String()
	}
	fmt.Fprintf(&b, "moves: %d tournament, %d casual, %d lines rejected\n\n", len(m.tournament), len(m.casual), m.rejects)
	if m.files > 0 {
		fmt.Fprintf(&b, "series log files parsed: %d\n\n", m.files)
	}
	b.WriteString(telemetryTable("tier x time control", m.tournament))
	b.WriteString(grantDepthTable(m.tournament))
	if len(m.firstWins) > 0 {
		b.WriteString("## red first (tournament series)\n\n")
		b.WriteString("| tc | first tier | second tier | first wins | second wins |\n|---|---|---|---|---|\n")
		for _, f := range m.firstWins {
			fmt.Fprintf(&b, "| %s | %s | %s | %d | %d |\n", f.tc, f.firstTier, f.secondTier, f.firstWins, f.secondWins)
		}
		b.WriteString("\n")
	}
	if len(m.outcomes) > 0 {
		b.WriteString("## game outcomes\n\n")
		b.WriteString("| tc | red tier | blue tier | outcome | count |\n|---|---|---|---|---|\n")
		for _, o := range m.outcomes {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %d |\n", o.key.tc, o.key.redTier, o.key.blueTier, o.key.outcome, o.count)
		}
		b.WriteString("\n")
	}
	if len(m.tournament) > 0 {
		usage := grantUsage(m.tournament)
		keys := make([]telemetryKey, 0, len(usage))
		for k := range usage {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].tier != keys[j].tier {
				return keys[i].tier < keys[j].tier
			}
			return keys[i].tc < keys[j].tc
		})
		b.WriteString("## grant usage\n\n")
		b.WriteString("| tier | tc | moves | used q1/med/q3 | under 10% | over 100% |\n|---|---|---|---|---|---|\n")
		for _, k := range keys {
			u := usage[k]
			fmt.Fprintf(&b, "| %s | %s | %d | %s | %d | %d |\n", k.tier, k.tc, u.moves, fmtSpread(u.used, 1, 2), u.under10, u.over100)
		}
		b.WriteString("\n")
		tagCounts := map[string]int{}
		for _, r := range m.tournament {
			tag := r.tag
			if tag == "" {
				tag = "-"
			}
			tagCounts[tag]++
		}
		b.WriteString("## tags\n\n")
		b.WriteString("| tag | moves |\n|---|---|\n")
		for _, tag := range sortedKeys(tagCounts) {
			fmt.Fprintf(&b, "| %s | %d |\n", tag, tagCounts[tag])
		}
		b.WriteString("\n")
	}
	b.WriteString(telemetryTable("casual m-lines", m.casual))
	return b.String()
}
