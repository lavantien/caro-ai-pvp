package main

import (
	"math"
	"strings"
	"testing"
)

func sweep(series int, winner, winnerTier, loser, loserTier string) string {
	return "run 1 series " + itoa(series) + "\npairing " + winner + " (" + winnerTier + ") vs " + loser + " (" + loserTier + ")\ntc 2+1 bo3\n" +
		"game 1: " + winner + " (red) beat " + loser + " (blue), 30 moves, won by 4\n" +
		"game 2: " + winner + " (blue) beat " + loser + " (red), 30 moves, won by 4\n" +
		"series " + winner + " 2-0\n"
}

func itoa(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func writeFloorDir(t *testing.T, firstWins, secondWins int) string {
	t.Helper()
	dir := t.TempDir()
	n := 1
	for i := 0; i < firstWins; i++ {
		writeSeries(t, dir, "run1_s"+itoa(n)+"_hard-1-vs-hard-2.txt", sweep(n, "hard-1", "hard", "hard-2", "hard"))
		n++
	}
	for i := 0; i < secondWins; i++ {
		writeSeries(t, dir, "run1_s"+itoa(n)+"_hard-2-vs-hard-1.txt", sweep(n, "hard-2", "hard", "hard-1", "hard"))
		n++
	}
	return dir
}

func TestGateFloorCenterAndCI(t *testing.T) {
	dir := writeFloorDir(t, 4, 4)
	a, err := loadArm("hard", dir)
	if err != nil {
		t.Fatal(err)
	}
	floors := computeFloors([]gateEntry{{key: "hard", dirs: []string{dir}}}, []*armData{a})
	fs := floors["hard"]
	if fs.games != 16 || fs.decisive != 16 || fs.wins != 8 {
		t.Fatalf("floor counts: games %d decisive %d wins %d", fs.games, fs.decisive, fs.wins)
	}
	if fs.frac != 0.5 {
		t.Errorf("floor frac: %v", fs.frac)
	}
	wantHalf := 1.96 * math.Sqrt(0.25/16)
	if math.Abs(fs.ciLo-(0.5-wantHalf)) > 1e-9 || math.Abs(fs.ciHi-(0.5+wantHalf)) > 1e-9 {
		t.Errorf("floor ci: %v to %v", fs.ciLo, fs.ciHi)
	}
	if !fs.coversHalf {
		t.Error("floor band must cover 0.5 at an even split")
	}
}

func TestGateFloorRejectsAsymmetry(t *testing.T) {
	dir := writeFloorDir(t, 7, 1)
	a, err := loadArm("hard", dir)
	if err != nil {
		t.Fatal(err)
	}
	floors := computeFloors([]gateEntry{{key: "hard", dirs: []string{dir}}}, []*armData{a})
	fs := floors["hard"]
	if fs.frac != 0.875 {
		t.Fatalf("floor frac: %v", fs.frac)
	}
	if fs.coversHalf {
		t.Error("a 14 of 16 floor must fail the 0.5 coverage check")
	}
}

func TestGateFamilyReads(t *testing.T) {
	floorDir := writeFloorDir(t, 4, 4)
	floorArm, err := loadArm("hard", floorDir)
	if err != nil {
		t.Fatal(err)
	}
	armDir := t.TempDir()
	writeSeries(t, armDir, "run1_s01_medium-1-vs-easy-1.txt", sweep(1, "medium-1", "medium", "easy-1", "easy"))
	writeSeries(t, armDir, "run1_s02_easy-1-vs-medium-1.txt", sweep(2, "medium-1", "medium", "easy-1", "easy"))
	writeSeries(t, armDir, "run1_s03_medium-1-vs-easy-1.txt", sweep(3, "medium-1", "medium", "easy-1", "easy"))
	writeSeries(t, armDir, "run1_s04_easy-1-vs-medium-1.txt", sweep(4, "medium-1", "medium", "easy-1", "easy"))
	writeSeries(t, armDir, "run1_s05_medium-1-vs-easy-1.txt", sweep(5, "medium-1", "medium", "easy-1", "easy"))
	writeSeries(t, armDir, "run1_s06_easy-1-vs-medium-1.txt", sweep(6, "medium-1", "medium", "easy-1", "easy"))
	arm, err := loadArm("rep1", armDir)
	if err != nil {
		t.Fatal(err)
	}
	floors := computeFloors([]gateEntry{{key: "hard", dirs: []string{floorDir}}}, []*armData{floorArm})
	families := computeFamilies([]*armData{arm}, floors)
	if len(families) != 1 {
		t.Fatalf("families: %d", len(families))
	}
	fs := families[0]
	if fs.lower != "easy" || fs.upper != "medium" || fs.floorTier != "hard" {
		t.Fatalf("family shape: %+v", fs)
	}
	if fs.n != 12 || fs.upperWins != 12 {
		t.Fatalf("family counts: n %d upperWins %d", fs.n, fs.upperWins)
	}
	wantSE := 0.5 * math.Sqrt(1.0/12+1.0/16)
	if math.Abs(fs.se-wantSE) > 1e-9 {
		t.Errorf("se: %v want %v", fs.se, wantSE)
	}
	if math.Abs(fs.zB-(1-0.5)/wantSE) > 1e-9 || !fs.bCert {
		t.Errorf("rule B: z %v cert %v", fs.zB, fs.bCert)
	}
	if fs.aFired {
		t.Error("rule A must stay quiet when the upper tier sweeps")
	}

	invertedDir := t.TempDir()
	writeSeries(t, invertedDir, "run1_s01_easy-1-vs-medium-1.txt", sweep(1, "easy-1", "easy", "medium-1", "medium"))
	writeSeries(t, invertedDir, "run1_s02_medium-1-vs-easy-1.txt", sweep(2, "easy-1", "easy", "medium-1", "medium"))
	writeSeries(t, invertedDir, "run1_s03_easy-1-vs-medium-1.txt", sweep(3, "easy-1", "easy", "medium-1", "medium"))
	writeSeries(t, invertedDir, "run1_s04_medium-1-vs-easy-1.txt", sweep(4, "easy-1", "easy", "medium-1", "medium"))
	writeSeries(t, invertedDir, "run1_s05_easy-1-vs-medium-1.txt", sweep(5, "easy-1", "easy", "medium-1", "medium"))
	writeSeries(t, invertedDir, "run1_s06_medium-1-vs-easy-1.txt", sweep(6, "easy-1", "easy", "medium-1", "medium"))
	inverted, err := loadArm("rep2", invertedDir)
	if err != nil {
		t.Fatal(err)
	}
	fs = computeFamilies([]*armData{inverted}, floors)[0]
	if fs.n != 12 || fs.upperWins != 0 || !fs.aFired {
		t.Errorf("inverted family: n %d upperWins %d fired %v", fs.n, fs.upperWins, fs.aFired)
	}
	if fs.bCert {
		t.Error("rule B must not certify a swept upper tier")
	}
}

func TestGateVoidOnUnfinished(t *testing.T) {
	dir := t.TempDir()
	writeSeries(t, dir, "run1_s01_hard-1-vs-hard-2.txt", "run 1 series 1\npairing hard-1 (hard) vs hard-2 (hard)\ntc 2+1 bo3\ngame 1: hard-1 (red) beat hard-2 (blue), 30 moves, won by 4\n")
	out, err := renderGate(nil, []gateEntry{{key: "hard", dirs: []string{dir}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "VOID") {
		t.Errorf("unfinished series must void the gate, got:\n%s", out)
	}
}

func TestGateRefusesMixedControls(t *testing.T) {
	a21 := writeFloorDir(t, 4, 4)
	a32 := t.TempDir()
	writeSeries(t, a32, "run1_s01_hard-1-vs-hard-2.txt", "run 1 series 1\npairing hard-1 (hard) vs hard-2 (hard)\ntc 3+2 bo3\ngame 1: hard-1 (red) beat hard-2 (blue), 30 moves, won by 4\ngame 2: hard-1 (blue) beat hard-2 (red), 30 moves, won by 4\nseries hard-1 2-0\n")
	_, err := renderGate([]gateEntry{{key: "rep1", dirs: []string{a21}}}, []gateEntry{{key: "hard", dirs: []string{a32}}})
	if err == nil || !strings.Contains(err.Error(), "mixed controls") {
		t.Fatalf("mixed controls must refuse, got %v", err)
	}
}

func TestGateRule0FailsOnStalledPonder(t *testing.T) {
	dir := t.TempDir()
	writeSeries(t, dir, "run1_s01_easy-1-vs-medium-1.txt", "run 1 series 1\npairing easy-1 (easy) vs medium-1 (medium)\ntc 2+1 bo3\n"+
		"M1, Red, H8, d=9, n=2.86m, nps=1.43m, ebf=5.1, tt=8%, hf=42%, fh1=89%, s=-200, thr=1, t=1.00, alloc=2.00, pv=H8\n"+
		"M2, Blue, H9, d=9, n=2.86m, nps=1.43m, ebf=5.1, tt=8%, hf=42%, fh1=89%, s=+200, thr=1, t=2.00, alloc=4.00, [PONDER], pv=H9\n"+
		"game 1: easy-1 (red) beat medium-1 (blue), 2 moves, won by 4\nseries easy-1 1-0\n")
	out, err := renderGate([]gateEntry{{key: "rep1", dirs: []string{dir}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| medium-1 | medium | 1 | 2.00 | 2.00 | 0.00 | FAIL |",
		"| easy-1 | easy | 0 | 0.00 | 0.00 | 0.50 | pass |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rule 0 row missing %q in:\n%s", want, out)
		}
	}
}

func TestParseGateEntries(t *testing.T) {
	f := &multiFlag{}
	if err := f.Set("hard=logs/a,logs/b"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("rep1=logs/c"); err != nil {
		t.Fatal(err)
	}
	entries, err := parseGateEntries(f, "gate-floor")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].key != "hard" || len(entries[0].dirs) != 2 || entries[0].dirs[1] != "logs/b" || entries[1].dirs[0] != "logs/c" {
		t.Errorf("entries: %+v", entries)
	}
	bad := &multiFlag{}
	if err := bad.Set("nodirs"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseGateEntries(bad, "gate-floor"); err == nil {
		t.Error("spec without = must fail")
	}
}
