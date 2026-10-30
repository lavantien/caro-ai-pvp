package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	pathGarbage       = 0
	pathDefenderWin   = 1
	pathFour          = 2
	pathNotThree      = 3
	pathThree         = 4
	resFail           = 0
	resWin            = 1
	resAbort          = 2
	boardSize         = 16
	boardStride       = 16
	mutLine           = 138
	mutCol            = 11
	huntPly           = 1
	huntPlyCap        = 7
	huntThreePartners = 9
)

var (
	repoRoot   string
	workRoot   string
	huntBudget = 8000
)

type stoneT struct {
	r, c int
	red  bool
}

type probeT struct {
	path   int
	result int
	n      int
	nodes  int
	cells  []int
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vcfhunt: "+format+"\n", args...)
	os.Exit(1)
}

func trace(format string, args ...any) {
	fmt.Printf("vcfhunt: "+format+"\n", args...)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

func prepTree(name string, withDriver bool) (string, error) {
	tree := filepath.Join(workRoot, name)
	if err := os.MkdirAll(tree, 0o755); err != nil {
		return "", err
	}
	for _, f := range [...]string{"go.mod", "go.sum"} {
		if _, err := os.Stat(filepath.Join(repoRoot, f)); err != nil {
			continue
		}
		if err := copyFile(filepath.Join(repoRoot, f), filepath.Join(tree, f)); err != nil {
			return "", err
		}
	}
	rootGo, err := filepath.Glob(filepath.Join(repoRoot, "*.go"))
	if err != nil {
		return "", err
	}
	for _, p := range rootGo {
		if err := copyFile(p, filepath.Join(tree, filepath.Base(p))); err != nil {
			return "", err
		}
	}
	if err := copyTree(filepath.Join(repoRoot, "internal"), filepath.Join(tree, "internal")); err != nil {
		return "", err
	}
	if withDriver {
		if err := os.MkdirAll(filepath.Join(tree, "hunt"), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(tree, "hunt", "main.go"), []byte(driverSrc), 0o644); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(tree, "internal", "vcf", "huntdrv.go"), []byte(probeSrc), 0o644); err != nil {
			return "", err
		}
	}
	return tree, nil
}

func spliceMutant(tree string) error {
	path := filepath.Join(tree, "internal", "vcf", "search.go")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src := string(data)
	lines := strings.Split(src, "\n")
	if mutLine > len(lines) {
		return fmt.Errorf("search.go has %d lines, want line %d", len(lines), mutLine)
	}
	want := "\tfor i := 0; i < n; i++ {"
	if strings.TrimRight(lines[mutLine-1], "\r") != want {
		return fmt.Errorf("search.go line %d is %q, want %q", mutLine, lines[mutLine-1], want)
	}
	off := 0
	for i := 0; i < mutLine-1; i++ {
		off += len(lines[i]) + 1
	}
	if off+mutCol-1 >= len(src) || src[off+mutCol-1] != '0' {
		return fmt.Errorf("search.go %d:%d is %q, want the loop start literal", mutLine, mutCol, string(src[off+mutCol-1]))
	}
	spliced := src[:off+mutCol-1] + "1" + src[off+mutCol:]
	if err := os.WriteFile(path, []byte(spliced), 0o644); err != nil {
		return err
	}
	trace("mutant spliced at search.go:%d:%d", mutLine, mutCol)
	return nil
}

func treeEnv(tree string) []string {
	base := tree + "-tmp"
	cache := tree + "-cache"
	os.MkdirAll(base, 0o755)
	os.MkdirAll(cache, 0o755)
	return append(os.Environ(),
		"CGO_ENABLED=1",
		"GOCACHE="+cache,
		"GOTMPDIR="+base,
		"GOMEMLIMIT=4GiB",
	)
}

func buildDriver(tree, bin string) error {
	cmd := exec.Command("go", "build", "-o", bin, "./hunt")
	cmd.Dir = tree
	cmd.Env = treeEnv(tree)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build %s: %v: %s", tree, err, out)
	}
	return nil
}

func runDriver(bin, candPath, outPath string) error {
	in, err := os.Open(candPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	cmd := exec.Command(bin)
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func writeBoard(sb *strings.Builder, g genBoard) {
	sb.WriteString(strconv.Itoa(huntPly))
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(huntPlyCap))
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(huntBudget))
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(g.lastR))
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(g.lastC))
	for _, s := range g.stones {
		k := 0
		if s.red {
			k = 1
		}
		sb.WriteString(fmt.Sprintf(" %d,%d,%d", s.r, s.c, k))
	}
	sb.WriteByte('\n')
}

func writeCands(path string, batch []genBoard) error {
	var sb strings.Builder
	for _, g := range batch {
		writeBoard(&sb, g)
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func writeOne(path string, g genBoard) error {
	var sb strings.Builder
	writeBoard(&sb, g)
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func parseProbes(path string) ([]probeT, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []probeT
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			return nil, fmt.Errorf("short probe line %q in %s", sc.Text(), path)
		}
		var p probeT
		vals := make([]int, 4)
		for i := range vals {
			v, err := strconv.Atoi(fields[i])
			if err != nil {
				return nil, fmt.Errorf("bad probe field %q in %s: %v", fields[i], path, err)
			}
			vals[i] = v
		}
		p.path, p.result, p.n, p.nodes = vals[0], vals[1], vals[2], vals[3]
		for _, tok := range fields[4:] {
			v, err := strconv.Atoi(tok)
			if err != nil {
				return nil, fmt.Errorf("bad probe cell %q in %s: %v", tok, path, err)
			}
			p.cells = append(p.cells, v)
		}
		out = append(out, p)
	}
	return out, sc.Err()
}

func evalPair(cleanBin, mutBin string, g genBoard) (probeT, probeT, error) {
	cand := filepath.Join(workRoot, "one.txt")
	if err := writeOne(cand, g); err != nil {
		return probeT{}, probeT{}, err
	}
	cleanOut := filepath.Join(workRoot, "one-clean.txt")
	mutOut := filepath.Join(workRoot, "one-mut.txt")
	if err := runDriver(cleanBin, cand, cleanOut); err != nil {
		return probeT{}, probeT{}, err
	}
	if err := runDriver(mutBin, cand, mutOut); err != nil {
		return probeT{}, probeT{}, err
	}
	cp, err := parseProbes(cleanOut)
	if err != nil {
		return probeT{}, probeT{}, err
	}
	mp, err := parseProbes(mutOut)
	if err != nil {
		return probeT{}, probeT{}, err
	}
	if len(cp) != 1 || len(mp) != 1 {
		return probeT{}, probeT{}, errors.New("probe pair returned wrong line count")
	}
	return cp[0], mp[0], nil
}

func killsClean(p, m probeT) bool {
	return p.path >= pathFour && p.result == resFail && m.result == resWin
}

var dirs = [4][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}

var partners = [][]int{
	{-1, 1}, {1, 2}, {-2, -1},
	{-1, 2}, {1, 3}, {-3, -1}, {-2, 1}, {-1, 3}, {2, 3}, {-3, -2},
}

type genBoard struct {
	stones []stoneT
	lastR  int
	lastC  int
}

func (g *genBoard) has(r, c int) bool {
	for _, s := range g.stones {
		if s.r == r && s.c == c {
			return true
		}
	}
	return false
}

func (g *genBoard) place(r, c int, red bool) bool {
	if r < 0 || r >= boardSize || c < 0 || c >= boardSize || g.has(r, c) {
		return false
	}
	g.stones = append(g.stones, stoneT{r, c, red})
	return true
}

func (g *genBoard) size() int { return len(g.stones) }

func genCandidate(rnd *rand.Rand) (genBoard, bool) {
	var g genBoard
	if rnd.Intn(100) < 35 {
		g.lastR = rnd.Intn(4) + 12*rnd.Intn(2)
		g.lastC = rnd.Intn(4) + 12*rnd.Intn(2)
	} else {
		g.lastR = 3 + rnd.Intn(10)
		g.lastC = 3 + rnd.Intn(10)
	}
	d := dirs[rnd.Intn(4)]
	dr, dc := d[0], d[1]
	pair := partners[rnd.Intn(huntThreePartners)]
	if !g.place(g.lastR, g.lastC, true) {
		return g, false
	}
	for _, k := range pair {
		if !g.place(g.lastR+k*dr, g.lastC+k*dc, true) {
			return g, false
		}
	}
	nBlue := rnd.Intn(3)
	for i := 0; i < nBlue; i++ {
		off := 1 + rnd.Intn(5)
		if rnd.Intn(2) == 0 {
			off = -off
		}
		g.place(g.lastR+off*dr, g.lastC+off*dc, false)
	}
	if rnd.Intn(100) < 35 {
		d2 := dirs[rnd.Intn(4)]
		off := rnd.Intn(5) - 2
		ar, ac := g.lastR+off*dr, g.lastC+off*dc
		pair2 := partners[rnd.Intn(len(partners))]
		if g.place(ar, ac, false) {
			for _, k := range pair2 {
				if !g.place(ar+k*d2[0], ac+k*d2[1], false) {
					break
				}
			}
		}
	}
	nRed := rnd.Intn(3)
	for i := 0; i < nRed; i++ {
		switch rnd.Intn(3) {
		case 0:
			d3 := dirs[rnd.Intn(4)]
			off := rnd.Intn(7) - 3
			if off == 0 {
				off = 3
			}
			g.place(g.lastR+off*d3[0], g.lastC+off*d3[1], true)
		case 1:
			off := 4 + rnd.Intn(3)
			if rnd.Intn(2) == 0 {
				off = -off
			}
			g.place(g.lastR+off*dr, g.lastC+off*dc, true)
		case 2:
			if len(g.stones) == 0 {
				continue
			}
			base := g.stones[rnd.Intn(len(g.stones))]
			d3 := dirs[rnd.Intn(4)]
			off := rnd.Intn(5) - 2
			if off == 0 {
				off = 1
			}
			g.place(base.r+off*d3[0], base.c+off*d3[1], true)
		}
	}
	if g.size() < 4 {
		return g, false
	}
	return g, true
}

func genBatch(rnd *rand.Rand, n int) []genBoard {
	var out []genBoard
	for len(out) < n {
		for attempt := 0; attempt < 8; attempt++ {
			if g, ok := genCandidate(rnd); ok {
				out = append(out, g)
				break
			}
		}
	}
	return out
}

func cellRC(cell int) (int, int) { return cell / boardStride, cell % boardStride }

func reportBoard(g genBoard, cp, mp probeT) {
	var sb strings.Builder
	sb.WriteString("WINNER")
	for _, s := range g.stones {
		k := 0
		if s.red {
			k = 1
		}
		sb.WriteString(fmt.Sprintf(" %d,%d,%d", s.r, s.c, k))
	}
	sb.WriteString(fmt.Sprintf(" last=%d,%d", g.lastR, g.lastC))
	fmt.Println(sb.String())
	fmt.Printf("probe clean path=%d res=%d nodes=%d n=%d\n", cp.path, cp.result, cp.nodes, cp.n)
	fmt.Printf("probe mut   path=%d res=%d nodes=%d n=%d\n", mp.path, mp.result, mp.nodes, mp.n)
	var defs strings.Builder
	for _, cell := range cp.cells {
		r, c := cellRC(cell)
		defs.WriteString(fmt.Sprintf(" (%d,%d)", r, c))
	}
	fmt.Printf("defends:%s\n", defs.String())
	var abs strings.Builder
	for _, s := range g.stones {
		fmt.Fprintf(&abs, "{%d, %d, %v}, ", s.r, s.c, s.red)
	}
	fmt.Printf("absStones: %s\n", abs.String())
}

func minimize(cleanBin, mutBin string, g genBoard) (genBoard, probeT, probeT, bool) {
	cp, mp, err := evalPair(cleanBin, mutBin, g)
	if err != nil || !killsClean(cp, mp) {
		return g, cp, mp, false
	}
	for i := len(g.stones) - 1; i >= 0; i-- {
		s := g.stones[i]
		if s.r == g.lastR && s.c == g.lastC && s.red {
			continue
		}
		trial := make([]stoneT, 0, len(g.stones)-1)
		trial = append(trial, g.stones[:i]...)
		trial = append(trial, g.stones[i+1:]...)
		t := genBoard{stones: trial, lastR: g.lastR, lastC: g.lastC}
		tcp, tmp, err := evalPair(cleanBin, mutBin, t)
		if err == nil && killsClean(tcp, tmp) {
			g = t
			cp, mp = tcp, tmp
		}
	}
	return g, cp, mp, true
}

func hunt(seed, rounds, per int) {
	cleanTree, err := prepTree("clean", true)
	if err != nil {
		fatalf("prep clean: %v", err)
	}
	mutTree, err := prepTree("mut", true)
	if err != nil {
		fatalf("prep mut: %v", err)
	}
	if err := spliceMutant(mutTree); err != nil {
		fatalf("splice: %v", err)
	}
	cleanBin := filepath.Join(workRoot, "clean.exe")
	mutBin := filepath.Join(workRoot, "mut.exe")
	trace("entry hunt seed=%d rounds=%d per=%d", seed, rounds, per)
	if err := buildDriver(cleanTree, cleanBin); err != nil {
		fatalf("build clean: %v", err)
	}
	if err := buildDriver(mutTree, mutBin); err != nil {
		fatalf("build mut: %v", err)
	}
	trace("drivers built")
	var fallback *genBoard
	var fbC, fbM probeT
	rnd := rand.New(rand.NewSource(int64(seed)))
	candPath := filepath.Join(workRoot, "cands.txt")
	cleanOut := filepath.Join(workRoot, "clean-out.txt")
	mutOut := filepath.Join(workRoot, "mut-out.txt")
	for round := 0; round < rounds; round++ {
		batch := genBatch(rnd, per)
		if err := writeCands(candPath, batch); err != nil {
			fatalf("write cands: %v", err)
		}
		if err := runDriver(cleanBin, candPath, cleanOut); err != nil {
			fatalf("clean driver round %d: %v", round, err)
		}
		cp, err := parseProbes(cleanOut)
		if err != nil {
			fatalf("parse clean round %d: %v", round, err)
		}
		if len(cp) != len(batch) {
			fatalf("round %d clean lines %d want %d", round, len(cp), len(batch))
		}
		stats := make(map[string]int)
		keep := make([]int, 0, len(batch))
		for i := range batch {
			key := fmt.Sprintf("path%d_clean%d", cp[i].path, cp[i].result)
			stats[key]++
			if cp[i].path >= pathFour && cp[i].result == resFail {
				keep = append(keep, i)
			}
		}
		trace("round %d clean: %v filtered=%d", round, stats, len(keep))
		if len(keep) == 0 {
			continue
		}
		var sb strings.Builder
		for _, i := range keep {
			writeBoard(&sb, batch[i])
		}
		filtered := filepath.Join(workRoot, "filtered.txt")
		if err := os.WriteFile(filtered, []byte(sb.String()), 0o644); err != nil {
			fatalf("write filtered: %v", err)
		}
		if err := runDriver(mutBin, filtered, mutOut); err != nil {
			fatalf("mut driver round %d: %v", round, err)
		}
		mp, err := parseProbes(mutOut)
		if err != nil {
			fatalf("parse mut round %d: %v", round, err)
		}
		if len(mp) != len(keep) {
			fatalf("round %d mut lines %d want %d", round, len(mp), len(keep))
		}
		for k, i := range keep {
			if cp[i].result == mp[k].result {
				continue
			}
			if cp[i].result == resFail && mp[k].result == resWin {
				stats[fmt.Sprintf("flip_p%d_0to1", cp[i].path)]++
				trace("round %d hit at index %d", round, i)
				mg, mcp, mmp, ok := minimize(cleanBin, mutBin, batch[i])
				if !ok {
					fatalf("hit at round %d index %d failed reconfirm", round, i)
				}
				reportBoard(mg, mcp, mmp)
				trace("exit hunt minimized")
				return
			}
			if fallback == nil && cp[i].path >= pathFour && cp[i].result == resFail && mp[k].result == resAbort {
				b := batch[i]
				fallback = &b
				fbC, fbM = cp[i], mp[k]
				trace("round %d fallback divergence at index %d", round, i)
			}
		}
		trace("round %d mut: %v", round, stats)
	}
	if fallback != nil {
		trace("no clean win divergence, reporting fallback")
		reportBoard(*fallback, fbC, fbM)
		trace("exit hunt fallback")
		return
	}
	trace("exit hunt no divergence in %d candidates", rounds*per)
}

func verify(test string) {
	cleanTree, err := prepTree("vclean", false)
	if err != nil {
		fatalf("prep vclean: %v", err)
	}
	mutTree, err := prepTree("vmut", false)
	if err != nil {
		fatalf("prep vmut: %v", err)
	}
	if err := spliceMutant(mutTree); err != nil {
		fatalf("splice: %v", err)
	}
	trace("entry verify test=%s", test)
	wantKill := false
	for _, tree := range [...]string{cleanTree, mutTree} {
		cmd := exec.Command("go", "test", "-count=1", "-run", test, "./internal/vcf")
		cmd.Dir = tree
		cmd.Env = treeEnv(tree)
		out, err := cmd.CombinedOutput()
		label := "clean"
		if wantKill {
			label = "mutant"
		}
		passed := (err == nil) != wantKill
		status := "FAIL"
		if passed {
			status = "OK"
		}
		trace("verify %s: %s", label, status)
		if len(out) > 0 {
			fmt.Println(string(out))
		}
		if !passed {
			trace("exit verify unexpected %s outcome", label)
			os.Exit(1)
		}
		wantKill = true
	}
	trace("exit verify clean passed and mutant killed")
}

func main() {
	mode := flag.String("mode", "hunt", "hunt or verify")
	seed := flag.Int("seed", 1, "rng seed")
	rounds := flag.Int("rounds", 6, "hunt rounds")
	per := flag.Int("per", 20000, "candidates per round")
	budget := flag.Int("budget", 8000, "per candidate node budget")
	test := flag.String("test", "", "verify go test -run filter")
	keep := flag.Bool("keep", false, "keep scratch trees")
	flag.Parse()
	huntBudget = *budget

	cwd, err := os.Getwd()
	if err != nil {
		fatalf("cwd: %v", err)
	}
	repoRoot = filepath.Dir(filepath.Dir(cwd))
	if _, err := os.Stat(filepath.Join(repoRoot, "internal", "vcf", "search.go")); err != nil {
		fatalf("repo root %s missing internal/vcf/search.go: %v", repoRoot, err)
	}
	workRoot = filepath.Join(cwd, "work")
	os.RemoveAll(workRoot)
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		fatalf("work dir: %v", err)
	}
	if !*keep {
		defer os.RemoveAll(workRoot)
	}

	switch *mode {
	case "hunt":
		hunt(*seed, *rounds, *per)
	case "verify":
		if *test == "" {
			fatalf("verify needs -test")
		}
		verify(*test)
	default:
		fatalf("unknown mode %s", *mode)
	}
}

const driverSrc = `package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for sc.Scan() {
		fields := strings.Fields(strings.TrimSpace(sc.Text()))
		if len(fields) < 6 {
			fmt.Fprintln(os.Stderr, "hunt: short line")
			os.Exit(1)
		}
		vals := make([]int, 5)
		for i := range vals {
			v, err := strconv.Atoi(fields[i])
			if err != nil {
				fmt.Fprintln(os.Stderr, "hunt: bad field", fields[i])
				os.Exit(1)
			}
			vals[i] = v
		}
		b := rules.NewBoard()
		seen := make(map[rules.Cell]bool)
		for _, tok := range fields[5:] {
			parts := strings.Split(tok, ",")
			if len(parts) != 3 {
				fmt.Fprintln(os.Stderr, "hunt: bad stone", tok)
				os.Exit(1)
			}
			nums := make([]int, 3)
			for i := range nums {
				v, err := strconv.Atoi(parts[i])
				if err != nil {
					fmt.Fprintln(os.Stderr, "hunt: bad stone", tok)
					os.Exit(1)
				}
				nums[i] = v
			}
			cell := rules.Cell(nums[0]*config.BoardStride + nums[1])
			if seen[cell] {
				continue
			}
			seen[cell] = true
			b.Side = rules.Blue
			if nums[2] == 1 {
				b.Side = rules.Red
			}
			b.Make(cell)
		}
		b.Side = rules.Blue
		last := rules.Cell(vals[3]*config.BoardStride + vals[4])
		res := vcf.HuntProbe(vcf.KindVCT, vals[0], vals[1], vals[2], b, last)
		fmt.Fprintf(out, "%d %d %d %d", res.Path, res.Result, res.N, res.Nodes)
		for i := 0; i < res.N; i++ {
			fmt.Fprintf(out, " %d", res.Cells[i])
		}
		fmt.Fprintln(out)
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "hunt:", err)
		os.Exit(1)
	}
}
`

const probeSrc = `package vcf

import (
	"math/bits"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const (
	HuntPathGarbage     = 0
	HuntPathDefenderWin = 1
	HuntPathFour        = 2
	HuntPathNotThree    = 3
	HuntPathThree       = 4
)

type HuntOut struct {
	Path   uint8
	Result uint8
	N      int
	Nodes  uint64
	Cells  [config.MaxBoardCells]rules.Cell
}

func huntPrime(kind Kind, plyCap, budget int) *Solver {
	s := New(kind)
	s.nodes, s.budget, s.check, s.aborted = 0, budget, config.SolverNodeCheckInterval, false
	s.plyCap = plyCap
	return s
}

func huntExistingWin(b *rules.Board) bool {
	cur := b.Side
	defer func() { b.Side = cur }()
	for _, side := range [...]rules.Color{rules.Red, rules.Blue} {
		src := &b.Red
		if side == rules.Blue {
			src = &b.Blue
		}
		b.Side = side
		for w := range b.Geom.Words {
			bitsLeft := src[w]
			for bitsLeft != 0 {
				m := bitsLeft & (^bitsLeft + 1)
				bitsLeft ^= m
				cell := rules.Cell(w*64 + bits.TrailingZeros64(m))
				if b.FastLastMoveWin(side, cell) {
					return true
				}
			}
		}
	}
	return false
}

func HuntProbe(kind Kind, ply, plyCap, budget int, b *rules.Board, last rules.Cell) HuntOut {
	var out HuntOut
	if huntExistingWin(b) {
		out.Path = HuntPathGarbage
		return out
	}
	attacker := b.Side.Opponent()
	cls := huntPrime(kind, plyCap, budget)
	nDef, _ := cls.probeWins(b, b.Side, nil)
	nA, _ := cls.probeWins(b, attacker, cls.winA[:])
	switch {
	case nDef > 0:
		out.Path = HuntPathDefenderWin
	case nA > 0:
		out.Path = HuntPathFour
	case !realThree(b, attacker, last):
		out.Path = HuntPathNotThree
	default:
		out.Path = HuntPathThree
		out.N = cls.threatReplies(b, attacker, last, ply)
		copy(out.Cells[:], cls.defends[ply][:out.N])
	}
	s := huntPrime(kind, plyCap, budget)
	out.Result = uint8(s.defend(b, last, ply))
	out.Nodes = uint64(s.nodes)
	return out
}
`
