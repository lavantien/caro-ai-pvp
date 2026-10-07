// clocktune sweeps PID gain sets for one time control over the real
// GameClock through the NewGameClockWithGains seam, ranking every grid cell
// by the wave objective: minimize the worst drain-trajectory deviation from
// GameClock.Target across cost models and game lengths, subject to the bank
// never dipping to the reserve at any commit. The committed per-TC row is
// scored in the same run so the sweep confirms or displaces it. The ranked
// table lands in out/ as the tuning artifact the config row cites.
package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	sweepSeedHi = 0xC10C7EED
	sweepSeedLo = 0x7E11
	// Slow-start shape, matching the clock bench's stress model: the first
	// moves cost a multiple of their grant while the controller is still
	// catching up.
	slowStartMoves  = 10
	slowStartFactor = 3.0
)

type cell struct {
	kp, ki, kd float64
	maxDevMs   float64
	breaches   int
	leftoverMs float64
	collapseMs float64
	stdevMs    float64
}

func main() {
	tcIdx := flag.Int("tc", 3, "time control index into config.TimeControls")
	kpMin := flag.Float64("kp-min", 0.01, "grid: minimum Kp")
	kpMax := flag.Float64("kp-max", 0.06, "grid: maximum Kp")
	kpStep := flag.Float64("kp-step", 0.01, "grid: Kp step")
	kiMin := flag.Float64("ki-min", 0.005, "grid: minimum Ki")
	kiMax := flag.Float64("ki-max", 0.03, "grid: maximum Ki")
	kiStep := flag.Float64("ki-step", 0.005, "grid: Ki step")
	kdList := flag.String("kd", "0,0.005,0.01", "grid: comma-separated Kd values")
	lengths := flag.String("lengths", "20,30,40,50,60", "comma-separated game lengths in moves per side")
	overshoot := flag.String("overshoot", "1.0,1.3", "overshoot model spend-factor bounds")
	out := flag.String("out", "", "artifact path (default playground/clocktune/out/sweep-<tc label>.md, repo-root relative like make clocktune runs it)")
	flag.Parse()

	if *tcIdx < 0 || *tcIdx >= len(config.TimeControls) {
		fmt.Fprintf(os.Stderr, "clocktune: -tc %d out of range\n", *tcIdx)
		os.Exit(1)
	}
	kds, lens, ovLo, ovHi, err := parseArgs(*kdList, *lengths, *overshoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clocktune: %v\n", err)
		os.Exit(1)
	}
	ctl := config.TimeControls[*tcIdx]
	if *out == "" {
		*out = filepath.Join("playground", "clocktune", "out", fmt.Sprintf("sweep-%d+%d.md", ctl.InitialMin, ctl.IncrementSec))
	}

	models := []struct {
		name   string
		factor func(length int) []float64
	}{
		{"exact", func(length int) []float64 { return constFactors(length, 1) }},
		{"overshoot", func(length int) []float64 { return uniformFactors(*tcIdx, length, ovLo, ovHi) }},
		{"slowstart", func(length int) []float64 { return slowStartFactors(length) }},
	}

	var cells []cell
	for _, kp := range axis(*kpMin, *kpMax, *kpStep) {
		for _, ki := range axis(*kiMin, *kiMax, *kiStep) {
			for _, kd := range kds {
				g := config.PIDGains{Kp: kp, Ki: ki, Kd: kd}
				c := cell{kp: kp, ki: ki, kd: kd}
				for _, length := range lens {
					for _, m := range models {
						r := simulate(*tcIdx, g, m.factor(length))
						c.maxDevMs = math.Max(c.maxDevMs, r.maxDevMs)
						c.breaches += r.breaches
						c.leftoverMs = math.Max(c.leftoverMs, r.leftoverMs)
						c.collapseMs = math.Max(c.collapseMs, r.collapseMs)
						c.stdevMs = math.Max(c.stdevMs, r.stdevMs)
					}
				}
				cells = append(cells, c)
			}
		}
	}

	committed := findCell(cells, config.ClockPID[*tcIdx])
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].breaches != cells[j].breaches {
			return cells[i].breaches < cells[j].breaches
		}
		return cells[i].maxDevMs < cells[j].maxDevMs
	})
	winner := cells[0]
	committedRank := 0
	for i, c := range cells {
		if c == committed {
			committedRank = i + 1
			break
		}
	}
	if committedRank == 0 {
		fmt.Fprintf(os.Stderr, "clocktune: committed row not in grid\n")
		os.Exit(1)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# clocktune sweep: %d+%d (tc %d)\n\n", ctl.InitialMin, ctl.IncrementSec, *tcIdx)
	fmt.Fprintf(&b, "grid Kp %g..%g step %g, Ki %g..%g step %g, Kd %s; lengths %s; models exact, overshoot[%g,%g], slowstart %dx%g; reserve %d ms; seeds %#x/%#x\n\n",
		*kpMin, *kpMax, *kpStep, *kiMin, *kiMax, *kiStep, *kdList, *lengths, ovLo, ovHi, slowStartMoves, slowStartFactor, config.SearchSafetyMarginMs, sweepSeedHi, sweepSeedLo)
	fmt.Fprintf(&b, "objective: rank by worst drain-trajectory deviation, reserve breaches disqualify; collapse and stdev are budget-volatility diagnostics from the same sequences (bench metric definitions)\n\n")
	fmt.Fprintf(&b, "committed row {Kp %g, Ki %g, Kd %g}: rank %d/%d, worst deviation %.0f ms, collapse %.0f ms, stdev %.0f ms, reserve breaches %d\n",
		committed.kp, committed.ki, committed.kd, committedRank, len(cells), committed.maxDevMs, committed.collapseMs, committed.stdevMs, committed.breaches)
	fmt.Fprintf(&b, "winner {Kp %g, Ki %g, Kd %g}: worst deviation %.0f ms, collapse %.0f ms, stdev %.0f ms, reserve breaches %d\n\n", winner.kp, winner.ki, winner.kd, winner.maxDevMs, winner.collapseMs, winner.stdevMs, winner.breaches)
	fmt.Fprintf(&b, "| rank | Kp | Ki | Kd | worst deviation ms | collapse ms | stdev ms | breaches | max leftover ms |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for i, c := range cells {
		fmt.Fprintf(&b, "| %d | %g | %g | %g | %.0f | %.0f | %.0f | %d | %.0f |\n", i+1, c.kp, c.ki, c.kd, c.maxDevMs, c.collapseMs, c.stdevMs, c.breaches, c.leftoverMs)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "clocktune: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "clocktune: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("clocktune: %d cells swept at tc %d (%d+%d)\n", len(cells), *tcIdx, ctl.InitialMin, ctl.IncrementSec)
	fmt.Printf("committed rank %d/%d, winner {Kp %g, Ki %g, Kd %g} worst deviation %.0f ms collapse %.0f ms breaches %d\n",
		committedRank, len(cells), winner.kp, winner.ki, winner.kd, winner.maxDevMs, winner.collapseMs, winner.breaches)
	fmt.Printf("artifact: %s\n", *out)
}

type simResult struct {
	maxDevMs   float64
	breaches   int
	leftoverMs float64
	collapseMs float64
	stdevMs    float64
}

// simulate plays one length-bounded game under the real clock law: budget,
// spend budget scaled by the model factor, commit, and measure the bank's
// deviation from the drain target after every commit.
func simulate(tcIdx int, g config.PIDGains, factors []float64) simResult {
	nsPerMs := float64(time.Millisecond)
	c := clock.NewGameClockWithGains(tcIdx, g)
	devMs := func() (float64, float64) {
		rem := float64(c.Remaining()) / nsPerMs
		tgt := float64(c.Target()) / nsPerMs
		return math.Abs(rem - tgt), rem
	}
	maxDevMs, _ := devMs()
	budgets := make([]float64, len(factors))
	breaches := 0
	for m := range factors {
		budgets[m] = float64(c.Budget()) / nsPerMs
		c.Commit(time.Duration(budgets[m] * factors[m] * nsPerMs))
		dev, rem := devMs()
		maxDevMs = math.Max(maxDevMs, dev)
		if rem <= config.SearchSafetyMarginMs {
			breaches++
		}
	}
	_, leftoverMs := devMs()
	return simResult{
		maxDevMs:   maxDevMs,
		breaches:   breaches,
		leftoverMs: leftoverMs,
		collapseMs: worstCollapse(budgets),
		stdevMs:    stdev(budgets),
	}
}

// worstCollapse is the largest single-step drop of the granted budget below
// the running mean of the grants so far, the bench's volatility metric.
func worstCollapse(b []float64) float64 {
	worst, sum := 0.0, 0.0
	for k, v := range b {
		sum += v
		if drop := sum/float64(k+1) - v; drop > worst {
			worst = drop
		}
	}
	return worst
}

func stdev(b []float64) float64 {
	var mean float64
	for _, v := range b {
		mean += v
	}
	mean /= float64(len(b))
	var ss float64
	for _, v := range b {
		d := v - mean
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(b)))
}

func axis(min, max, step float64) []float64 {
	if step <= 0 || min > max {
		return []float64{min}
	}
	n := int(math.Round((max - min) / step))
	var out []float64
	for i := 0; i <= n; i++ {
		out = append(out, math.Round((min+float64(i)*step)/step)*step)
	}
	return out
}

func constFactors(length int, v float64) []float64 {
	f := make([]float64, length)
	for i := range f {
		f[i] = v
	}
	return f
}

// uniformFactors draws the overshoot model's spend factors. The sequence
// keys on (tc, length) so every gain cell in one (model, length) column
// spends against the identical cost sequence and the comparison stays fair.
func uniformFactors(tcIdx, length int, lo, hi float64) []float64 {
	rng := rand.New(rand.NewPCG(sweepSeedHi, sweepSeedLo+uint64(uint32(tcIdx))<<32|uint64(uint32(length))))
	f := make([]float64, length)
	for i := range f {
		f[i] = lo + (hi-lo)*rng.Float64()
	}
	return f
}

func slowStartFactors(length int) []float64 {
	f := constFactors(length, 1)
	for i := range min(slowStartMoves, length) {
		f[i] = slowStartFactor
	}
	return f
}

func findCell(cells []cell, g config.PIDGains) cell {
	// Epsilon compare: axis-generated values come from min+i*step arithmetic
	// and need not equal the parsed config literals bit-for-bit.
	for _, c := range cells {
		if math.Abs(c.kp-g.Kp) < 1e-9 && math.Abs(c.ki-g.Ki) < 1e-9 && math.Abs(c.kd-g.Kd) < 1e-9 {
			return c
		}
	}
	return cell{kp: g.Kp, ki: g.Ki, kd: g.Kd, maxDevMs: math.NaN(), breaches: -1}
}

func parseArgs(kdList, lengths, overshoot string) (kds []float64, lens []int, ovLo, ovHi float64, err error) {
	for s := range strings.SplitSeq(kdList, ",") {
		v, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if e != nil {
			return nil, nil, 0, 0, fmt.Errorf("kd value %q: %w", s, e)
		}
		kds = append(kds, v)
	}
	for s := range strings.SplitSeq(lengths, ",") {
		v, e := strconv.Atoi(strings.TrimSpace(s))
		if e != nil || v <= 0 {
			return nil, nil, 0, 0, fmt.Errorf("length value %q: invalid", s)
		}
		lens = append(lens, v)
	}
	parts := strings.Split(overshoot, ",")
	if len(parts) != 2 {
		return nil, nil, 0, 0, fmt.Errorf("overshoot %q: want lo,hi", overshoot)
	}
	if ovLo, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err != nil {
		return nil, nil, 0, 0, fmt.Errorf("overshoot lo: %w", err)
	}
	if ovHi, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err != nil {
		return nil, nil, 0, 0, fmt.Errorf("overshoot hi: %w", err)
	}
	if ovLo <= 0 || ovHi < ovLo {
		return nil, nil, 0, 0, fmt.Errorf("overshoot bounds %g,%g invalid", ovLo, ovHi)
	}
	return kds, lens, ovLo, ovHi, nil
}
