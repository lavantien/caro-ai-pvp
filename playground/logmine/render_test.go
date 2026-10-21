package main

import (
	"strings"
	"testing"
)

func TestRenderReportDeterministic(t *testing.T) {
	db := fixtureDB(t)
	m1 := mine(db)
	a1 := renderReport("gates-x", `db\gates-x.db`, m1)
	m2 := mine(db)
	a2 := renderReport("gates-x", `db\gates-x.db`, m2)
	if a1 != a2 {
		t.Fatal("render not deterministic")
	}
	for _, want := range []string{
		"# logmine: gates-x", "source: db\\gates-x.db",
		"## tier x time control", "| easy | 1+0 | 1 | 1 | 10/10/10 |",
		"| hard | 1+0 | 2 | 2 | 16/16/16 |",
		"## grant x depth", "| easy | 2-5 | 1 |", "| hard | 10-20 | 2 |",
		"## red first", "| 1+0 | easy | hard | 3 | 1 |",
		"## game outcomes", "| 1+0 | easy | hard | red | 1 |", "| 1+0 | hard | easy | blue | 1 |",
		"## grant usage", "| hard | 1+0 | 2 |", "## tags", "| VCT | 1 |", "## casual m-lines",
		"| hard | 2+1 | 1 | 1 | 9/9/9 |", "| human | 2+1 | 1 | 1 | 8/8/8 |",
	} {
		if !strings.Contains(a1, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestRenderEmptyDB(t *testing.T) {
	m := mined{}
	out := renderReport("empty", "empty.db", m)
	if !strings.Contains(out, "# logmine: empty") || !strings.Contains(out, "no move data") {
		t.Errorf("empty render = %q", out)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		`db\gates-v020.db`:   "db__gates-v020",
		`docs\evidence\a.db`: "docs__evidence__a",
		"plain.db":           "plain",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}
