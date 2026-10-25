package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const badgeFixture = `mode: atomic
github.com/lavantien/caro-ai-pvp/internal/rules/win.go:10.2,11.10 2 1
github.com/lavantien/caro-ai-pvp/internal/rules/board.go:5.2,6.8 3 3
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:3.2,4.9 100 100
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:8.2,9.9 5 0
`

const badgeWarnFixture = `mode: atomic
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:3.2,4.9 92 92
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:8.2,9.9 8 0
`

const badgePinnedTime = "2026-10-25T12:00:00Z"

var pinnedNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func TestBadgeSVGBytes(t *testing.T) {
	cases := []struct {
		label string
		value string
		fill  string
		want  string
	}{
		{
			"global coverage", "95.5%", "#4c1",
			`<svg xmlns="http://www.w3.org/2000/svg" width="164" height="20" role="img" aria-label="global coverage: 95.5%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="164" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="164" height="20" fill="#555"/><rect x="117" width="47" height="20" fill="#4c1"/><rect width="164" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="58" y="14">global coverage</text><text x="140" y="14">95.5%</text></g></svg>`,
		},
		{
			"core coverage", "100.0%", "#4c1",
			`<svg xmlns="http://www.w3.org/2000/svg" width="157" height="20" role="img" aria-label="core coverage: 100.0%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="157" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="157" height="20" fill="#555"/><rect x="103" width="54" height="20" fill="#4c1"/><rect width="157" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="51" y="14">core coverage</text><text x="130" y="14">100.0%</text></g></svg>`,
		},
		{
			"core coverage", "0.0%", "#e05d44",
			`<svg xmlns="http://www.w3.org/2000/svg" width="143" height="20" role="img" aria-label="core coverage: 0.0%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="143" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="143" height="20" fill="#555"/><rect x="103" width="40" height="20" fill="#e05d44"/><rect width="143" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="51" y="14">core coverage</text><text x="123" y="14">0.0%</text></g></svg>`,
		},
	}
	for _, c := range cases {
		if got := badgeSVG(c.label, c.value, c.fill); got != c.want {
			t.Errorf("badgeSVG(%q, %q) = %q, want %q", c.label, c.value, got, c.want)
		}
	}
}

func TestBadgeJSONBytes(t *testing.T) {
	cases := []struct {
		label string
		value string
		color string
		want  string
	}{
		{"global coverage", "95.5%", "brightgreen", `{"schemaVersion":1,"label":"global coverage","message":"95.5%","color":"brightgreen"}`},
		{"global coverage", "92.0%", "yellow", `{"schemaVersion":1,"label":"global coverage","message":"92.0%","color":"yellow"}`},
		{"core coverage", "0.0%", "red", `{"schemaVersion":1,"label":"core coverage","message":"0.0%","color":"red"}`},
	}
	for _, c := range cases {
		if got := badgeJSON(c.label, c.value, c.color); got != c.want {
			t.Errorf("badgeJSON(%q, %q) = %q, want %q", c.label, c.value, got, c.want)
		}
	}
}

func TestBandThresholds(t *testing.T) {
	cases := []struct {
		p     float64
		good  float64
		warn  float64
		fill  string
		color string
	}{
		{95.0, 95, 90, "#4c1", "brightgreen"},
		{95.1, 95, 90, "#4c1", "brightgreen"},
		{94.9, 95, 90, "#dfb317", "yellow"},
		{90.0, 95, 90, "#dfb317", "yellow"},
		{89.9, 95, 90, "#e05d44", "red"},
		{0.0, 95, 90, "#e05d44", "red"},
		{100.0, 100, 100, "#4c1", "brightgreen"},
		{99.9, 100, 100, "#e05d44", "red"},
	}
	for _, c := range cases {
		fill, color := band(c.p, c.good, c.warn)
		if fill != c.fill || color != c.color {
			t.Errorf("band(%v, %v, %v) = %q %q, want %q %q", c.p, c.good, c.warn, fill, color, c.fill, c.color)
		}
	}
}

func TestWriteBadgesOutput(t *testing.T) {
	dir := t.TempDir()
	if err := writeBadges(mustParse(t, badgeFixture), dir, "abc123def", "9876543210", pinnedNow); err != nil {
		t.Fatalf("writeBadges err = %v", err)
	}
	assertFile(t, filepath.Join(dir, "coverage-global.svg"),
		`<svg xmlns="http://www.w3.org/2000/svg" width="164" height="20" role="img" aria-label="global coverage: 95.5%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="164" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="164" height="20" fill="#555"/><rect x="117" width="47" height="20" fill="#4c1"/><rect width="164" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="58" y="14">global coverage</text><text x="140" y="14">95.5%</text></g></svg>`)
	assertFile(t, filepath.Join(dir, "coverage-global.json"),
		`{"schemaVersion":1,"label":"global coverage","message":"95.5%","color":"brightgreen"}`)
	assertFile(t, filepath.Join(dir, "coverage-core.svg"),
		`<svg xmlns="http://www.w3.org/2000/svg" width="157" height="20" role="img" aria-label="core coverage: 100.0%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="157" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="157" height="20" fill="#555"/><rect x="103" width="54" height="20" fill="#4c1"/><rect width="157" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="51" y="14">core coverage</text><text x="130" y="14">100.0%</text></g></svg>`)
	assertFile(t, filepath.Join(dir, "coverage-core.json"),
		`{"schemaVersion":1,"label":"core coverage","message":"100.0%","color":"brightgreen"}`)
	assertFile(t, filepath.Join(dir, "source.json"),
		`{"schemaVersion":1,"sha":"abc123def","runId":"9876543210","global":95.5,"core":100,"timestamp":"`+badgePinnedTime+`"}`)
}

func TestWriteBadgesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := writeBadges(mustParse(t, badgeFixture), dir, "deadbeef", "17", pinnedNow); err != nil {
		t.Fatalf("writeBadges err = %v", err)
	}
	var src badgeSource
	data, err := os.ReadFile(filepath.Join(dir, "source.json"))
	if err != nil {
		t.Fatalf("ReadFile err = %v", err)
	}
	if err := json.Unmarshal(data, &src); err != nil {
		t.Fatalf("Unmarshal err = %v", err)
	}
	want := badgeSource{SchemaVersion: 1, Sha: "deadbeef", RunId: "17", Global: 95.5, Core: 100, Timestamp: badgePinnedTime}
	if !reflect.DeepEqual(src, want) {
		t.Errorf("source.json = %+v, want %+v", src, want)
	}
}

func TestWriteBadgesBandsEndToEnd(t *testing.T) {
	dir := t.TempDir()
	if err := writeBadges(mustParse(t, badgeWarnFixture), dir, "cafe", "9", pinnedNow); err != nil {
		t.Fatalf("writeBadges err = %v", err)
	}
	assertFile(t, filepath.Join(dir, "coverage-global.svg"),
		`<svg xmlns="http://www.w3.org/2000/svg" width="164" height="20" role="img" aria-label="global coverage: 92.0%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="164" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="164" height="20" fill="#555"/><rect x="117" width="47" height="20" fill="#dfb317"/><rect width="164" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="58" y="14">global coverage</text><text x="140" y="14">92.0%</text></g></svg>`)
	assertFile(t, filepath.Join(dir, "coverage-core.svg"),
		`<svg xmlns="http://www.w3.org/2000/svg" width="143" height="20" role="img" aria-label="core coverage: 0.0%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="143" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="143" height="20" fill="#555"/><rect x="103" width="40" height="20" fill="#e05d44"/><rect width="143" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="51" y="14">core coverage</text><text x="123" y="14">0.0%</text></g></svg>`)
}

func TestWriteBadgesEmptyProfile(t *testing.T) {
	err := writeBadges(mustParse(t, "mode: set\n"), t.TempDir(), "s", "1", pinnedNow)
	if err == nil || !strings.Contains(err.Error(), "empty profile") {
		t.Errorf("writeBadges(empty) err = %v, want empty profile failure", err)
	}
}

func TestWriteBadgesCreatesOutDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "badges")
	if err := writeBadges(mustParse(t, badgeFixture), dir, "s", "1", pinnedNow); err != nil {
		t.Fatalf("writeBadges(fresh dir) err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "source.json")); err != nil {
		t.Errorf("source.json missing in fresh dir: %v", err)
	}
}

func TestRunCLIBadges(t *testing.T) {
	if code := runCLI([]string{"-badges"}); code != 2 {
		t.Errorf("runCLI(-badges, no args) = %d, want 2", code)
	}
	if code := runCLI([]string{"-badges", "a", "b", "c"}); code != 2 {
		t.Errorf("runCLI(-badges, short args) = %d, want 2", code)
	}
	if code := runCLI([]string{"-badges", filepath.Join(t.TempDir(), "missing.out"), t.TempDir(), "s", "1"}); code != 1 {
		t.Errorf("runCLI(-badges, missing) = %d, want 1", code)
	}
	dir := t.TempDir()
	if code := runCLI([]string{"-badges", writeProfile(t, badgeFixture), dir, "abc", "42"}); code != 0 {
		t.Errorf("runCLI(-badges, pass) = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "source.json")); err != nil {
		t.Errorf("source.json missing after runCLI: %v", err)
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) err = %v", path, err)
	}
	if string(data) != want {
		t.Errorf("%s = %q, want %q", filepath.Base(path), string(data), want)
	}
}
