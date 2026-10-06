// mtri is the mutation-triage driver: it reuses cmd/mutate's exact splice
// grammar (the collect logic copied verbatim) to apply a mutant by its
// report key against the scratch module copy in ../work, run the package
// suite the way the gate does, and restore the file.
//
// Usage:
//
//	mtri keys <file>...            list mutant keys for files under work/
//	mtri anchor <allowlist>        check every allowlist entry against work/
//	mtri run [-run RE] [-v] <key>  apply one mutant, test, restore, report
package main

import (
	"bytes"
	"encoding/json"
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

type edit struct {
	start int
	end   int
	repl  string
}

type mutation struct {
	file  string
	pkg   string
	line  int
	col   int
	desc  string
	edits []edit
}

func (m mutation) key(workDir string) string {
	return fmt.Sprintf("%s:%d:%d %s", displayPath(workDir, m.file), m.line, m.col, m.desc)
}

func displayPath(workDir, path string) string {
	rel, err := filepath.Rel(workDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// collect is copied verbatim from cmd/mutate/mutate.go so the splice
// grammar, positions, and descriptors match the gate exactly.
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
					file:  filename,
					line:  line,
					col:   col,
					desc:  fmt.Sprintf("replace %s with %s", e.Op, swap),
					edits: []edit{{start: off(e.OpPos), end: off(e.OpPos) + len(e.Op.String()), repl: swap.String()}},
				})
			}
		case *ast.IfStmt:
			ms = append(ms, negate(fset, e.Cond, filename)...)
		case *ast.ForStmt:
			if e.Cond != nil {
				ms = append(ms, negate(fset, e.Cond, filename)...)
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
					file:  filename,
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
				file:  filename,
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
				file:  filename,
				line:  line,
				col:   col,
				desc:  fmt.Sprintf("replace %s with %s", e.Tok, repl),
				edits: []edit{{start: off(e.TokPos), end: off(e.TokPos) + len(e.Tok.String()), repl: repl.String()}},
			})
		}
		return true
	})
	sortMutants(ms)
	return ms, nil
}

func negate(fset *token.FileSet, cond ast.Expr, filename string) []mutation {
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
			file:  filename,
			line:  p.Line,
			col:   p.Column,
			desc:  "drop !",
			edits: []edit{{start: p.Offset, end: p.Offset + 1, repl: ""}},
		}}
	}
	s, e := fset.Position(cond.Pos()), fset.Position(cond.End())
	return []mutation{{
		file:  filename,
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

func sortMutants(ms []mutation) {
	sort.Slice(ms, func(i, j int) bool {
		if ms[i].file != ms[j].file {
			return ms[i].file < ms[j].file
		}
		if ms[i].line != ms[j].line {
			return ms[i].line < ms[j].line
		}
		if ms[i].col != ms[j].col {
			return ms[i].col < ms[j].col
		}
		return ms[i].desc < ms[j].desc
	})
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

func workDir() string {
	for _, cand := range []string{"work", "mutkill-scratch/work", "./mutkill-scratch/work"} {
		if _, err := os.Stat(filepath.Join(cand, "go.mod")); err == nil {
			abs, _ := filepath.Abs(cand)
			return abs
		}
	}
	panic("mtri: cannot locate the work/ module copy; run from playground/")
}

// pkgOf maps a file path under work/ to its test package pattern.
func pkgOf(workDir, file string) string {
	rel := displayPath(workDir, file)
	parts := strings.Split(rel, "/")
	dir := strings.Join(parts[:len(parts)-1], "/")
	if dir == "" {
		return "."
	}
	return "./" + dir
}

func collectAll(workDir string, relFiles []string) (map[string]mutation, error) {
	keys := map[string]mutation{}
	for _, rel := range relFiles {
		path := filepath.Join(workDir, filepath.FromSlash(rel))
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		ms, err := collect(path, src)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			m.pkg = pkgOf(workDir, m.file)
			keys[m.key(workDir)] = m
		}
	}
	return keys, nil
}

// prodFiles lists the non-test go files of the given package dirs under
// work/, mirroring go list GoFiles.
func prodFiles(workDir string, pkgs []string) ([]string, error) {
	cmd := exec.Command("go", append([]string{"list", "-e", "-json"}, pkgs...)...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %v", err)
	}
	var files []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p struct {
			Dir     string
			GoFiles []string
		}
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		for _, f := range p.GoFiles {
			rel, err := filepath.Rel(workDir, filepath.Join(p.Dir, f))
			if err != nil {
				return nil, err
			}
			files = append(files, filepath.ToSlash(rel))
		}
	}
	sort.Strings(files)
	return files, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mtri keys|anchor|run ...")
		os.Exit(2)
	}
	wd := workDir()
	switch os.Args[1] {
	case "keys":
		if err := runKeys(wd, os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "mtri:", err)
			os.Exit(1)
		}
	case "anchor":
		if err := runAnchor(wd, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "mtri:", err)
			os.Exit(1)
		}
	case "run":
		if err := runOne(wd, os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "mtri:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "mtri: unknown subcommand", os.Args[1])
		os.Exit(2)
	}
}

func runKeys(wd string, relFiles []string) error {
	if len(relFiles) == 0 {
		return fmt.Errorf("keys: want at least one file")
	}
	for _, rel := range relFiles {
		path := filepath.Join(wd, filepath.FromSlash(rel))
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ms, err := collect(path, src)
		if err != nil {
			return err
		}
		for _, m := range ms {
			fmt.Println(m.key(wd))
		}
	}
	return nil
}

func runAnchor(wd string, allowPath string) error {
	data, err := os.ReadFile(allowPath)
	if err != nil {
		return err
	}
	files, err := prodFiles(wd, []string{"./internal/rules", "./internal/engine", "./internal/clock"})
	if err != nil {
		return err
	}
	keys, err := collectAll(wd, files)
	if err != nil {
		return err
	}
	var matched, unmatched int
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, " # ")
		if !ok {
			return fmt.Errorf("%s:%d: malformed entry", allowPath, n+1)
		}
		if _, hit := keys[key]; hit {
			matched++
			continue
		}
		unmatched++
		fmt.Printf("UNMATCHED %s\n", key)
	}
	fmt.Printf("\n%d matched, %d unmatched, %d total mutants in tree\n", matched, unmatched, len(keys))
	return nil
}

func runOne(wd string, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	runRE := fs.String("run", "", "go test -run filter")
	timeout := fs.Duration("timeout", 5*time.Minute, "per-test timeout")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("run: want exactly one mutant key")
	}
	key := fs.Arg(0)
	files, err := prodFiles(wd, []string{"./internal/rules", "./internal/engine", "./internal/clock"})
	if err != nil {
		return err
	}
	keys, err := collectAll(wd, files)
	if err != nil {
		return err
	}
	m, ok := keys[key]
	if !ok {
		return fmt.Errorf("no such mutant in tree: %s", key)
	}
	orig, err := os.ReadFile(m.file)
	if err != nil {
		return err
	}
	mutated, err := applyEdits(orig, m.edits)
	if err != nil {
		return err
	}
	if werr := os.WriteFile(m.file, mutated, 0o644); werr != nil {
		return werr
	}
	defer os.WriteFile(m.file, orig, 0o644)

	testArgs := []string{"test", "-count=1", "-short", "-timeout", timeout.String()}
	if *runRE != "" {
		testArgs = append(testArgs, "-run", *runRE)
	}
	testArgs = append(testArgs, m.pkg)
	cmd := exec.Command("go", testArgs...)
	cmd.Dir = wd
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOMAXPROCS=4")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("KILLED %s\n--- output tail ---\n%s\n", key, tail(string(out), 4000))
		return nil
	}
	fmt.Printf("SURVIVED %s\n", key)
	return nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
