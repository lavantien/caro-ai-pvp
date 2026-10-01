package main

import (
	"errors"
	"strings"
	"testing"
)

func TestSplitPkgs(t *testing.T) {
	got := splitPkgs(" a , internal/rules,,./internal/engine ")
	want := "./a|./internal/rules|./internal/engine"
	parts := make([]string, 0, len(got))
	parts = append(parts, got...)
	if strings.Join(parts, "|") != want {
		t.Errorf("splitPkgs = %v, want %s", got, want)
	}
	if got := splitPkgs(" , ,"); len(got) != 0 {
		t.Errorf("splitPkgs(empty) = %v, want none", got)
	}
}

func TestExitCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		res  result
		want int
	}{
		{name: "all killed", res: result{killed: 4, survived: 0, run: 4, total: 4}, want: 0},
		{name: "allowed survivor", res: result{killed: 3, allowed: 1, run: 4, total: 4}, want: 0},
		{name: "survivor", res: result{killed: 3, survived: 1, run: 4, total: 4}, want: 1},
		{name: "run error", err: errors.New("boom"), want: 1},
		{name: "no mutants", res: result{}, want: 1},
		{name: "aborted", res: result{killed: 2, run: 2, total: 4}, want: 1},
	} {
		if got := exitCode(tc.err, tc.res); got != tc.want {
			t.Errorf("%s: exitCode = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestRunCLIUsage(t *testing.T) {
	for _, args := range [][]string{
		{"stray"},
		{"-timeout", "1s"},
		{"-pkgs", " , "},
		{"-badflag"},
	} {
		if code := runCLI(args, &strings.Builder{}, &strings.Builder{}, "."); code != 2 {
			t.Errorf("runCLI(%v) = %d, want 2", args, code)
		}
	}
}
