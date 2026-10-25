package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
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

const ciPassedSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="80" height="20" role="img" aria-label="ci: passed"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="80" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="80" height="20" fill="#555"/><rect x="26" width="54" height="20" fill="#4c1"/><rect width="80" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="13" y="14">ci</text><text x="53" y="14">passed</text></g></svg>`

const ciFailedSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="80" height="20" role="img" aria-label="ci: failed"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="80" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="80" height="20" fill="#555"/><rect x="26" width="54" height="20" fill="#e05d44"/><rect width="80" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="13" y="14">ci</text><text x="53" y="14">failed</text></g></svg>`

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

func TestVerdictBand(t *testing.T) {
	fill, color, err := verdictBand(config.VerdictPassed)
	if err != nil || fill != "#4c1" || color != "brightgreen" {
		t.Errorf("verdictBand(passed) = %q %q %v, want #4c1 brightgreen nil", fill, color, err)
	}
	fill, color, err = verdictBand(config.VerdictFailed)
	if err != nil || fill != "#e05d44" || color != "red" {
		t.Errorf("verdictBand(failed) = %q %q %v, want #e05d44 red nil", fill, color, err)
	}
	if _, _, err := verdictBand("passing"); err == nil {
		t.Error("verdictBand(passing) err = nil, want error")
	}
}

func TestCIBadgeSVGBytes(t *testing.T) {
	svg, err := ciBadgeSVG(config.VerdictPassed)
	if err != nil {
		t.Fatalf("ciBadgeSVG(passed) err = %v", err)
	}
	if svg != ciPassedSVG {
		t.Errorf("ciBadgeSVG(passed) = %q, want %q", svg, ciPassedSVG)
	}
	svg, err = ciBadgeSVG(config.VerdictFailed)
	if err != nil {
		t.Fatalf("ciBadgeSVG(failed) err = %v", err)
	}
	if svg != ciFailedSVG {
		t.Errorf("ciBadgeSVG(failed) = %q, want %q", svg, ciFailedSVG)
	}
	if _, err := ciBadgeSVG("passing"); err == nil {
		t.Error("ciBadgeSVG(passing) err = nil, want error")
	}
}

func TestWriteBadgesOutput(t *testing.T) {
	dir := t.TempDir()
	if err := writeBadges(mustParse(t, badgeFixture), dir, "abc123def", "9876543210", config.VerdictPassed, pinnedNow); err != nil {
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
	assertFile(t, filepath.Join(dir, "ci.svg"), ciPassedSVG)
	assertFile(t, filepath.Join(dir, "source.json"),
		`{"schemaVersion":1,"sha":"abc123def","runId":"9876543210","verdict":"passed","global":95.5,"core":100,"timestamp":"`+badgePinnedTime+`"}`)
}

func TestWriteBadgesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := writeBadges(mustParse(t, badgeFixture), dir, "deadbeef", "17", config.VerdictPassed, pinnedNow); err != nil {
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
	global, core := 95.5, 100.0
	want := badgeSource{SchemaVersion: 1, Sha: "deadbeef", RunId: "17", Verdict: config.VerdictPassed, Global: &global, Core: &core, Timestamp: badgePinnedTime}
	if !reflect.DeepEqual(src, want) {
		t.Errorf("source.json = %+v, want %+v", src, want)
	}
}

func TestWriteBadgesBandsEndToEnd(t *testing.T) {
	dir := t.TempDir()
	if err := writeBadges(mustParse(t, badgeWarnFixture), dir, "cafe", "9", config.VerdictFailed, pinnedNow); err != nil {
		t.Fatalf("writeBadges err = %v", err)
	}
	assertFile(t, filepath.Join(dir, "coverage-global.svg"),
		`<svg xmlns="http://www.w3.org/2000/svg" width="164" height="20" role="img" aria-label="global coverage: 92.0%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="164" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="164" height="20" fill="#555"/><rect x="117" width="47" height="20" fill="#dfb317"/><rect width="164" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="58" y="14">global coverage</text><text x="140" y="14">92.0%</text></g></svg>`)
	assertFile(t, filepath.Join(dir, "coverage-core.svg"),
		`<svg xmlns="http://www.w3.org/2000/svg" width="143" height="20" role="img" aria-label="core coverage: 0.0%"><linearGradient id="g" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="143" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="143" height="20" fill="#555"/><rect x="103" width="40" height="20" fill="#e05d44"/><rect width="143" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="51" y="14">core coverage</text><text x="123" y="14">0.0%</text></g></svg>`)
	assertFile(t, filepath.Join(dir, "ci.svg"), ciFailedSVG)
}

func TestWriteBadgesEmptyProfile(t *testing.T) {
	err := writeBadges(mustParse(t, "mode: set\n"), t.TempDir(), "s", "1", config.VerdictPassed, pinnedNow)
	if err == nil || !strings.Contains(err.Error(), "empty profile") {
		t.Errorf("writeBadges(empty) err = %v, want empty profile failure", err)
	}
}

func TestWriteBadgesCreatesOutDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "badges")
	if err := writeBadges(mustParse(t, badgeFixture), dir, "s", "1", config.VerdictPassed, pinnedNow); err != nil {
		t.Fatalf("writeBadges(fresh dir) err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "source.json")); err != nil {
		t.Errorf("source.json missing in fresh dir: %v", err)
	}
}

func TestWriteBadgesRejectsUnknownVerdict(t *testing.T) {
	dir := t.TempDir()
	err := writeBadges(mustParse(t, badgeFixture), dir, "s", "1", "passing", pinnedNow)
	if err == nil || !strings.Contains(err.Error(), "verdict") {
		t.Errorf("writeBadges(unknown verdict) err = %v, want verdict failure", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir err = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("writeBadges(unknown verdict) wrote %d files, want 0", len(entries))
	}
}

func TestWriteVerdictOutput(t *testing.T) {
	dir := t.TempDir()
	if err := writeVerdict(dir, "abc123def", "9876543210", config.VerdictFailed, pinnedNow); err != nil {
		t.Fatalf("writeVerdict err = %v", err)
	}
	assertFile(t, filepath.Join(dir, "ci.svg"), ciFailedSVG)
	assertFile(t, filepath.Join(dir, "source.json"),
		`{"schemaVersion":1,"sha":"abc123def","runId":"9876543210","verdict":"failed","timestamp":"`+badgePinnedTime+`"}`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir err = %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("writeVerdict wrote %d files, want 2", len(entries))
	}
}

func TestWriteVerdictRejectsUnknownVerdict(t *testing.T) {
	dir := t.TempDir()
	err := writeVerdict(dir, "s", "1", "passing", pinnedNow)
	if err == nil || !strings.Contains(err.Error(), "verdict") {
		t.Errorf("writeVerdict(unknown verdict) err = %v, want verdict failure", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir err = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("writeVerdict(unknown verdict) wrote %d files, want 0", len(entries))
	}
}

func TestRunCLIBadges(t *testing.T) {
	if code := runCLI([]string{"-badges"}); code != 2 {
		t.Errorf("runCLI(-badges, no args) = %d, want 2", code)
	}
	if code := runCLI([]string{"-badges", "a", "b", "c", "d"}); code != 2 {
		t.Errorf("runCLI(-badges, short args) = %d, want 2", code)
	}
	if code := runCLI([]string{"-badges", filepath.Join(t.TempDir(), "missing.out"), t.TempDir(), "s", "1", config.VerdictPassed}); code != 1 {
		t.Errorf("runCLI(-badges, missing) = %d, want 1", code)
	}
	dir := t.TempDir()
	if code := runCLI([]string{"-badges", writeProfile(t, badgeFixture), dir, "abc", "42", config.VerdictPassed}); code != 0 {
		t.Errorf("runCLI(-badges, pass) = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "ci.svg")); err != nil {
		t.Errorf("ci.svg missing after runCLI: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "source.json")); err != nil {
		t.Errorf("source.json missing after runCLI: %v", err)
	}
	if code := runCLI([]string{"-badges", writeProfile(t, badgeFixture), t.TempDir(), "abc", "42", "passing"}); code != 1 {
		t.Errorf("runCLI(-badges, bad verdict) = %d, want 1", code)
	}
}

func TestRunCLIVerdict(t *testing.T) {
	if code := runCLI([]string{"-verdict"}); code != 2 {
		t.Errorf("runCLI(-verdict, no args) = %d, want 2", code)
	}
	if code := runCLI([]string{"-verdict", "failed", "b", "c"}); code != 2 {
		t.Errorf("runCLI(-verdict, short args) = %d, want 2", code)
	}
	dir := t.TempDir()
	if code := runCLI([]string{"-verdict", "passing", dir, "s", "1"}); code != 1 {
		t.Errorf("runCLI(-verdict, bad verdict) = %d, want 1", code)
	}
	if code := runCLI([]string{"-verdict", config.VerdictFailed, dir, "abc", "42"}); code != 0 {
		t.Errorf("runCLI(-verdict, failed) = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "ci.svg")); err != nil {
		t.Errorf("ci.svg missing after runCLI -verdict: %v", err)
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
