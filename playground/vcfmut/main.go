package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var binarySwaps = map[token.Token]token.Token{
	token.EQL:  token.NEQ,
	token.NEQ:  token.EQL,
	token.LSS:  token.LEQ,
	token.LEQ:  token.LSS,
	token.GTR:  token.GEQ,
	token.GEQ:  token.GTR,
	token.ADD:  token.SUB,
	token.SUB:  token.ADD,
	token.MUL:  token.QUO,
	token.QUO:  token.MUL,
	token.LAND: token.LOR,
	token.LOR:  token.LAND,
	token.AND:  token.OR,
	token.OR:   token.AND,
	token.SHL:  token.SHR,
	token.SHR:  token.SHL,
}

var generatedRe = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

var surviveRe = regexp.MustCompile(`^(internal/vcf/\S+\.go:\d+:\d+ .+) SURVIVED$`)

type edit struct {
	start int
	end   int
	repl  string
}

type mutation struct {
	rel   string
	line  int
	col   int
	desc  string
	edits []edit
}

func (m mutation) key() string {
	return fmt.Sprintf("%s:%d:%d %s", m.rel, m.line, m.col, m.desc)
}

var repoRoot string

var relRoot string

func collect(filename string, src []byte) ([]mutation, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	if isGenerated(f) {
		return nil, nil
	}
	var ms []mutation
	rel, _ := filepath.Rel(relRoot, filename)
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	at := func(p token.Pos) (int, int) {
		pp := fset.Position(p)
		return pp.Line, pp.Column
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.BinaryExpr:
			if swap, ok := binarySwaps[e.Op]; ok {
				line, col := at(e.OpPos)
				ms = append(ms, mutation{
					rel:   filepath.ToSlash(rel),
					line:  line,
					col:   col,
					desc:  fmt.Sprintf("replace %s with %s", e.Op, swap),
					edits: []edit{{start: off(e.OpPos), end: off(e.OpPos) + len(e.Op.String()), repl: swap.String()}},
				})
			}
		case *ast.IfStmt:
			ms = append(ms, negate(fset, e.Cond, rel)...)
		case *ast.ForStmt:
			if e.Cond != nil {
				ms = append(ms, negate(fset, e.Cond, rel)...)
			}
		case *ast.BasicLit:
			if e.Kind != token.INT {
				return true
			}
			v, err := strconv.ParseInt(e.Value, 0, 64)
			if err != nil {
				return true
			}
			line, col := at(e.Pos())
			for _, d := range [...]int64{1, -1} {
				w := v + d
				ms = append(ms, mutation{
					rel:   filepath.ToSlash(rel),
					line:  line,
					col:   col,
					desc:  fmt.Sprintf("replace %s with %d", e.Value, w),
					edits: []edit{{start: off(e.Pos()), end: off(e.Pos()) + len(e.Value), repl: strconv.FormatInt(w, 10)}},
				})
			}
		case *ast.Ident:
			if e.Name != "true" && e.Name != "false" {
				return true
			}
			repl := "true"
			if e.Name == "true" {
				repl = "false"
			}
			line, col := at(e.Pos())
			ms = append(ms, mutation{
				rel:   filepath.ToSlash(rel),
				line:  line,
				col:   col,
				desc:  fmt.Sprintf("replace %s with %s", e.Name, repl),
				edits: []edit{{start: off(e.Pos()), end: off(e.Pos()) + len(e.Name), repl: repl}},
			})
		case *ast.BranchStmt:
			var repl token.Token
			switch e.Tok {
			case token.BREAK:
				repl = token.CONTINUE
			case token.CONTINUE:
				repl = token.BREAK
			default:
				return true
			}
			line, col := at(e.TokPos)
			ms = append(ms, mutation{
				rel:   filepath.ToSlash(rel),
				line:  line,
				col:   col,
				desc:  fmt.Sprintf("replace %s with %s", e.Tok, repl),
				edits: []edit{{start: off(e.TokPos), end: off(e.TokPos) + len(e.Tok.String()), repl: repl.String()}},
			})
		}
		return true
	})
	sort.Slice(ms, func(i, j int) bool {
		if ms[i].rel != ms[j].rel {
			return ms[i].rel < ms[j].rel
		}
		if ms[i].line != ms[j].line {
			return ms[i].line < ms[j].line
		}
		if ms[i].col != ms[j].col {
			return ms[i].col < ms[j].col
		}
		return ms[i].desc < ms[j].desc
	})
	return ms, nil
}

func negate(fset *token.FileSet, cond ast.Expr, rel string) []mutation {
	inner := cond
	for {
		p, ok := inner.(*ast.ParenExpr)
		if !ok {
			break
		}
		inner = p.X
	}
	if u, ok := inner.(*ast.UnaryExpr); ok && u.Op == token.NOT {
		p := fset.Position(u.OpPos)
		return []mutation{{
			rel:   filepath.ToSlash(rel),
			line:  p.Line,
			col:   p.Column,
			desc:  "drop !",
			edits: []edit{{start: p.Offset, end: p.Offset + 1, repl: ""}},
		}}
	}
	s, e := fset.Position(cond.Pos()), fset.Position(cond.End())
	return []mutation{{
		rel:   filepath.ToSlash(rel),
		line:  s.Line,
		col:   s.Column,
		desc:  "negate condition",
		edits: []edit{{start: e.Offset, end: e.Offset, repl: ")"}, {start: s.Offset, end: s.Offset, repl: "!("}},
	}}
}

func isGenerated(f *ast.File) bool {
	for _, g := range f.Comments {
		if g.Pos() >= f.Package {
			break
		}
		for _, c := range g.List {
			if generatedRe.MatchString(c.Text) {
				return true
			}
		}
	}
	return false
}

func applyEdits(src []byte, edits []edit) ([]byte, error) {
	es := make([]edit, len(edits))
	copy(es, edits)
	sort.Slice(es, func(i, j int) bool {
		if es[i].start != es[j].start {
			return es[i].start < es[j].start
		}
		return es[i].end < es[j].end
	})
	var out []byte
	prev := 0
	for _, e := range es {
		if e.start < prev || e.start > e.end || e.end > len(src) {
			return nil, fmt.Errorf("bad or overlapping edit [%d,%d) of %d bytes", e.start, e.end, len(src))
		}
		out = append(out, src[prev:e.start]...)
		out = append(out, e.repl...)
		prev = e.end
	}
	out = append(out, src[prev:]...)
	return out, nil
}

func loadKeys(logPath string) ([]string, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, err
	}
	var keys []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		if m := surviveRe.FindStringSubmatch(sc.Text()); m != nil {
			keys = append(keys, m[1])
		}
	}
	return keys, sc.Err()
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
		if d.Type().IsRegular() {
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			return os.WriteFile(target, data, 0o644)
		}
		return nil
	})
}

func refreshWork(work string) error {
	if err := os.RemoveAll(work); err != nil {
		return err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	for _, name := range [...]string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := os.WriteFile(filepath.Join(work, name), data, 0o644); err != nil {
			return err
		}
	}
	if err := copyTree(filepath.Join(repoRoot, "internal"), filepath.Join(work, "internal")); err != nil {
		return err
	}
	rootGo, err := filepath.Glob(filepath.Join(repoRoot, "*.go"))
	if err != nil {
		return err
	}
	for _, p := range rootGo {
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if werr := os.WriteFile(filepath.Join(work, filepath.Base(p)), data, 0o644); werr != nil {
			return werr
		}
	}
	return nil
}

func collectWork(root string) (map[string]mutation, error) {
	relRoot = root
	files := []string{
		filepath.Join(root, "internal", "vcf", "solver.go"),
		filepath.Join(root, "internal", "vcf", "search.go"),
		filepath.Join(root, "internal", "vcf", "threat.go"),
	}
	keys := map[string]mutation{}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		ms, err := collect(path, src)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			keys[m.key()] = m
		}
	}
	return keys, nil
}

func mutatedLine(path string, m mutation) (string, string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	mutated, err := applyEdits(src, m.edits)
	if err != nil {
		return "", "", err
	}
	origLines := strings.Split(string(src), "\n")
	mutLines := strings.Split(string(mutated), "\n")
	o := strings.TrimSpace(origLines[m.line-1])
	n := strings.TrimSpace(mutLines[m.line-1])
	return o, n, nil
}

func main() {
	logPath := flag.String("log", `..\..\logs\archive\mutate-refaudit-vcf-rerun2.log`, "archived mutate log")
	workers := flag.Int("workers", 4, "parallel work trees")
	runRE := flag.String("run", "", "go test -run filter")
	timeout := flag.Duration("timeout", 150*time.Second, "per-mutant test timeout")
	mode := flag.String("mode", "run", "probe or run")
	noRefresh := flag.Bool("no-refresh", false, "reuse existing work trees")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vcfmut:", err)
		os.Exit(1)
	}
	base, _ := filepath.Abs(cwd)
	repoRoot = filepath.Dir(filepath.Dir(base))

	want, err := loadKeys(*logPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vcfmut:", err)
		os.Exit(1)
	}
	fmt.Printf("vcfmut: %d survivor keys from log\n", len(want))

	if !*noRefresh {
		for i := 0; i < *workers; i++ {
			if err := refreshWork(filepath.Join(base, fmt.Sprintf("work%d", i))); err != nil {
				fmt.Fprintln(os.Stderr, "vcfmut:", err)
				os.Exit(1)
			}
		}
	}

	first, err := collectWork(filepath.Join(base, "work0"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "vcfmut:", err)
		os.Exit(1)
	}
	var jobs []mutation
	for _, k := range want {
		m, ok := first[k]
		if !ok {
			fmt.Printf("MISSING %s\n", k)
			continue
		}
		jobs = append(jobs, m)
	}
	fmt.Printf("vcfmut: %d jobs matched of %d wanted\n", len(jobs), len(want))

	if *mode == "probe" {
		root := filepath.Join(base, "work0")
		for _, m := range jobs {
			o, n, err := mutatedLine(filepath.Join(root, filepath.FromSlash(m.rel)), m)
			if err != nil {
				fmt.Printf("%s\n  ERROR %v\n", m.key(), err)
				continue
			}
			fmt.Printf("%s\n  orig: %s\n  mut : %s\n", m.key(), o, n)
		}
		return
	}

	next := make(chan int)
	results := make([]string, len(jobs))
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			root := filepath.Join(base, fmt.Sprintf("work%d", w))
			env := append(os.Environ(),
				"CGO_ENABLED=1",
				"GOCACHE="+filepath.Join(base, fmt.Sprintf("gocache%d", w)),
				"GOTMPDIR="+filepath.Join(base, fmt.Sprintf("gotmp%d", w)),
				"GOMAXPROCS=2",
			)
			os.MkdirAll(filepath.Join(base, fmt.Sprintf("gocache%d", w)), 0o755)
			os.MkdirAll(filepath.Join(base, fmt.Sprintf("gotmp%d", w)), 0o755)
			for idx := range next {
				m := jobs[idx]
				path := filepath.Join(root, filepath.FromSlash(m.rel))
				orig, err := os.ReadFile(path)
				if err != nil {
					results[idx] = fmt.Sprintf("ERROR %s %v", m.key(), err)
					continue
				}
				mutated, err := applyEdits(orig, m.edits)
				if err != nil {
					results[idx] = fmt.Sprintf("ERROR %s %v", m.key(), err)
					continue
				}
				if err := os.WriteFile(path, mutated, 0o644); err != nil {
					results[idx] = fmt.Sprintf("ERROR %s %v", m.key(), err)
					continue
				}
				testArgs := []string{"test", "-count=1", "-short", "-timeout", (*timeout).String()}
				if *runRE != "" {
					testArgs = append(testArgs, "-run", *runRE)
				}
				testArgs = append(testArgs, "./internal/vcf")
				cmd := exec.Command("go", testArgs...)
				cmd.Dir = root
				cmd.Env = env
				_, err = cmd.CombinedOutput()
				os.WriteFile(path, orig, 0o644)
				if err != nil {
					results[idx] = fmt.Sprintf("KILLED   %s", m.key())
				} else {
					results[idx] = fmt.Sprintf("SURVIVED %s", m.key())
				}
			}
		}(w)
	}
	for i := 0; i < len(jobs); i++ {
		next <- i
	}
	close(next)
	wg.Wait()

	survived := 0
	for _, r := range results {
		fmt.Println(r)
		if strings.HasPrefix(r, "SURVIVED") {
			survived++
		}
	}
	fmt.Printf("vcfmut: %d/%d survived\n", survived, len(jobs))
}
