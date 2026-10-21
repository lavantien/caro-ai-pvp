package main

import (
	"bytes"
	"os"

	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func baseFloorCfg() floorConfig {
	return floorConfig{
		name: "floor-master21", seat: "master-nobook", tcIdx: 1,
		gamesPerDir: 20, dirs: 5, blockDirs: 5, maxBlocks: 3, z: 1.96,
	}
}

func TestValidateFloorConfig(t *testing.T) {
	ok := baseFloorCfg()
	if err := validateFloorConfig(ok, len(config.TimeControls)); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cases := []struct {
		mutate func(*floorConfig)
		want   string
	}{
		{func(c *floorConfig) { c.name = "" }, "name"},
		{func(c *floorConfig) { c.seat = "no-such-seat" }, "seat"},
		{func(c *floorConfig) { c.tcIdx = -1 }, "tc"},
		{func(c *floorConfig) { c.tcIdx = len(config.TimeControls) }, "tc"},
		{func(c *floorConfig) { c.gamesPerDir = 0 }, "games"},
		{func(c *floorConfig) { c.dirs = -1 }, "dirs"},
		{func(c *floorConfig) { c.blockDirs = 0 }, "blockDirs"},
		{func(c *floorConfig) { c.maxBlocks = 0 }, "maxBlocks"},
		{func(c *floorConfig) { c.z = 0 }, "z"},
	}
	for _, c := range cases {
		cfg := baseFloorCfg()
		c.mutate(&cfg)
		err := validateFloorConfig(cfg, len(config.TimeControls))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("validate after %s mutation = %v, want error naming %s", c.want, err, c.want)
		}
	}
}

func TestWaldIntervalFromDecisiveCounts(t *testing.T) {
	frac, lower, upper, ok := waldInterval(11, 18, 1.96)
	if !ok {
		t.Fatal("wald 11 of 18: want ok")
	}
	if !nearly(frac, 11.0/18.0) || !nearly(lower, 0.3858985153) || !nearly(upper, 0.8363237069) {
		t.Errorf("wald 11 of 18 = %v %v %v, want frac 0.611111 lower 0.385899 upper 0.836324", frac, lower, upper)
	}
	if _, _, _, ok := waldInterval(0, 0, 1.96); ok {
		t.Error("wald with 0 decisive: want not ok")
	}
	f, lo, hi, ok := waldInterval(0, 20, 1.96)
	if !ok || f != 0 || lo != 0 || hi != 0 {
		t.Errorf("wald clean sweep = %v %v %v %v, want ok 0 0 0", ok, f, lo, hi)
	}
}

func TestWaldCoverageBoundaryIsUnrounded(t *testing.T) {
	f530, lo530, _, ok := waldInterval(530, 1000, 1.96)
	if !ok || lo530 >= 0.5 {
		t.Errorf("530 of 1000 lower = %v, want below 0.5 so the pool fails", lo530)
	}
	f531, lo531, _, ok := waldInterval(531, 1000, 1.96)
	if !ok || lo531 < 0.5 {
		t.Errorf("531 of 1000 lower = %v, want at or above 0.5 so the pool passes", lo531)
	}
	if !nearly(f530, 0.53) || !nearly(f531, 0.531) {
		t.Errorf("fracs = %v %v, want 0.53 and 0.531", f530, f531)
	}
}

func writeDir(t *testing.T, dir, name string, headerGames int, results []floorGameResult) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("floor " + name + " tc 1 seat master-nobook games " + strconv.Itoa(headerGames) + "\n")
	for i, r := range results {
		b.WriteString(floorGameLine(name, i+1, r) + "\n")
	}
	b.WriteString("summary d1: " + strconv.Itoa(len(results)) + " games\n")
	path := filepath.Join(dir, "d1.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseFloorDirCountsAndFlagsGaps(t *testing.T) {
	cfg := baseFloorCfg()
	results := []floorGameResult{
		{redWon: true, moves: 30}, {redWon: false, drawn: true, moves: 60}, {redWon: false, moves: 44},
	}
	path := writeDir(t, t.TempDir(), "floor-master21", 20, results)
	tally, err := parseFloorDir(path, cfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tally.games != 3 || tally.redWins != 1 || tally.blueWins != 1 || tally.draws != 1 {
		t.Errorf("tally = %+v, want 3 games 1 red 1 blue 1 draw", tally)
	}
	if tally.unfinished != 17 || tally.badLines != 0 {
		t.Errorf("integrity = unfinished %d bad %d, want 17 unfinished from a 20-game header", tally.unfinished, tally.badLines)
	}
}

func TestParseFloorDirRejectsForeignHeader(t *testing.T) {
	cfg := baseFloorCfg()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "d1.txt"), []byte("floor other-pool tc 1 seat master-nobook games 1\ngame 1: master-nobook (red) beat master-nobook (blue) at 30\n"), 0o644)
	if _, err := parseFloorDir(filepath.Join(dir, "d1.txt"), cfg); err == nil {
		t.Error("foreign pool name: want refusal")
	}
}

func TestAggregateFloorDirsMathAndBlocks(t *testing.T) {
	cfg := baseFloorCfg()
	root := t.TempDir()
	pool := filepath.Join(root, "floor-master21")
	perDir := []floorGameResult{
		{redWon: true, moves: 30}, {redWon: true, moves: 31}, {redWon: false, drawn: true, moves: 60},
		{redWon: false, moves: 44}, {redWon: false, moves: 45},
	}
	for d := 1; d <= 5; d++ {
		path := filepath.Join(pool, "d"+strconv.Itoa(d)+".txt")
		os.MkdirAll(pool, 0o755)
		var b strings.Builder
		b.WriteString("floor floor-master21 tc 1 seat master-nobook games 5\n")
		for i, r := range perDir {
			b.WriteString(floorGameLine("master-nobook", i+1, r) + "\n")
		}
		b.WriteString("summary d" + strconv.Itoa(d) + ": 5 games\n")
		os.WriteFile(path, []byte(b.String()), 0o644)
	}
	agg := aggregateFloorDirs(pool, cfg)
	if agg.dirs != 5 || agg.games != 25 || agg.redWins != 10 || agg.blueWins != 10 || agg.draws != 5 {
		t.Fatalf("aggregate = %+v, want 5 dirs 25 games 10 red 10 blue 5 draws", agg)
	}
	if !agg.coversHalf() {
		t.Error("frac 0.5 pool: want covers true")
	}
	if agg.extension() != extensionNone {
		t.Errorf("covered pool extension = %q, want none", agg.extension())
	}
}

func TestAggregateFloorDirsExtendRules(t *testing.T) {
	build := func(nDirs int) floorAgg {
		cfg := baseFloorCfg()
		cfg.gamesPerDir = 1
		pool := t.TempDir()
		os.MkdirAll(pool, 0o755)
		for d := 1; d <= nDirs; d++ {
			path := filepath.Join(pool, "d"+strconv.Itoa(d)+".txt")
			os.WriteFile(path, []byte("floor floor-master21 tc 1 seat master-nobook games 1\ngame 1: master-nobook (red) beat master-nobook (blue) at 30\nsummary d1: 1 games\n"), 0o644)
		}
		return aggregateFloorDirs(pool, cfg)
	}
	cases := []struct {
		dirs int
		want string
	}{
		{1, extensionMidBlock},
		{3, extensionMidBlock},
		{5, extensionNeeded},
		{6, extensionMidBlock},
		{10, extensionNeeded},
		{11, extensionMidBlock},
		{15, extensionCap},
	}
	for _, c := range cases {
		agg := build(c.dirs)
		if agg.games != c.dirs || agg.decisive() != c.dirs {
			t.Fatalf("dirs %d aggregate = %+v", c.dirs, agg)
		}
		if got := agg.extension(); got != c.want {
			t.Errorf("dirs %d (games %d): extension = %q, want %q", c.dirs, agg.games, got, c.want)
		}
	}
}

func TestFloorGameLineRoundTrip(t *testing.T) {
	results := []floorGameResult{
		{redWon: true, moves: 30},
		{redWon: false, moves: 44},
		{redWon: false, drawn: true, moves: 61},
	}
	for i, r := range results {
		line := floorGameLine("master-nobook", i+1, r)
		parsed, ok := parseFloorGameLine(line)
		if !ok || parsed != r {
			t.Errorf("line %q round trip = %+v ok %v", line, parsed, ok)
		}
	}
}

func TestRunFloorPoolAppendsAndAlternates(t *testing.T) {
	cfg := baseFloorCfg()
	cfg.gamesPerDir = 4
	cfg.dirs = 2
	pool := filepath.Join(t.TempDir(), "floor-master21")
	os.MkdirAll(pool, 0o755)
	os.WriteFile(filepath.Join(pool, "d1.txt"), []byte("floor floor-master21 tc 1 seat master-nobook games 4\n"), 0o644)
	var redFirst []bool
	err := runFloorPool(cfg, pool, func(game int, redFirstSeat bool) (floorGameResult, error) {
		redFirst = append(redFirst, redFirstSeat)
		return floorGameResult{redWon: redFirstSeat, moves: 20 + game}, nil
	})
	if err != nil {
		t.Fatalf("runFloorPool: %v", err)
	}
	wantRedFirst := []bool{true, false, true, false, true, false, true, false}
	if len(redFirst) != len(wantRedFirst) {
		t.Fatalf("games driven = %d, want %d", len(redFirst), len(wantRedFirst))
	}
	for i, v := range wantRedFirst {
		if redFirst[i] != v {
			t.Fatalf("game %d redFirst = %v, want %v", i+1, redFirst[i], v)
		}
	}
	for _, name := range []string{"d2.txt", "d3.txt"} {
		data, err := os.ReadFile(filepath.Join(pool, name))
		if err != nil {
			t.Fatalf("dir %s missing: %v", name, err)
		}
		text := string(data)
		if !strings.HasPrefix(text, "floor floor-master21 tc 1 seat master-nobook games 4\n") {
			t.Errorf("%s header = %q", name, text)
		}
		if strings.Count(text, "game ") != 4 {
			t.Errorf("%s game lines = %d, want 4", name, strings.Count(text, "game "))
		}
		if !strings.Contains(text, "summary ") {
			t.Errorf("%s missing summary line", name)
		}
	}
}

func TestRenderFloorArtifactMatchesRegateShape(t *testing.T) {
	agg := floorAgg{
		seat: "master-nobook", dirs: 5, games: 100, redWins: 47, blueWins: 46, draws: 7,
		badLines: 0, unfinished: 0, cfg: baseFloorCfg(),
	}
	var buf bytes.Buffer
	renderFloorArtifact(&buf, agg)
	want := strings.Join([]string{
		"# floor floor-master21",
		"",
		"integrity: 5 dirs, bad lines 0, unfinished 0",
		"",
		"| seat | dirs | games | decisive | frac | 95% ci | covers 0.5 |",
		"|---|---|---|---|---|---|---|",
		"| master-nobook | 5 | 100 | 93 | 0.505 | 0.404 to 0.607 | true |",
		"",
		"extend: not needed, interval covers 0.5",
		"",
	}, "\n")
	if buf.String() != want {
		t.Errorf("artifact:\n got %q\nwant %q", buf.String(), want)
	}
}

func TestRenderFloorArtifactExtendAndCapLines(t *testing.T) {
	needed := floorAgg{seat: "master-nobook", dirs: 5, games: 5, redWins: 5, blueWins: 0, draws: 0, cfg: baseFloorCfg()}
	var a, b bytes.Buffer
	renderFloorArtifact(&a, needed)
	if !strings.Contains(a.String(), "extend: needed, run another block") {
		t.Errorf("needed artifact missing extend line:\n%s", a.String())
	}
	capped := needed
	capped.dirs = 15
	capped.games = 15
	capped.redWins = 15
	renderFloorArtifact(&b, capped)
	if !strings.Contains(b.String(), "extend: blocked, block cap reached") {
		t.Errorf("capped artifact missing cap line:\n%s", b.String())
	}
}

func nearly(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-6
}
