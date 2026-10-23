package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

func main() {
	mode := flag.String("mode", "claim", "claim|verify|table|geom")
	out := flag.String("out", "out", "artifact directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	switch *mode {
	case "claim":
		runClaim(*out)
	case "verify":
		runVerify(*out)
	case "table":
		runTable(*out)
	case "geom":
		runGeom(*out)
	default:
		fail(fmt.Errorf("unknown mode %s", *mode))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "advhex:", err)
	os.Exit(1)
}

func runClaim(outDir string) {
	path := filepath.Join(outDir, "solver-claim.txt")
	fh, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	defer func() { _ = fh.Close() }()
	s := func(format string, args ...any) string { return fmt.Sprintf(format, args...) }
	lines := []string{}
	g, err := geom.ByName("hex")
	if err != nil {
		fail(err)
	}
	b := rules.NewBoardFor(g)
	for _, st := range hexFixture() {
		if st.red {
			b.Side = rules.Red
		} else {
			b.Side = rules.Blue
		}
		b.Make(rules.Cell(st.q*g.Stride + st.r))
	}
	b.Side = rules.Red
	for _, kind := range []struct {
		name string
		k    vcf.Kind
	}{{"vcf", vcf.KindVCF}, {"vct", vcf.KindVCT}} {
		sv := vcf.New(kind.k)
		var stats vcf.SolverStats
		f := sv.Solve(b, config.SolverNodeBudget, nil, &stats)
		lines = append(lines, s("%s: found=%v plies=%d nodes=%d pv=%s", kind.name, f, stats.Plies, stats.Nodes, renderPV(g, stats)))
	}
	var sb strings.Builder
	for i, l := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(l)
	}
	out := sb.String()
	fmt.Println(out)
	_, _ = fmt.Fprint(fh, out+"\n")
}

type fixtureStone struct {
	q, r int
	red  bool
}

func hexFixture() []fixtureStone {
	return []fixtureStone{
		{11, 12, true}, {11, 13, true}, {11, 14, true}, {11, 11, false},
		{14, 12, true}, {14, 13, true}, {14, 14, true}, {14, 11, false},
	}
}

func solverClaim() (found bool, plies int, pv []rules.Move, nodes uint64) {
	g, err := geom.ByName("hex")
	if err != nil {
		fail(err)
	}
	b := rules.NewBoardFor(g)
	for _, s := range hexFixture() {
		if s.red {
			b.Side = rules.Red
		} else {
			b.Side = rules.Blue
		}
		b.Make(rules.Cell(s.q*g.Stride + s.r))
	}
	b.Side = rules.Red
	for _, kind := range []vcf.Kind{vcf.KindVCF, vcf.KindVCT} {
		s := vcf.New(kind)
		var stats vcf.SolverStats
		f := s.Solve(b, config.SolverNodeBudget, nil, &stats)
		fmt.Printf("claim kind=%d found=%v plies=%d nodes=%d pv=%s\n", kind, f, stats.Plies, stats.Nodes, renderPV(g, stats))
		if kind == vcf.KindVCT {
			return f, stats.Plies, stats.PV[:stats.Plies], stats.Nodes
		}
	}
	return false, 0, nil, 0
}

func renderPV(g *geom.Geometry, stats vcf.SolverStats) string {
	var sb strings.Builder
	for i := 0; i < stats.Plies && i < len(stats.PV); i++ {
		if i > 0 {
			sb.WriteString(" ")
		}
		name, _ := g.CellName(int(stats.PV[i]))
		sb.WriteString(name)
	}
	return sb.String()
}
