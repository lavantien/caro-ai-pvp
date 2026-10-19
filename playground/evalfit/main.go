package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	rejectBudgetFrac = 0.02
	deToleranceFrac  = 0.02
	calibrationBins  = 10
	minSamples       = 1000
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "evalfit: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	dbs := flag.String("dbs", "", "comma-separated sqlite db paths holding tournament_games")
	tiers := flag.String("tiers", "master,grandmaster", "comma-separated seat tiers both seats must hold, empty keeps every tier")
	seed := flag.Int64("seed", 1, "series split and optimizer seed")
	trainFrac := flag.Float64("train-frac", 0.9, "fraction of series held for training")
	out := flag.String("out", "out/evalfit.txt", "artifact path")
	flag.Parse()
	fmt.Fprintf(os.Stderr, "evalfit: start dbs=%s tiers=%s seed=%d train-frac=%.2f\n", *dbs, *tiers, *seed, *trainFrac)
	if *dbs == "" {
		fatal("-dbs is required")
	}
	roots := strings.Split(*dbs, ",")
	tierSet := map[string]bool{}
	if strings.TrimSpace(*tiers) != "" {
		for _, t := range strings.Split(*tiers, ",") {
			tierSet[strings.TrimSpace(t)] = true
		}
	}
	perKept := map[string]int{}
	perRejected := map[string]int{}
	gamesKept := 0
	gamesRejected := 0
	var rejectReasons []string
	var samples []sample
	keySet := map[string]bool{}
	var seriesKeys []string
	for _, root := range roots {
		load, err := loadDBGames(root, tierSet)
		if err != nil {
			fatal("%v", err)
		}
		for _, s := range load.Series {
			keptInSeries := false
			for _, g := range s.Games {
				ss, err := reconstructGame(g, s.Key)
				if err != nil {
					gamesRejected++
					perRejected[root]++
					if len(rejectReasons) < 10 {
						rejectReasons = append(rejectReasons, fmt.Sprintf("%s: %v", s.Key, err))
					}
					continue
				}
				gamesKept++
				perKept[root]++
				keptInSeries = true
				samples = append(samples, ss...)
			}
			if keptInSeries && !keySet[s.Key] {
				keySet[s.Key] = true
				seriesKeys = append(seriesKeys, s.Key)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "evalfit: loaded %d games (%d rejected) over %d series, %d positions\n",
		gamesKept, gamesRejected, len(seriesKeys), len(samples))
	if len(rejectReasons) > 0 {
		for _, r := range rejectReasons {
			fmt.Fprintf(os.Stderr, "evalfit: rejected: %s\n", r)
		}
	}
	if gamesRejected > int(rejectBudgetFrac*float64(gamesKept+gamesRejected)) {
		fatal("%d of %d games rejected, over the %.0f%% integrity budget", gamesRejected, gamesKept+gamesRejected, 100*rejectBudgetFrac)
	}
	if len(samples) < minSamples {
		fatal("%d positions fall under the %d minimum", len(samples), minSamples)
	}
	sort.Strings(seriesKeys)
	trainSet := splitSeriesKeys(seriesKeys, *seed, *trainFrac)
	var train, val []sample
	for _, s := range samples {
		if trainSet[s.Series] {
			train = append(train, s)
		} else {
			val = append(val, s)
		}
	}
	if len(train) == 0 || len(val) == 0 {
		fatal("split left train=%d val=%d positions", len(train), len(val))
	}
	trainSeries := 0
	for _, k := range seriesKeys {
		if trainSet[k] {
			trainSeries++
		}
	}
	fmt.Fprintf(os.Stderr, "evalfit: split %d train series (%d positions), %d val series (%d positions)\n",
		trainSeries, len(train), len(seriesKeys)-trainSeries, len(val))

	d := aggregateFit(train)
	rng := rand.New(rand.NewSource(*seed))
	res := fitGD(d, defaultGDOptions(), rng, func(restart, step int, loss float64) {
		fmt.Fprintf(os.Stderr, "evalfit: gd restart %d step %d loss %.9f\n", restart, step, loss)
	})
	fmt.Fprintf(os.Stderr, "evalfit: gd best loss %.9f at restart %d after %d steps\n", res.Loss, res.Restart, res.Steps)

	var cur [5]float64
	for i := range cur {
		cur[i] = float64(config.PatternClassWeights[i+1])
	}
	base := fitScaleGD(d, cur, defaultGDOptions(), rng)

	de := fitDE(d, defaultDEOptions(), rand.New(rand.NewSource(*seed+1)))
	fmt.Fprintf(os.Stderr, "evalfit: de loss %.9f (gen %d)\n", de.Loss, de.Gen)
	if de.Loss < res.Loss*(1-deToleranceFrac) {
		fatal("gd loss %.9f loses to de %.9f beyond the %.0f%% tolerance", res.Loss, de.Loss, 100*deToleranceFrac)
	}

	wInt := projectMonotoneInt(res.W)
	intDrift := maxIntDrift(res.W, wInt)
	var wShip [5]float64
	for i := range wInt {
		wShip[i] = float64(wInt[i])
	}
	valRecorded := evalLoss(val, wShip, res.S)
	draws := 0
	for _, s := range samples {
		if s.Y == 0.5 {
			draws++
		}
	}
	a := artifactData{
		Date:            time.Now().Format("2006-01-02"),
		Roots:           roots,
		Tiers:           sortedKeys(tierSet),
		PerRootKept:     perKept,
		PerRootRejected: perRejected,
		GamesKept:       gamesKept,
		GamesRejected:   gamesRejected,
		PositionsKept:   len(samples),
		DrawRate:        float64(draws) / float64(len(samples)),
		Seed:            *seed,
		TrainSeries:     trainSeries,
		ValSeries:       len(seriesKeys) - trainSeries,
		W:               res.W,
		WInt:            wInt,
		IntDrift:        intDrift,
		S:               res.S,
		BestRestart:     res.Restart,
		TrainLossGD:     res.Loss,
		ValLossGD:       evalLoss(val, res.W, res.S),
		BaselineS:       base.S,
		BaselineTrain:   base.Loss,
		BaselineVal:     evalLoss(val, cur, base.S),
		DELoss:          de.Loss,
		ValLossRecorded: valRecorded,
		Calibration:     calibrate(val, wShip, res.S),
	}
	if err := writeArtifact(*out, a); err != nil {
		fatal("%v", err)
	}
	if err := verifyArtifact(a, val); err != nil {
		fatal("%v", err)
	}
	fmt.Fprintf(os.Stderr, "evalfit: artifact %s written and verified\n", *out)
	fmt.Printf("games %d kept %d rejected, %d positions, draw rate %.4f\n", gamesKept, gamesRejected, len(samples), a.DrawRate)
	fmt.Printf("series %d train %d val\n", a.TrainSeries, a.ValSeries)
	fmt.Printf("gd loss train %.9f val %.9f (restart %d)\n", a.TrainLossGD, a.ValLossGD, a.BestRestart)
	fmt.Printf("baseline scale %.3f train %.9f val %.9f\n", a.BaselineS, a.BaselineTrain, a.BaselineVal)
	fmt.Printf("de loss %.9f\n", a.DELoss)
	fmt.Printf("weights %d %d %d %d %d\n", wInt[0], wInt[1], wInt[2], wInt[3], wInt[4])
	fmt.Printf("weights-int-drift %s\n", fmtF(intDrift))
	fmt.Printf("scale %.3f\n", a.S)
	fmt.Printf("artifact %s\n", *out)
}

func calibrate(val []sample, w [5]float64, s float64) []bucketRow {
	type row struct {
		p, y, t float64
	}
	rows := make([]row, 0, len(val))
	for _, sm := range val {
		dot := float64(config.EvalTempo)
		for j := range sm.F {
			dot += float64(sm.F[j]) * w[j]
		}
		rows = append(rows, row{p: sigmoid(dot / s), y: sm.Y, t: dot / s})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].t < rows[j].t })
	out := make([]bucketRow, 0, calibrationBins)
	for b := 0; b < calibrationBins; b++ {
		lo := b * len(rows) / calibrationBins
		hi := (b + 1) * len(rows) / calibrationBins
		if b == calibrationBins-1 {
			hi = len(rows)
		}
		if hi <= lo {
			continue
		}
		var ySum, pSum float64
		for _, r := range rows[lo:hi] {
			ySum += r.y
			pSum += r.p
		}
		n := hi - lo
		out = append(out, bucketRow{
			Lo:        rows[lo].t,
			Hi:        rows[hi-1].t,
			Empirical: ySum / float64(n),
			Predicted: pSum / float64(n),
			N:         n,
		})
	}
	return out
}
