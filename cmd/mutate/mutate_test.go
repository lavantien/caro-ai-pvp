package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func collectSource(t *testing.T, filename, src string) []mutation {
	t.Helper()
	ms, err := collect(filename, []byte(src))
	if err != nil {
		t.Fatalf("collect(%s) err = %v", filename, err)
	}
	return ms
}

func collectFile(t *testing.T, path string) []mutation {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) err = %v", path, err)
	}
	return collectSource(t, path, string(src))
}

func descs(ms []mutation) string {
	var out []string
	for _, m := range ms {
		out = append(out, m.desc)
	}
	return strings.Join(out, ", ")
}

func findMutant(t *testing.T, ms []mutation, path string, line int, desc string) mutation {
	t.Helper()
	for _, m := range ms {
		if m.line == line && m.desc == desc {
			return m
		}
	}
	t.Fatalf("no mutant at %s:%d with desc %q, have: %s", path, line, desc, descs(ms))
	return mutation{}
}

func TestCollectFixtureOperators(t *testing.T) {
	path := filepath.Join("testdata", "arith.go")
	ms := collectFile(t, path)
	m := findMutant(t, ms, path, 4, "replace + with -")
	if m.col != 11 {
		t.Errorf("+ swap col = %d, want 11", m.col)
	}
	findMutant(t, ms, path, 8, "replace - with +")
	findMutant(t, ms, path, 12, "replace < with <=")
	findMutant(t, ms, path, 15, "replace > with >=")

	path = filepath.Join("testdata", "logic.go")
	ms = collectFile(t, path)
	findMutant(t, ms, path, 4, "replace && with ||")
	findMutant(t, ms, path, 8, "replace || with &&")
	findMutant(t, ms, path, 16, "replace << with >>")
	findMutant(t, ms, path, 16, "replace >> with <<")
	findMutant(t, ms, path, 21, "replace true with false")
	findMutant(t, ms, path, 23, "replace false with true")
	var maskLine []string
	for _, m := range ms {
		if m.line == 12 {
			maskLine = append(maskLine, m.desc)
		}
	}
	sort.Strings(maskLine)
	want := []string{"replace & with |", "replace | with &"}
	if strings.Join(maskLine, "|") != strings.Join(want, "|") {
		t.Errorf("line 12 mutants = %v, want %v (xor must stay untouched)", maskLine, want)
	}
}

func TestCollectFixtureLiterals(t *testing.T) {
	path := filepath.Join("testdata", "loops.go")
	ms := collectFile(t, path)
	findMutant(t, ms, path, 6, "replace 100 with 101")
	findMutant(t, ms, path, 6, "replace 100 with 99")
	findMutant(t, ms, path, 4, "replace 0 with 1")
	findMutant(t, ms, path, 4, "replace 0 with -1")
	findMutant(t, ms, path, 5, "replace 1 with 2")
	findMutant(t, ms, path, 5, "replace 1 with 0")
}

func TestCollectFixtureControlFlow(t *testing.T) {
	path := filepath.Join("testdata", "loops.go")
	ms := collectFile(t, path)
	findMutant(t, ms, path, 7, "replace break with continue")
	findMutant(t, ms, path, 18, "replace continue with break")
	findMutant(t, ms, path, 5, "negate condition")
	findMutant(t, ms, path, 17, "negate condition")

	path = filepath.Join("testdata", "arith.go")
	ms = collectFile(t, path)
	findMutant(t, ms, path, 12, "negate condition")
	findMutant(t, ms, path, 15, "negate condition")
}

func TestCollectFixtureCounts(t *testing.T) {
	for _, tc := range []struct {
		file string
		want int
	}{
		{file: "arith.go", want: 12},
		{file: "logic.go", want: 14},
		{file: "loops.go", want: 16},
	} {
		if ms := collectFile(t, filepath.Join("testdata", tc.file)); len(ms) != tc.want {
			t.Errorf("%s: %d mutants, want %d: %s", tc.file, len(ms), tc.want, descs(ms))
		}
	}
}

func TestCollectDeterministic(t *testing.T) {
	path := filepath.Join("testdata", "loops.go")
	a := collectFile(t, path)
	b := collectFile(t, path)
	if len(a) == 0 {
		t.Fatal("no mutants collected")
	}
	for i := range a {
		if a[i].line != b[i].line || a[i].col != b[i].col || a[i].desc != b[i].desc {
			t.Fatalf("mutant %d differs across runs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestCollectNegationForms(t *testing.T) {
	ms := collectSource(t, "n.go", "package p\n\nfunc f(x bool) {\n\tif x {\n\t}\n\tif !x {\n\t}\n\tif (x) {\n\t}\n\tif (!x) {\n\t}\n\tfor {\n\t}\n}\n")
	findMutant(t, ms, "n.go", 4, "negate condition")
	findMutant(t, ms, "n.go", 6, "drop !")
	findMutant(t, ms, "n.go", 8, "negate condition")
	findMutant(t, ms, "n.go", 10, "drop !")
	var n int
	for _, m := range ms {
		if m.desc == "negate condition" || m.desc == "drop !" {
			n++
		}
	}
	if n != 4 {
		t.Errorf("%d negation mutants, want 4 (nil for-cond must be skipped): %s", n, descs(ms))
	}
}

func TestCollectLiteralForms(t *testing.T) {
	ms := collectSource(t, "l.go", "package p\n\nconst (\n\ta = 0x10\n\tb = 1_000\n\tc = 9223372036854775808\n\td = 7\n)\n")
	findMutant(t, ms, "l.go", 4, "replace 0x10 with 17")
	findMutant(t, ms, "l.go", 4, "replace 0x10 with 15")
	findMutant(t, ms, "l.go", 5, "replace 1_000 with 1001")
	findMutant(t, ms, "l.go", 5, "replace 1_000 with 999")
	findMutant(t, ms, "l.go", 7, "replace 7 with 8")
	findMutant(t, ms, "l.go", 7, "replace 7 with 6")
	for _, m := range ms {
		if m.line == 6 {
			t.Errorf("overflow literal mutated: %+v", m)
		}
	}
}

func TestCollectGeneratedFileSkipped(t *testing.T) {
	src := "// Code generated by tool. DO NOT EDIT.\n\npackage p\n\nfunc f(a int) int {\n\treturn a + 1\n}\n"
	if ms := collectSource(t, "g.go", src); len(ms) != 0 {
		t.Errorf("generated file yielded %d mutants, want 0: %s", len(ms), descs(ms))
	}
	src = "package p\n\n// Code generated by tool. DO NOT EDIT.\n\nfunc f(a int) int {\n\treturn a + 1\n}\n"
	if ms := collectSource(t, "g2.go", src); len(ms) != 3 {
		t.Errorf("marker after package clause skipped file, want 3 mutants: %s", descs(ms))
	}
}

func TestCollectSyntaxError(t *testing.T) {
	if _, err := collect("bad.go", []byte("package p\nfunc {\n")); err == nil {
		t.Error("collect(bad syntax) err = nil, want error")
	}
}

func TestCollectNonMutableNodesUntouched(t *testing.T) {
	src := "package p\n\nvar s = \"a && b\"\n\nfunc f(p *int, c chan int) int {\n\tq := &s\n\t<-c\n\treturn *p\n}\n"
	if ms := collectSource(t, "u.go", src); len(ms) != 0 {
		t.Errorf("non-mutable source yielded %d mutants, want 0: %s", len(ms), descs(ms))
	}
}

func TestApplyEdits(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		line int
		desc string
		want string
	}{
		{
			name: "negate wraps condition",
			src:  "package p\nfunc f(x bool) {\n\tif x {\n\t}\n}\n",
			line: 3, desc: "negate condition",
			want: "package p\nfunc f(x bool) {\n\tif !(x) {\n\t}\n}\n",
		},
		{
			name: "drop removes bang",
			src:  "package p\nfunc f(x bool) {\n\tif !x {\n\t}\n}\n",
			line: 3, desc: "drop !",
			want: "package p\nfunc f(x bool) {\n\tif x {\n\t}\n}\n",
		},
		{
			name: "operator splice keeps formatting",
			src:  "package p\nfunc f(a int) int { return a + 1 }\n",
			line: 2, desc: "replace + with -",
			want: "package p\nfunc f(a int) int { return a - 1 }\n",
		},
		{
			name: "branch splice",
			src:  "package p\nfunc f() {\n\tfor {\n\t\tbreak\n\t}\n}\n",
			line: 4, desc: "replace break with continue",
			want: "package p\nfunc f() {\n\tfor {\n\t\tcontinue\n\t}\n}\n",
		},
		{
			name: "literal delta uses decimal",
			src:  "package p\nconst a = 0x10\n",
			line: 2, desc: "replace 0x10 with 15",
			want: "package p\nconst a = 15\n",
		},
		{
			name: "boolean flip",
			src:  "package p\nfunc f() bool {\n\treturn true\n}\n",
			line: 3, desc: "replace true with false",
			want: "package p\nfunc f() bool {\n\treturn false\n}\n",
		},
	} {
		ms := collectSource(t, "a.go", tc.src)
		m := findMutant(t, ms, "a.go", tc.line, tc.desc)
		got, err := applyEdits([]byte(tc.src), m.edits)
		if err != nil {
			t.Fatalf("%s: applyEdits err = %v", tc.name, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestApplyEditsRejectsBadEdits(t *testing.T) {
	if _, err := applyEdits([]byte("abc"), []edit{{start: 5, end: 9, repl: "x"}}); err == nil {
		t.Error("out-of-bounds edit err = nil, want error")
	}
	if _, err := applyEdits([]byte("abc"), []edit{{start: 1, end: 3, repl: "x"}, {start: 0, end: 2, repl: "y"}}); err == nil {
		t.Error("overlapping edit err = nil, want error")
	}
	if _, err := applyEdits([]byte("abc"), []edit{{start: 1, end: 3, repl: "x"}, {start: 1, end: 2, repl: "y"}}); err == nil {
		t.Error("same-start overlapping edit err = nil, want error")
	}
	if _, err := applyEdits([]byte("abc"), []edit{{start: 2, end: 1, repl: "x"}}); err == nil {
		t.Error("inverted edit err = nil, want error")
	}
}

func TestApplyAllFixtureMutantsParse(t *testing.T) {
	for _, name := range []string{"arith.go", "logic.go", "loops.go"} {
		path := filepath.Join("testdata", name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) err = %v", path, err)
		}
		ms := collectFile(t, path)
		if len(ms) == 0 {
			t.Fatalf("%s: no mutants", name)
		}
		for _, m := range ms {
			got, err := applyEdits(src, m.edits)
			if err != nil {
				t.Fatalf("%s:%d:%d %s: applyEdits err = %v", name, m.line, m.col, m.desc, err)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), name, got, 0); err != nil {
				t.Errorf("%s:%d:%d %s: mutant does not parse: %v", name, m.line, m.col, m.desc, err)
			}
		}
	}
}
