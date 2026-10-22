package main

import (
	"fmt"
	"sort"
)

func medianInt(v []int) int {
	s := append([]int{}, v...)
	sort.Ints(s)
	return s[len(s)/2]
}

func medianFloat(v []float64) float64 {
	s := append([]float64{}, v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

func smoothCfgs() []cfg {
	base := smoothBaseCfg("base")
	sh := smoothBaseCfg("shade")
	sh.Shade = true
	su := smoothBaseCfg("shade+sur")
	su.Shade = true
	su.Surrogate = true
	return []cfg{base, sh, su}
}

func decCfgs() []cfg {
	base := smoothBaseCfg("base")
	sh := smoothBaseCfg("shade")
	sh.Shade = true
	rep := smoothBaseCfg("shade+rep")
	rep.Shade = true
	rep.Repair = true
	rs := smoothBaseCfg("shade+rep+sur")
	rs.Shade = true
	rs.Repair = true
	rs.Surrogate = true
	return []cfg{base, sh, rep, rs}
}

func gameCfgs() []cfg {
	deRef := gameCfg("de+ref")
	deRef.Shade = false
	ref := gameCfg("shade+ref")
	crn := gameCfg("shade+ref+crn")
	crn.CRN = true
	bt := gameCfg("shade+bt")
	bt.Estimator = "bt"
	sprt := gameCfg("shade+btsprt")
	sprt.Estimator = "btsprt"
	sprtc := gameCfg("shade+btsprt+crn")
	sprtc.Estimator = "btsprt"
	sprtc.CRN = true
	allon := gameCfg("allon")
	allon.Estimator = "btsprt"
	allon.CRN = true
	allon.Surrogate = true
	return []cfg{deRef, ref, crn, bt, sprt, sprtc, allon}
}

type benchObj struct {
	name   string
	make   func(seed int64) objective
	target float64
	cfgs   []cfg
}

func decReference() float64 {
	best := 1e18
	c := smoothBaseCfg("decref")
	c.Shade = true
	c.Repair = true
	c.Gens = 1200
	c.Budget = 60000
	for _, sd := range []int64{9001, 9002, 9003} {
		o := newRosenbrockDec(6)
		r := run(c, o, sd, 0)
		if r.BestTrue < best {
			best = r.BestTrue
		}
	}
	return best
}

func buildBench(decTarget float64) []benchObj {
	outs := []benchObj{
		{name: "rastrigin", make: func(int64) objective { return &rastrigin{d: 6} }, target: rastriginTarget, cfgs: smoothCfgs()},
		{name: "rosenbrock", make: func(int64) objective { return &rosenbrock{d: 6} }, target: rosenbrockTarget, cfgs: smoothCfgs()},
		{name: "rosenbrockDec", make: func(int64) objective { return newRosenbrockDec(6) }, target: decTarget, cfgs: decCfgs()},
	}
	for _, m := range []struct {
		name   string
		margin float64
	}{
		{"game55", marginWeak},
		{"game70", marginMid},
		{"game90", marginStrong},
	} {
		outs = append(outs, benchObj{
			name:   m.name,
			make:   func(seed int64) objective { return newGameObj(6, m.margin, seed) },
			target: gameTargetEps,
			cfgs:   gameCfgs(),
		})
	}
	return outs
}

type benchRow struct {
	Obj      string
	Cfg      string
	Hits     int
	MedEvals int
	MedBest  float64
	MedTotal int
}

func runBench(objs []benchObj, seedList []int64) []benchRow {
	var rows []benchRow
	for _, bo := range objs {
		for _, c := range bo.cfgs {
			hits := 0
			var hitEvals, bests []float64
			totals := 0
			for _, sd := range seedList {
				o := bo.make(sd)
				r := run(c, o, sd, bo.target)
				if r.Hit {
					hits++
					hitEvals = append(hitEvals, float64(r.EvalsToTarget))
				}
				bests = append(bests, r.BestTrue)
				totals += r.TotalEvals
			}
			med := 0.0
			if len(hitEvals) > 0 {
				med = medianFloat(hitEvals)
			}
			rows = append(rows, benchRow{
				Obj: bo.name, Cfg: c.Name, Hits: hits,
				MedEvals: int(med),
				MedBest:  medianFloat(bests),
				MedTotal: totals / len(seedList),
			})
		}
	}
	return rows
}

func printRows(rows []benchRow) {
	fmt.Printf("%-14s %-16s %4s %10s %14s %10s\n", "objective", "config", "hit", "med-hit-ev", "med-best-true", "avg-total")
	for _, r := range rows {
		fmt.Printf("%-14s %-16s %4d %10d %14.6g %10d\n", r.Obj, r.Cfg, r.Hits, r.MedEvals, r.MedBest, r.MedTotal)
	}
}
