package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

type stubSearcher struct {
	next func(b *rules.Board) rules.Move
}

func (s stubSearcher) Search(b *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	return s.next(b), engine.SearchStats{}, ""
}

func (s stubSearcher) StartPonder(*rules.Board) {}

func (s stubSearcher) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (s stubSearcher) Close() {}

func winLadderSearcher() server.Searcher {
	return stubSearcher{next: func(b *rules.Board) rules.Move {
		if b.Side == rules.Red {
			for _, cell := range []rules.Cell{122, 120, 121, 123} {
				if !b.Occupied(cell) {
					return rules.Move(cell)
				}
			}
		}
		for cell := range config.BoardCells {
			c := rules.Cell(cell)
			if !b.Occupied(c) {
				return rules.Move(c)
			}
		}
		return 0
	}}
}

func writeGamesFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

const gamesFixtureHeader = "F games v1 tier=master budget-ms=250 jitter=0.5 games=3 seed=7\n"

func cells(t *testing.T, names ...string) []int {
	t.Helper()
	out := make([]int, 0, len(names))
	for _, n := range names {
		c, err := rules.ParseCell(n)
		if err != nil {
			t.Fatalf("cell %s: %v", n, err)
		}
		out = append(out, int(c))
	}
	return out
}

func gamesFixtureBlock(t *testing.T, idx int, outcome string, cellNames ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("G " + strconv.Itoa(idx) + " " + outcome + "\n")
	for _, c := range cells(t, cellNames...) {
		b.WriteString("M " + strconv.Itoa(c) + "\n")
	}
	b.WriteString("E " + strconv.Itoa(len(cellNames)) + " 0\n")
	return b.String()
}

func redWinNames() []string {
	return []string{"H8", "A1", "H9", "B1", "H10", "C1", "H11", "D1", "H12"}
}

func TestParseGamesFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.txt")
	body := gamesFixtureHeader +
		gamesFixtureBlock(t, 0, "red", redWinNames()...) +
		gamesFixtureBlock(t, 1, "draw", "H8", "A1")
	writeGamesFixture(t, path, body)
	gf, err := parseGamesFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if gf.tier != "master" || gf.budgetMs != 250 || gf.jitter != 0.5 || gf.games != 3 || gf.seed != 7 {
		t.Fatalf("header fields = tier %s budget %d jitter %v games %d seed %d", gf.tier, gf.budgetMs, gf.jitter, gf.games, gf.seed)
	}
	if gf.torn {
		t.Fatal("complete file flagged torn")
	}
	if len(gf.blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(gf.blocks))
	}
	if gf.blocks[0].outcome != outcomeRedWin || gf.blocks[1].outcome != outcomeDraw {
		t.Fatalf("outcomes = %v %v", gf.blocks[0].outcome, gf.blocks[1].outcome)
	}
	if gf.blocks[0].cells[0] != int(config.BookOpeningCells[0]) {
		t.Fatalf("first cell = %d, want opening cell %d", gf.blocks[0].cells[0], config.BookOpeningCells[0])
	}
}

func TestParseGamesFileRejectsMalformed(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{"torn block", gamesFixtureHeader + "G 0 red\nM 119\n", "", false},
		{"bad outcome", gamesFixtureHeader + "G 0 purple\nM 119\nE 1 0\n", "outcome", true},
		{"index gap", gamesFixtureHeader + "G 1 red\nM 119\nE 1 0\n", "sequence", true},
		{"count mismatch", gamesFixtureHeader + "G 0 red\nM 119\nE 2 0\n", "count", true},
		{"missing header", gamesFixtureBlock(t, 0, "red", "H8"), "header", true},
		{"cell out of board", gamesFixtureHeader + "G 0 red\nM 9999\nE 1 0\n", "cell", true},
		{"record outside block", gamesFixtureHeader + "M 119\n", "block", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "games.txt")
			writeGamesFixture(t, path, tc.body)
			gf, err := parseGamesFile(path)
			if !tc.wantErr {
				if err != nil || !gf.torn {
					t.Fatalf("want torn tail, got err=%v torn=%v", err, gf.torn)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestAppendGenGameResumesFromTornTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.txt")
	body := gamesFixtureHeader + gamesFixtureBlock(t, 0, "red", "H8", "A1")
	torn := "G 1 draw\nM 118\n"
	writeGamesFixture(t, path, body+torn)
	gf, err := parseGamesFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !gf.torn || len(gf.blocks) != 1 {
		t.Fatalf("torn=%v blocks=%d, want torn with 1 block", gf.torn, len(gf.blocks))
	}
	if gf.fp != genFingerprint("master", 250, 0.5, 3, 7) {
		t.Fatalf("fingerprint %q", gf.fp)
	}
	blk := genGame{idx: 1, outcome: outcomeDraw, cells: []int{119, 118}}
	clean, err := appendGenGame(path, gf.clean, blk)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	gf2, err := parseGamesFile(path)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if gf2.torn || len(gf2.blocks) != 2 {
		t.Fatalf("torn=%v blocks=%d, want clean with 2 blocks", gf2.torn, len(gf2.blocks))
	}
	if gf2.blocks[1].cells[1] != 118 || gf2.clean != clean {
		t.Fatalf("appended block = %+v clean=%d want=%d", gf2.blocks[1], gf2.clean, clean)
	}
}

func TestRunGenerateFingerprintMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.txt")
	writeGamesFixture(t, path, gamesFixtureHeader+gamesFixtureBlock(t, 0, "red", "H8"))
	o := genOpts{Path: path, Tier: "master", Budget: 250 * time.Millisecond, Jitter: 0.9, Games: 3, Seed: 7}
	err := runGenerate(o, func(int, genGame) {})
	if err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("err = %v, want fingerprint mismatch", err)
	}
}

func TestRunGenerateRewritesBlocklessHeader(t *testing.T) {
	stub := func() ([2]server.Searcher, error) {
		return [2]server.Searcher{winLadderSearcher(), winLadderSearcher()}, nil
	}
	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "games.txt")
		o := genOpts{Path: path, Tier: "master", Budget: time.Millisecond, Jitter: 0, Games: 1, Seed: 7, Seats: stub}
		writeGamesFixture(t, path, "")
		if err := runGenerate(o, func(int, genGame) {}); err != nil {
			t.Fatalf("run over an empty file: %v", err)
		}
		gf, err := parseGamesFile(path)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(gf.blocks) != 1 {
			t.Fatalf("blocks = %d, want 1 after the header rewrite", len(gf.blocks))
		}
	})
	t.Run("malformed header line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "games.txt")
		o := genOpts{Path: path, Tier: "master", Budget: time.Millisecond, Jitter: 0, Games: 1, Seed: 7, Seats: stub}
		writeGamesFixture(t, path, "F games v1 tier=master\n")
		if err := runGenerate(o, func(int, genGame) {}); err != nil {
			t.Fatalf("run over a malformed header: %v", err)
		}
		gf, err := parseGamesFile(path)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(gf.blocks) != 1 || gf.fp != genFingerprint("master", 1, 0, 1, 7) {
			t.Fatalf("blocks=%d fp=%q, want the rewritten fingerprint and 1 block", len(gf.blocks), gf.fp)
		}
	})
	t.Run("corrupt content after a valid header still errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "games.txt")
		o := genOpts{Path: path, Tier: "master", Budget: time.Millisecond, Jitter: 0, Games: 1, Seed: 7, Seats: stub}
		writeGamesFixture(t, path, genFingerprint("master", 1, 0, 1, 7)+"\nX junk\n")
		err := runGenerate(o, func(int, genGame) {})
		if err == nil || !strings.Contains(err.Error(), "unknown line kind") {
			t.Fatalf("err = %v, want the corrupt-content error to stand behind a valid header", err)
		}
	})
}

func TestRunGenerateSkipsCompleteGames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.txt")
	header := "F games v1 tier=master budget-ms=250 jitter=0.5 games=2 seed=7\n"
	writeGamesFixture(t, path, header+gamesFixtureBlock(t, 0, "red", "H8", "A1")+gamesFixtureBlock(t, 1, "draw", "H8", "A1"))
	o := genOpts{Path: path, Tier: "master", Budget: 250 * time.Millisecond, Jitter: 0.5, Games: 2, Seed: 7}
	played := 0
	if err := runGenerate(o, func(int, genGame) { played++ }); err != nil {
		t.Fatalf("run: %v", err)
	}
	if played != 0 {
		t.Fatalf("played %d games over a complete file", played)
	}
}

func TestRunGeneratePlaysUntilTargetWithStubSeats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.txt")
	o := genOpts{Path: path, Tier: "master", Budget: time.Millisecond, Jitter: 0, Games: 2, Seed: 7, Seats: func() ([2]server.Searcher, error) {
		return [2]server.Searcher{winLadderSearcher(), winLadderSearcher()}, nil
	}}
	played := 0
	if err := runGenerate(o, func(int, genGame) { played++ }); err != nil {
		t.Fatalf("run: %v", err)
	}
	if played != 2 {
		t.Fatalf("played %d games, want 2", played)
	}
	gf, err := parseGamesFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(gf.blocks) != 2 || gf.blocks[0].outcome != outcomeRedWin {
		t.Fatalf("blocks=%d outcome0=%v, want 2 blocks with red win first", len(gf.blocks), gf.blocks[0].outcome)
	}
}

func TestPlayGenGameRecordsMovesAndOutcome(t *testing.T) {
	res := playGenGame([2]server.Searcher{winLadderSearcher(), winLadderSearcher()}, 0, 0, time.Millisecond, 0, 7, io.Discard)
	if res.outcome != outcomeRedWin {
		t.Fatalf("outcome = %v, want red win from the ladder", res.outcome)
	}
	if len(res.cells) != 9 {
		t.Fatalf("cells = %v, want the 9-move red ladder H8 through L8", res.cells)
	}
	if res.cells[0] != int(config.BookOpeningCells[0]) {
		t.Fatalf("first move %d, want opening cell %d", res.cells[0], config.BookOpeningCells[0])
	}
	if res.cells[2] != 122 {
		t.Fatalf("red second move %d, want 122 at Chebyshev 3 from the opening", res.cells[2])
	}
	if res.illegal != 0 {
		t.Fatalf("illegal = %d, want 0", res.illegal)
	}
}

func TestPlayGenGameIllegalFallbackCounts(t *testing.T) {
	bad := stubSearcher{next: func(*rules.Board) rules.Move { return rules.Move(9999) }}
	res := playGenGame([2]server.Searcher{bad, bad}, 0, 1, time.Millisecond, 0, 7, io.Discard)
	if res.outcome != outcomeBlueWin {
		t.Fatalf("outcome = %v, want blue win, the ascending fallback fill hands blue the even cells and the A1 to A5 column", res.outcome)
	}
	if len(res.cells) != 66 {
		t.Fatalf("cells = %d, want 66 moves ending on blue's A5", len(res.cells))
	}
	if res.illegal != 65 {
		t.Fatalf("illegal = %d, want 65 fallen back engine moves", res.illegal)
	}
}

func TestGenBudgetsDeterministicAndVaried(t *testing.T) {
	base := 200 * time.Millisecond
	a := genBudgets(base, 0.5, 3, 0, 8)
	b := genBudgets(base, 0.5, 3, 0, 8)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("budgets must repeat for a fixed seed and index")
		}
	}
	c := genBudgets(base, 0.5, 3, 1, 8)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
		}
	}
	if same {
		t.Fatal("different game indices must see different budget sequences")
	}
	for _, d := range a {
		if d < 100*time.Millisecond || d > 300*time.Millisecond {
			t.Fatalf("budget %v outside the jitter band", d)
		}
	}
}

func TestLoadGamesFileTierGate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.txt")
	writeGamesFixture(t, path, gamesFixtureHeader+gamesFixtureBlock(t, 0, "red", redWinNames()...))
	gf, ok, err := loadGamesFile(path, map[string]bool{"grandmaster": true})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if ok || gf != nil {
		t.Fatalf("master-tier file must be skipped under a grandmaster-only filter, got ok=%v", ok)
	}
	gf, ok, err = loadGamesFile(path, map[string]bool{"master": true})
	if err != nil || !ok || gf == nil {
		t.Fatalf("master-tier file must pass a master filter, got ok=%v err=%v", ok, err)
	}
	if len(gf.blocks) != 1 || gf.blocks[0].outcome != outcomeRedWin {
		t.Fatalf("blocks = %+v", gf.blocks)
	}
}

func TestGamesFileBlocksReconstructToSamples(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.txt")
	writeGamesFixture(t, path, gamesFixtureHeader+gamesFixtureBlock(t, 0, "red", redWinNames()...))
	gf, ok, err := loadGamesFile(path, nil)
	if err != nil || !ok || gf == nil {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	blk := gamesToParsed(gf)[0]
	ss, err := reconstructGame(blk, genSeriesKey(path, 0))
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	if len(ss) != len(redWinNames()) {
		t.Fatalf("samples = %d, want %d", len(ss), len(redWinNames()))
	}
	for i, s := range ss {
		wantY := 1.0
		if i%2 == 1 {
			wantY = 0.0
		}
		if s.Y != wantY {
			t.Fatalf("sample %d y = %v, want %v", i, s.Y, wantY)
		}
	}
	if ss[0].Series != genSeriesKey(path, 0) {
		t.Fatalf("series = %q", ss[0].Series)
	}
}
