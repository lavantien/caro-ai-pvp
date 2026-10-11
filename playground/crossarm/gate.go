package main

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	gateZCut       = 2.496
	gateFloorCIZ   = 1.96
	gateRule0TMax  = 1.0
	gateRule0Ratio = 0.95
)

type gateEntry struct {
	key  string
	dirs []string
}

func parseGateEntries(specs *multiFlag, what string) ([]gateEntry, error) {
	out := make([]gateEntry, 0, len(*specs))
	for _, spec := range *specs {
		key, rest, ok := strings.Cut(spec, "=")
		if !ok || key == "" || rest == "" {
			return nil, fmt.Errorf("--%s wants key=dir[,dir...], got %q", what, spec)
		}
		dirs := strings.Split(rest, ",")
		for i := range dirs {
			dirs[i] = strings.TrimSpace(dirs[i])
			if dirs[i] == "" {
				return nil, fmt.Errorf("--%s wants key=dir[,dir...], got %q", what, spec)
			}
		}
		out = append(out, gateEntry{key: key, dirs: dirs})
	}
	return out, nil
}

type floorStat struct {
	tier       string
	dirs       int
	games      int
	decisive   int
	wins       int
	frac       float64
	ciLo, ciHi float64
	coversHalf bool
}

type familyStat struct {
	lower, upper   string
	floorTier      string
	n              int
	upperWins      int
	pUpper, pLower float64
	se             float64
	zA, zB         float64
	aFired, bCert  bool
	hasFloor       bool
}

func renderGate(armEntries, floorEntries []gateEntry) (string, error) {
	var arms []*armData
	for _, e := range armEntries {
		for _, dir := range e.dirs {
			a, err := loadArm(e.key, dir)
			if err != nil {
				return "", err
			}
			arms = append(arms, a)
		}
	}
	var floorArms []*armData
	for _, e := range floorEntries {
		for _, dir := range e.dirs {
			a, err := loadArm(e.key, dir)
			if err != nil {
				return "", err
			}
			floorArms = append(floorArms, a)
		}
	}
	all := append(slices.Clone(arms), floorArms...)
	if len(all) == 0 {
		return "", fmt.Errorf("gate mode needs at least one run directory")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## gate %d+%d\n\n", all[0].initialMin, all[0].incSec)
	badLines, unfinished, mismatches := 0, 0, 0
	for _, a := range all {
		if a.initialMin != all[0].initialMin || a.incSec != all[0].incSec {
			return "", fmt.Errorf("mixed controls across gate runs: %d+%d and %d+%d",
				all[0].initialMin, all[0].incSec, a.initialMin, a.incSec)
		}
		badLines += a.badLines
		unfinished += len(a.unfinished)
		mismatches += len(a.foldMismatch)
		for _, m := range a.unfinished {
			fmt.Fprintf(&b, "unfinished: %s\n", m)
		}
		for _, m := range a.foldMismatch {
			fmt.Fprintf(&b, "fold mismatch: %s\n", m)
		}
	}
	fmt.Fprintf(&b, "integrity: %d dirs, bad lines %d, unfinished %d, fold mismatches %d\n\n",
		len(all), badLines, unfinished, mismatches)
	if badLines > 0 || unfinished > 0 || mismatches > 0 {
		b.WriteString("verdict: VOID, integrity failed, rerun the failed arms whole\n\n")
		return b.String(), nil
	}
	b.WriteString(renderRule0(all))
	floors := computeFloors(floorEntries, floorArms)
	b.WriteString(renderFloors(floorEntries, floors))
	families := computeFamilies(arms, floors)
	b.WriteString(renderFamilies(families))
	return b.String(), nil
}

func renderRule0(all []*armData) string {
	type pooled struct {
		tier             string
		pond             pondStats
		ratioSum, ratioN float64
	}
	bySeat := map[string]*pooled{}
	for _, a := range all {
		for name, s := range a.seats {
			p := bySeat[name]
			if p == nil {
				p = &pooled{tier: s.tier}
				bySeat[name] = p
			}
			p.pond = p.pond.add(s.pond)
			p.ratioSum += s.tel.ratioSum
			p.ratioN += float64(s.tel.ratioN)
		}
	}
	names := make([]string, 0, len(bySeat))
	for n := range bySeat {
		names = append(names, n)
	}
	slices.Sort(names)
	var b strings.Builder
	b.WriteString("### rule 0 telemetry\n\n")
	b.WriteString("| seat | tier | ponder | t mean | t max | t/alloc | verdict |\n|---|---|---|---|---|---|---|\n")
	for _, n := range names {
		p := bySeat[n]
		ratio := 0.0
		if p.ratioN > 0 {
			ratio = p.ratioSum / p.ratioN
		}
		pass := p.pond.tMax < gateRule0TMax && ratio <= gateRule0Ratio
		verdict := "pass"
		if !pass {
			verdict = "FAIL"
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %.2f | %.2f | %.2f | %s |\n",
			n, p.tier, p.pond.adopt, p.pond.tMean(), p.pond.tMax, ratio, verdict)
	}
	b.WriteString("\n")
	return b.String()
}

func computeFloors(entries []gateEntry, arms []*armData) map[string]*floorStat {
	out := map[string]*floorStat{}
	for _, e := range entries {
		fs := &floorStat{tier: e.key}
		for _, a := range arms {
			if a.label != e.key {
				continue
			}
			fs.dirs++
			for _, g := range a.games {
				if g.redTier != fs.tier || g.blueTier != fs.tier {
					continue
				}
				fs.games++
				if g.outcome == outcomeDraw {
					continue
				}
				fs.decisive++
				winner := g.blue
				if g.outcome == outcomeRed {
					winner = g.red
				}
				if winner == min(g.red, g.blue) {
					fs.wins++
				}
			}
		}
		if fs.decisive > 0 {
			fs.frac = float64(fs.wins) / float64(fs.decisive)
			half := gateFloorCIZ * math.Sqrt(0.25/float64(fs.decisive))
			fs.ciLo, fs.ciHi = fs.frac-half, fs.frac+half
			fs.coversHalf = fs.ciLo <= 0.5 && 0.5 <= fs.ciHi
		}
		out[e.key] = fs
	}
	return out
}

func renderFloors(entries []gateEntry, floors map[string]*floorStat) string {
	var b strings.Builder
	b.WriteString("### floors\n\n")
	b.WriteString("| tier | dirs | games | decisive | frac | 95% ci | covers 0.5 |\n|---|---|---|---|---|---|---|\n")
	for _, e := range entries {
		fs := floors[e.key]
		ci := "-"
		covers := "-"
		if fs.decisive > 0 {
			ci = fmt.Sprintf("%.3f to %.3f", fs.ciLo, fs.ciHi)
			covers = fmt.Sprintf("%v", fs.coversHalf)
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %.3f | %s | %s |\n",
			fs.tier, fs.dirs, fs.games, fs.decisive, fs.frac, ci, covers)
	}
	b.WriteString("\n")
	return b.String()
}

func computeFamilies(arms []*armData, floors map[string]*floorStat) []familyStat {
	present := map[string]bool{}
	for _, a := range arms {
		for _, s := range a.seats {
			present[s.tier] = true
		}
	}
	var out []familyStat
	for i := 0; i+1 < len(config.Tiers); i++ {
		lower, upper := config.Tiers[i].Name, config.Tiers[i+1].Name
		if !present[lower] || !present[upper] {
			continue
		}
		fs := familyStat{lower: lower, upper: upper, floorTier: config.TierHard.Name}
		if upper == config.TierGrandmaster.Name {
			fs.floorTier = config.TierGrandmaster.Name
		}
		for _, a := range arms {
			for _, g := range a.games {
				if g.outcome == outcomeDraw {
					continue
				}
				match := (g.redTier == lower && g.blueTier == upper) || (g.redTier == upper && g.blueTier == lower)
				if !match {
					continue
				}
				fs.n++
				winnerTier := g.blueTier
				if g.outcome == outcomeRed {
					winnerTier = g.redTier
				}
				if winnerTier == upper {
					fs.upperWins++
				}
			}
		}
		fl := floors[fs.floorTier]
		if fl != nil && fl.decisive > 0 && fs.n > 0 {
			fs.hasFloor = true
			fs.pUpper = float64(fs.upperWins) / float64(fs.n)
			fs.pLower = 1 - fs.pUpper
			fs.se = 0.5 * math.Sqrt(1/float64(fs.n)+1/float64(fl.decisive))
			fs.zA = (fs.pLower - fl.frac) / fs.se
			fs.zB = (fs.pUpper - fl.frac) / fs.se
			fs.aFired = fs.zA > gateZCut
			fs.bCert = fs.zB > gateZCut
		}
		out = append(out, fs)
	}
	return out
}

func renderFamilies(families []familyStat) string {
	var b strings.Builder
	b.WriteString("### family reads\n\n")
	b.WriteString("| family | floor | N | upper frac | lower frac | z rule A | z rule B | rule A | rule B |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, fs := range families {
		if !fs.hasFloor {
			fmt.Fprintf(&b, "| %s v %s | %s | %d | - | - | - | - | no floor | no floor |\n",
				fs.upper, fs.lower, fs.floorTier, fs.n)
			continue
		}
		ruleA := "quiet"
		if fs.aFired {
			ruleA = "FIRED"
		}
		ruleB := "unproven"
		if fs.bCert {
			ruleB = "certified"
		}
		fmt.Fprintf(&b, "| %s v %s | %s | %d | %.3f | %.3f | %.2f | %.2f | %s | %s |\n",
			fs.upper, fs.lower, fs.floorTier, fs.n, fs.pUpper, fs.pLower, fs.zA, fs.zB, ruleA, ruleB)
	}
	b.WriteString("\n")
	return b.String()
}
