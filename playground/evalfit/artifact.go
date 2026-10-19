package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type bucketRow struct {
	Lo, Hi    float64
	Empirical float64
	Predicted float64
	N         int
}

type artifactData struct {
	Date            string
	Roots           []string
	Tiers           []string
	PerRootKept     map[string]int
	PerRootRejected map[string]int
	GamesKept       int
	GamesRejected   int
	PositionsKept   int
	DrawRate        float64
	Seed            int64
	TrainSeries     int
	ValSeries       int
	W               [5]float64
	WInt            [5]int64
	IntDrift        float64
	S               float64
	BestRestart     int
	TrainLossGD     float64
	ValLossGD       float64
	BaselineS       float64
	BaselineTrain   float64
	BaselineVal     float64
	DELoss          float64
	ValLossRecorded float64
	Calibration     []bucketRow
}

func fmtF(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func writeArtifact(path string, a artifactData) error {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	w("date %s", a.Date)
	for _, root := range a.Roots {
		w("root %s", root)
	}
	for _, tier := range a.Tiers {
		w("tier %s", tier)
	}
	for _, root := range a.Roots {
		w("kept %s %d", root, a.PerRootKept[root])
		w("rejected %s %d", root, a.PerRootRejected[root])
	}
	w("games %d %d", a.GamesKept, a.GamesRejected)
	w("positions %d", a.PositionsKept)
	w("draw-rate %s", fmtF(a.DrawRate))
	w("seed %d", a.Seed)
	w("series %d %d", a.TrainSeries, a.ValSeries)
	w("weights %d %d %d %d %d", a.WInt[0], a.WInt[1], a.WInt[2], a.WInt[3], a.WInt[4])
	w("weights-exact %s %s %s %s %s", fmtF(a.W[0]), fmtF(a.W[1]), fmtF(a.W[2]), fmtF(a.W[3]), fmtF(a.W[4]))
	w("weights-int-drift %s", fmtF(a.IntDrift))
	w("scale %s", fmtF(a.S))
	w("best-restart %d", a.BestRestart)
	w("loss-gd %s %s", fmtF(a.TrainLossGD), fmtF(a.ValLossGD))
	w("baseline %s %s %s", fmtF(a.BaselineS), fmtF(a.BaselineTrain), fmtF(a.BaselineVal))
	w("loss-de %s", fmtF(a.DELoss))
	w("val-loss %s", fmtF(a.ValLossRecorded))
	for _, row := range a.Calibration {
		w("calibration %s %s %d %s %s", fmtF(row.Lo), fmtF(row.Hi), row.N, fmtF(row.Empirical), fmtF(row.Predicted))
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("evalfit: create artifact %s: %w", path, err)
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	if _, err := bw.WriteString(b.String()); err != nil {
		return fmt.Errorf("evalfit: write artifact %s: %w", path, err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("evalfit: flush artifact %s: %w", path, err)
	}
	return nil
}

func parseArtifact(path string) (artifactData, error) {
	f, err := os.Open(path)
	if err != nil {
		return artifactData{}, fmt.Errorf("evalfit: open artifact %s: %w", path, err)
	}
	defer f.Close()
	a := artifactData{PerRootKept: map[string]int{}, PerRootRejected: map[string]int{}}
	sc := bufio.NewScanner(f)
	lineno := 0
	for sc.Scan() {
		lineno++
		line := strings.TrimSuffix(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, rest, _ := strings.Cut(line, " ")
		fail := func(format string, args ...any) error {
			return fmt.Errorf("evalfit: %s:%d: %s", path, lineno, fmt.Sprintf(format, args...))
		}
		floats := func(n int) ([]float64, error) {
			parts := strings.Fields(rest)
			if len(parts) != n {
				return nil, fail("want %d fields, got %d", n, len(parts))
			}
			out := make([]float64, n)
			for i, p := range parts {
				v, err := strconv.ParseFloat(p, 64)
				if err != nil {
					return nil, fail("field %d unparsable: %q", i+1, p)
				}
				out[i] = v
			}
			return out, nil
		}
		ints := func(n int) ([]int64, error) {
			fs, err := floats(n)
			if err != nil {
				return nil, err
			}
			out := make([]int64, n)
			for i, v := range fs {
				out[i] = int64(math.Round(v))
			}
			return out, nil
		}
		switch key {
		case "date":
			a.Date = rest
		case "root":
			a.Roots = append(a.Roots, rest)
		case "tier":
			a.Tiers = append(a.Tiers, rest)
		case "kept", "rejected":
			name, n, err := parseRootCount(path, lineno, rest)
			if err != nil {
				return artifactData{}, err
			}
			if key == "kept" {
				a.PerRootKept[name] = n
			} else {
				a.PerRootRejected[name] = n
			}
		case "games":
			ns, err := ints(2)
			if err != nil {
				return artifactData{}, err
			}
			a.GamesKept, a.GamesRejected = int(ns[0]), int(ns[1])
		case "positions":
			ns, err := ints(1)
			if err != nil {
				return artifactData{}, err
			}
			a.PositionsKept = int(ns[0])
		case "draw-rate":
			fs, err := floats(1)
			if err != nil {
				return artifactData{}, err
			}
			a.DrawRate = fs[0]
		case "seed":
			ns, err := ints(1)
			if err != nil {
				return artifactData{}, err
			}
			a.Seed = ns[0]
		case "series":
			ns, err := ints(2)
			if err != nil {
				return artifactData{}, err
			}
			a.TrainSeries, a.ValSeries = int(ns[0]), int(ns[1])
		case "weights":
			ns, err := ints(5)
			if err != nil {
				return artifactData{}, err
			}
			for i := range a.WInt {
				a.WInt[i] = ns[i]
			}
		case "weights-exact":
			fs, err := floats(5)
			if err != nil {
				return artifactData{}, err
			}
			for i := range a.W {
				a.W[i] = fs[i]
			}
		case "weights-int-drift":
			fs, err := floats(1)
			if err != nil {
				return artifactData{}, err
			}
			a.IntDrift = fs[0]
		case "scale":
			fs, err := floats(1)
			if err != nil {
				return artifactData{}, err
			}
			a.S = fs[0]
		case "best-restart":
			ns, err := ints(1)
			if err != nil {
				return artifactData{}, err
			}
			a.BestRestart = int(ns[0])
		case "loss-gd":
			fs, err := floats(2)
			if err != nil {
				return artifactData{}, err
			}
			a.TrainLossGD, a.ValLossGD = fs[0], fs[1]
		case "baseline":
			fs, err := floats(3)
			if err != nil {
				return artifactData{}, err
			}
			a.BaselineS, a.BaselineTrain, a.BaselineVal = fs[0], fs[1], fs[2]
		case "loss-de":
			fs, err := floats(1)
			if err != nil {
				return artifactData{}, err
			}
			a.DELoss = fs[0]
		case "val-loss":
			fs, err := floats(1)
			if err != nil {
				return artifactData{}, err
			}
			a.ValLossRecorded = fs[0]
		case "calibration":
			fs, err := floats(5)
			if err != nil {
				return artifactData{}, err
			}
			a.Calibration = append(a.Calibration, bucketRow{Lo: fs[0], Hi: fs[1], N: int(math.Round(fs[2])), Empirical: fs[3], Predicted: fs[4]})
		default:
			return artifactData{}, fail("unrecognized line %q", line)
		}
	}
	if err := sc.Err(); err != nil {
		return artifactData{}, fmt.Errorf("evalfit: scan artifact %s: %w", path, err)
	}
	return a, nil
}

func parseRootCount(path string, lineno int, rest string) (string, int, error) {
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return "", 0, fmt.Errorf("evalfit: %s:%d: want <root> <count>, got %q", path, lineno, rest)
	}
	n, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return "", 0, fmt.Errorf("evalfit: %s:%d: count unparsable in %q", path, lineno, rest)
	}
	return strings.Join(fields[:len(fields)-1], " "), n, nil
}

func verifyArtifact(a artifactData, samples []sample) error {
	for i := 1; i < len(a.WInt); i++ {
		if a.WInt[i-1] >= a.WInt[i] {
			return fmt.Errorf("evalfit: integer weights %v are not strictly ascending", a.WInt)
		}
	}
	drift := maxIntDrift(a.W, a.WInt)
	if math.Abs(drift-a.IntDrift) > 1e-9*(1+math.Abs(a.IntDrift)) {
		return fmt.Errorf("evalfit: recorded int drift %s does not reproduce from the weights (%s)", fmtF(a.IntDrift), fmtF(drift))
	}
	var w [5]float64
	for i, v := range a.WInt {
		w[i] = float64(v)
	}
	loss := evalLoss(samples, w, a.S)
	if math.Abs(loss-a.ValLossRecorded) > 1e-9*(1+math.Abs(a.ValLossRecorded)) {
		return fmt.Errorf("evalfit: recorded val loss %s does not reproduce from the shipped weights (%s)", fmtF(a.ValLossRecorded), fmtF(loss))
	}
	return nil
}
