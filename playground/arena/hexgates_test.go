package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestParseFloorHeaderGeometry(t *testing.T) {
	cases := []struct {
		line       string
		name, seat string
		geometry   string
		games      int
		ok         bool
	}{
		{"floor f tc 1 seat master games 20", "f", "master", "square", 20, true},
		{"floor f tc 2 seat hard games 5 geometry hex", "f", "hard", "hex", 5, true},
		{"floor f tc 1 seat master games 20 geometry hex", "f", "master", "hex", 20, true},
		{"floor f tc 1 seat master games", "", "", "", 0, false},
		{"floor f tc 1 seat master games 20 extra", "", "", "", 0, false},
		{"floor f tc 1 seat master games 20 geometry", "", "", "", 0, false},
	}
	for _, c := range cases {
		name, seat, geometry, games, ok := parseFloorHeader(c.line)
		if ok != c.ok || name != c.name || seat != c.seat || geometry != c.geometry || games != c.games {
			t.Fatalf("parseFloorHeader(%q) = %q %q %q %d %v, want %q %q %q %d %v", c.line, name, seat, geometry, games, ok, c.name, c.seat, c.geometry, c.games, c.ok)
		}
	}
}

func TestParsePairLogHeaderGeometry(t *testing.T) {
	a, b, geometry, tc, games, ok := parsePairLogHeader("pairlog v1 seats a,b tc 1 games 4")
	if !ok || a != "a" || b != "b" || geometry != "square" || tc != 1 || games != 4 {
		t.Fatalf("square header parsed %+v", ok)
	}
	a, b, geometry, tc, games, ok = parsePairLogHeader("pairlog v1 seats a,b tc 2 games 8 geometry hex")
	if !ok || a != "a" || b != "b" || geometry != "hex" || tc != 2 || games != 8 {
		t.Fatalf("hex header parsed %q %q %q %d %d %v", a, b, geometry, tc, games, ok)
	}
	if _, _, _, _, _, ok = parsePairLogHeader("pairlog v1 seats a,b tc 1 games 4 extra"); ok {
		t.Fatal("six-field header accepted")
	}
	if _, _, _, _, _, ok = parsePairLogHeader("pairlog v1 seats a,b tc 1 games 4 geometry"); ok {
		t.Fatal("dangling geometry accepted")
	}
}

func TestPairLogGeometryMismatchRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.pairlog")
	pl, err := openPairLogForAppend(path, "a", "b", 1, 2, "square")
	if err != nil {
		t.Fatalf("square append: %v", err)
	}
	if err := pl.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPairLog(path, "a", "b", 1, 2, "hex"); err == nil {
		t.Fatal("hex invocation accepted a square pairlog")
	}
	hexPath := filepath.Join(dir, "h.pairlog")
	hpl, err := openPairLogForAppend(hexPath, "a", "b", 1, 2, "hex")
	if err != nil {
		t.Fatalf("hex append: %v", err)
	}
	if err := hpl.close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(hexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(data)), "geometry hex") {
		t.Fatalf("hex pairlog header %q lacks geometry", string(data))
	}
	if _, err := loadPairLog(hexPath, "a", "b", 1, 2, "square"); err == nil {
		t.Fatal("square invocation accepted a hex pairlog")
	}
}

func TestPassThreshold(t *testing.T) {
	cases := map[string]float64{"easy": 0.90, "medium": 0.75, "hard": 0.60}
	for opp, want := range cases {
		got, err := passThreshold(opp)
		if err != nil || got != want {
			t.Fatalf("passThreshold(%q) = %v, %v, want %v", opp, got, err, want)
		}
	}
	if _, err := passThreshold("grandmaster"); err == nil {
		t.Fatal("grandmaster accepted as passgate opponent")
	}
}

func TestPassVerdictLine(t *testing.T) {
	if got := passVerdictLine(1.0, 0.90, "easy", 1); got != "hex-passgates: DONE" {
		t.Fatalf("pass verdict %q", got)
	}
	if got := passVerdictLine(0.8995, 0.90, "easy", 2); got != "hex-passgates: FAILED opp=easy tc=2 wd=0.899 threshold=0.900" {
		t.Fatalf("fail verdict %q", got)
	}
	if got := passVerdictLine(0.649, 0.60, "hard", 1); got != "hex-passgates: DONE" {
		t.Fatalf("boundary verdict %q", got)
	}
}

func TestLadderVerdictLines(t *testing.T) {
	inv, line := ladderEdgeVerdict("easy", "master", 1, 0.5, 40)
	if !inv || line != "hex-ladder: FAILED edge=easy-vs-master tc=1 wd=0.500 n=40" {
		t.Fatalf("inversion verdict %q inv=%v", line, inv)
	}
	inv, line = ladderEdgeVerdict("easy", "master", 1, 0.5001, 40)
	if inv || line != "" {
		t.Fatalf("hold verdict %q inv=%v", line, inv)
	}
	if got := ladderDoneLine(40); got != "hex-ladder: DONE zero inversions at 40 games per edge, deliberately underpowered" {
		t.Fatalf("done verdict %q", got)
	}
}

func TestGateWinner(t *testing.T) {
	if got := gateWinner(rules.Red, false); got != winnerRed {
		t.Fatalf("red winner %q", got)
	}
	if got := gateWinner(rules.Blue, false); got != winnerBlue {
		t.Fatalf("blue winner %q", got)
	}
	if got := gateWinner(rules.Red, true); got != winnerDraw {
		t.Fatalf("draw winner %q", got)
	}
}

func TestResolveGeometryRejectsUnknown(t *testing.T) {
	if _, err := resolveGeometry("circle"); err == nil {
		t.Fatal("unknown geometry accepted")
	}
	env, err := resolveGeometry("square")
	if err != nil || env.variant != "square" {
		t.Fatalf("square resolve %v %v", env, err)
	}
}
