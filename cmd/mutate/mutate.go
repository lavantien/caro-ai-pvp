package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
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
