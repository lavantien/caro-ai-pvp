package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	phaseEarlyMax = 20
	phaseMidMax   = 40
	phaseCount    = 3
)

const (
	outcomeRed      = "red"
	outcomeBlue     = "blue"
	outcomeDraw     = "draw"
	wonByFour       = "4"
	wonByOpenFour   = "open 4"
	wonByDoubleFour = "double 4"
)

type wld struct {
	w, l, d int
}

func (a wld) add(b wld) wld {
	return wld{w: a.w + b.w, l: a.l + b.l, d: a.d + b.d}
}

func (a wld) String() string {
	return fmt.Sprintf("%d-%d-%d", a.w, a.l, a.d)
}

func (a wld) empty() bool {
	return a.w == 0 && a.l == 0 && a.d == 0
}

type telStats struct {
	lines    int
	tagged   int
	depths   []int
	ts       []float64
	tSum     float64
	allocSum float64
	ratioSum float64
	ratioN   int
	npsSum   float64
	nodesSum float64
}

func (t *telStats) add(m *mLine) {
	if m.tag != "" {
		t.tagged++
		return
	}
	t.lines++
	t.depths = append(t.depths, m.depth)
	t.ts = append(t.ts, m.t)
	t.tSum += m.t
	t.allocSum += m.alloc
	if m.alloc > 0 {
		t.ratioSum += m.t / m.alloc
		t.ratioN++
	}
	t.npsSum += float64(m.nps)
	t.nodesSum += float64(m.nodes)
}

func (t telStats) depthMean() float64 {
	if t.lines == 0 {
		return 0
	}
	sum := 0
	for _, d := range t.depths {
		sum += d
	}
	return float64(sum) / float64(t.lines)
}

func (t telStats) depthMedian() float64 {
	if t.lines == 0 {
		return 0
	}
	c := slices.Clone(t.depths)
	slices.Sort(c)
	if len(c)%2 == 1 {
		return float64(c[len(c)/2])
	}
	return float64(c[len(c)/2-1]+c[len(c)/2]) / 2
}

func (t telStats) tMean() float64 {
	if t.lines == 0 {
		return 0
	}
	return t.tSum / float64(t.lines)
}

func (t telStats) allocMean() float64 {
	if t.lines == 0 {
		return 0
	}
	return t.allocSum / float64(t.lines)
}

func (t telStats) ratioMean() float64 {
	if t.ratioN == 0 {
		return 0
	}
	return t.ratioSum / float64(t.ratioN)
}

func (t telStats) npsMean() float64 {
	if t.lines == 0 {
		return 0
	}
	return t.npsSum / float64(t.lines)
}

func (t telStats) nodesMean() float64 {
	if t.lines == 0 {
		return 0
	}
	return t.nodesSum / float64(t.lines)
}

type seatStats struct {
	name, tier string
	seriesW    int
	seriesL    int
	games      wld
	vsTier     map[string]wld
	vsSeat     map[string]wld
	seriesBy   map[string]wld
	red        wld
	blue       wld
	tel        telStats
	phase      [phaseCount]telStats
}

type gameRec struct {
	arm      string
	series   int64
	gameNo   int
	red      string
	blue     string
	redTier  string
	blueTier string
	outcome  string
	moves    int
	wonBy    string
	redLast  int
	blueLast int
}

func (g gameRec) loser() (string, string, bool) {
	switch g.outcome {
	case outcomeRed:
		return g.blue, g.blueTier, true
	case outcomeBlue:
		return g.red, g.redTier, true
	}
	return "", "", false
}

func (g gameRec) colorOf(name string) string {
	if name == g.red {
		return "red"
	}
	return "blue"
}

func (g gameRec) lastScoreOf(name string) int {
	if name == g.red {
		return g.redLast
	}
	return g.blueLast
}

type armData struct {
	label        string
	dir          string
	files        int
	series       int
	games        []gameRec
	seats        map[string]*seatStats
	badLines     int
	badSamples   []string
	unfinished   []string
	foldMismatch []string
}

func tierRank(name string) int {
	for i := range config.Tiers {
		if config.Tiers[i].Name == name {
			return i
		}
	}
	return -1
}

func loadArm(label, dir string) (*armData, error) {
	paths, err := readSeriesLogs(dir)
	if err != nil {
		return nil, err
	}
	a := &armData{label: label, dir: dir, files: len(paths), seats: map[string]*seatStats{}}
	seat := func(name, tier string) *seatStats {
		if s, ok := a.seats[name]; ok {
			return s
		}
		s := &seatStats{name: name, tier: tier, vsTier: map[string]wld{}, vsSeat: map[string]wld{}, seriesBy: map[string]wld{}}
		a.seats[name] = s
		return s
	}
	for _, path := range paths {
		f, err := parseFile(path)
		if err != nil {
			return nil, err
		}
		a.badLines += len(f.bad)
		for _, b := range f.bad {
			if len(a.badSamples) < 5 {
				a.badSamples = append(a.badSamples, fmt.Sprintf("%s:%d %s", b.path, b.lineno, b.text))
			}
		}
		if !f.hasHead {
			return nil, fmt.Errorf("series log without a full header: %s", path)
		}
		a.series++
		pf := seat(f.head.redName, f.head.redTier)
		ps := seat(f.head.blueName, f.head.blueTier)
		redIsFirst := true
		firstWins, secondWins := 0, 0
		redLast, blueLast := 0, 0
		flush := func(gl *gameLine) {
			redSeat, blueSeat := pf, ps
			if !redIsFirst {
				redSeat, blueSeat = ps, pf
			}
			g := gameRec{
				arm: a.label, series: f.head.series, gameNo: gl.game,
				red: redSeat.name, blue: blueSeat.name,
				redTier: redSeat.tier, blueTier: blueSeat.tier,
				outcome: gl.outcome, moves: gl.moves, wonBy: gl.wonBy,
				redLast: redLast, blueLast: blueLast,
			}
			a.games = append(a.games, g)
			switch gl.outcome {
			case outcomeRed:
				redSeat.games.w++
				blueSeat.games.l++
				redSeat.vsTier[blueSeat.tier] = redSeat.vsTier[blueSeat.tier].add(wld{w: 1})
				blueSeat.vsTier[redSeat.tier] = blueSeat.vsTier[redSeat.tier].add(wld{l: 1})
				redSeat.vsSeat[blueSeat.name] = redSeat.vsSeat[blueSeat.name].add(wld{w: 1})
				blueSeat.vsSeat[redSeat.name] = blueSeat.vsSeat[redSeat.name].add(wld{l: 1})
				redSeat.red = redSeat.red.add(wld{w: 1})
				blueSeat.blue = blueSeat.blue.add(wld{l: 1})
				if redSeat == pf {
					firstWins++
				} else {
					secondWins++
				}
			case outcomeBlue:
				blueSeat.games.w++
				redSeat.games.l++
				blueSeat.vsTier[redSeat.tier] = blueSeat.vsTier[redSeat.tier].add(wld{w: 1})
				redSeat.vsTier[blueSeat.tier] = redSeat.vsTier[blueSeat.tier].add(wld{l: 1})
				blueSeat.vsSeat[redSeat.name] = blueSeat.vsSeat[redSeat.name].add(wld{w: 1})
				redSeat.vsSeat[blueSeat.name] = redSeat.vsSeat[blueSeat.name].add(wld{l: 1})
				blueSeat.blue = blueSeat.blue.add(wld{w: 1})
				redSeat.red = redSeat.red.add(wld{l: 1})
				if blueSeat == pf {
					firstWins++
				} else {
					secondWins++
				}
			default:
				redSeat.games.d++
				blueSeat.games.d++
				redSeat.vsTier[blueSeat.tier] = redSeat.vsTier[blueSeat.tier].add(wld{d: 1})
				blueSeat.vsTier[redSeat.tier] = blueSeat.vsTier[redSeat.tier].add(wld{d: 1})
				redSeat.vsSeat[blueSeat.name] = redSeat.vsSeat[blueSeat.name].add(wld{d: 1})
				blueSeat.vsSeat[redSeat.name] = blueSeat.vsSeat[redSeat.name].add(wld{d: 1})
				redSeat.red = redSeat.red.add(wld{d: 1})
				blueSeat.blue = blueSeat.blue.add(wld{d: 1})
			}
			redIsFirst = !redIsFirst
			redLast, blueLast = 0, 0
		}
		for _, r := range f.records {
			if r.ml != nil {
				redSeat, blueSeat := pf, ps
				if !redIsFirst {
					redSeat, blueSeat = ps, pf
				}
				mover := redSeat
				if r.ml.side != "Red" {
					mover = blueSeat
				}
				mover.tel.add(r.ml)
				mover.phase[phaseOf(r.ml.move)].add(r.ml)
				if r.ml.side == "Red" {
					redLast = r.ml.score
				} else {
					blueLast = r.ml.score
				}
				continue
			}
			flush(r.gl)
		}
		if f.verdict == nil {
			a.unfinished = append(a.unfinished, path)
			continue
		}
		switch f.verdict.winner {
		case f.head.redName:
			pf.seriesW++
			ps.seriesL++
			pf.seriesBy[ps.name] = pf.seriesBy[ps.name].add(wld{w: 1})
			ps.seriesBy[pf.name] = ps.seriesBy[pf.name].add(wld{l: 1})
		case f.head.blueName:
			ps.seriesW++
			pf.seriesL++
			ps.seriesBy[pf.name] = ps.seriesBy[pf.name].add(wld{w: 1})
			pf.seriesBy[ps.name] = pf.seriesBy[ps.name].add(wld{l: 1})
		}
		want := "drawn"
		switch {
		case firstWins > secondWins:
			want = f.head.redName
		case secondWins > firstWins:
			want = f.head.blueName
		}
		if f.verdict.firstWins != firstWins || f.verdict.secondWins != secondWins || f.verdict.winner != want {
			a.foldMismatch = append(a.foldMismatch, fmt.Sprintf(
				"%s: folded %d-%d (%s) against verdict %s %d-%d",
				path, firstWins, secondWins, want,
				f.verdict.winner, f.verdict.firstWins, f.verdict.secondWins))
		}
	}
	return a, nil
}

func phaseOf(move int) int {
	switch {
	case move <= phaseEarlyMax:
		return 0
	case move <= phaseMidMax:
		return 1
	}
	return 2
}

func (a *armData) seatList() []*seatStats {
	out := make([]*seatStats, 0, len(a.seats))
	for _, s := range a.seats {
		out = append(out, s)
	}
	slices.SortFunc(out, func(x, y *seatStats) int {
		rx, ry := tierRank(x.tier), tierRank(y.tier)
		if rx != ry {
			return ry - rx
		}
		return strings.Compare(x.name, y.name)
	})
	return out
}

func (a *armData) totals() (decisive, draws int) {
	for _, g := range a.games {
		if g.outcome == outcomeDraw {
			draws++
		} else {
			decisive++
		}
	}
	return decisive, draws
}

func (a *armData) zeroSumHolds() bool {
	w, l, d := 0, 0, 0
	for _, s := range a.seats {
		w += s.games.w
		l += s.games.l
		d += s.games.d
	}
	return w == l && d%2 == 0
}

func (a *armData) drawsByTierPair() map[string]int {
	out := map[string]int{}
	for _, g := range a.games {
		if g.outcome != outcomeDraw {
			continue
		}
		x, y := g.redTier, g.blueTier
		if strings.Compare(x, y) > 0 {
			x, y = y, x
		}
		out[x+"-"+y]++
	}
	return out
}
