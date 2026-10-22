package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type benchCase struct {
	name   string
	dims   []dim
	target float64
	obj    func(rng *rand.Rand) func([]float64) float64
}

type method struct {
	name string
	run  func(dims []dim, obj func([]float64) float64, seed uint64, budget int, target float64) runResult
}

func methods() []method {
	return []method{
		{"bayes", runBayes},
		{"grid", func(dims []dim, obj func([]float64) float64, _ uint64, budget int, target float64) runResult {
			return runGrid(dims, obj, budget, target)
		}},
		{"random", runRandom},
		{"aco", runACO},
	}
}

func benchCases() []benchCase {
	quad := []dim{
		{"x0", -quadDomain, quadDomain, false},
		{"x1", -quadDomain, quadDomain, false},
		{"x2", -quadDomain, quadDomain, false},
	}
	return []benchCase{
		{"branin", []dim{{"x0", braninLo0, braninHi0, false}, {"x1", braninLo1, braninHi1, false}}, braninTarget,
			func(*rand.Rand) func([]float64) float64 { return branin }},
		{"hartmann3", []dim{{"x0", hartmannLo, hartmannHi, false}, {"x1", hartmannLo, hartmannHi, false}, {"x2", hartmannLo, hartmannHi, false}}, hartmannTarget,
			func(*rand.Rand) func([]float64) float64 { return hartmann3 }},
		{"noisyquadratic", quad, quadTarget,
			func(rng *rand.Rand) func([]float64) float64 { return noised(quadratic3, rng) }},
	}
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[(len(s)-1)/2]
}

func writeValidationTable(path string) error {
	cases := benchCases()
	ms := methods()
	type cellT struct {
		trials []float64
		best   []float64
	}
	table := make([]map[string]*cellT, len(cases))
	for i := range table {
		table[i] = map[string]*cellT{}
		for _, m := range ms {
			table[i][m.name] = &cellT{}
		}
	}
	type job struct {
		caseIdx int
		seed    uint64
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < tableWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				c := cases[j.caseIdx]
				noiseRng := rand.New(rand.NewPCG(noiseSeedBase+j.seed, validationSeedBase+j.seed))
				obj := c.obj(noiseRng)
				for _, m := range ms {
					r := m.run(c.dims, obj, validationSeedBase+j.seed, tableBudget, c.target)
					table[j.caseIdx][m.name].trials = append(table[j.caseIdx][m.name].trials, float64(min(r.trialsToTarget, tableBudget+1)))
					table[j.caseIdx][m.name].best = append(table[j.caseIdx][m.name].best, r.bestF)
				}
			}
		}()
	}
	for s := uint64(1); s <= seedCount; s++ {
		for i := range cases {
			jobs <- job{caseIdx: i, seed: s}
		}
	}
	close(jobs)
	wg.Wait()

	var b strings.Builder
	fmt.Fprintf(&b, "# tune lane validation\n\n")
	fmt.Fprintf(&b, "budget %d trials per run, %d seeds, median trials-to-target (%d means never reached), median best value at budget\n\n",
		tableBudget, seedCount, tableBudget+1)
	fmt.Fprintf(&b, "| objective | target | method | median trials-to-target | median best f |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | --- |\n")
	for i, c := range cases {
		for _, m := range ms {
			tt := median(table[i][m.name].trials)
			label := fmt.Sprintf("%.0f", tt)
			if tt > tableBudget {
				label = fmt.Sprintf(">%d", tableBudget)
			}
			fmt.Fprintf(&b, "| %s | %g | %s | %s | %.4f |\n", c.name, c.target, m.name, label, median(table[i][m.name].best))
		}
	}
	return writeFile(path, b.String())
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func main() {
	tablePath := filepath.Join("out", "table.md")
	demoPath := filepath.Join("out", "demo.md")
	if err := writeValidationTable(tablePath); err != nil {
		fmt.Fprintf(os.Stderr, "tune: table: %v\n", err)
		os.Exit(1)
	}
	if err := writeDemoArtifact(demoPath); err != nil {
		fmt.Fprintf(os.Stderr, "tune: demo: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("tune: artifacts at %s and %s\n", tablePath, demoPath)
}
