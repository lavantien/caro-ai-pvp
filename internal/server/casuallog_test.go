package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const casualTestFormat = "s%d_%s-vs-%s.txt"

func newTestCasualLogs(t *testing.T) (*casualLogs, string) {
	t.Helper()
	dir := t.TempDir()
	return newCasualLogs(dir, casualTestFormat), dir
}

func casualFilePath(dir string, h casualGameHeader) string {
	return filepath.Join(dir, fmt.Sprintf(casualTestFormat,
		h.SeriesID, sanitizeLogName(h.Red), sanitizeLogName(h.Blue)))
}

func casualHeader() casualGameHeader {
	return casualGameHeader{
		SeriesID: 7, GameNo: 1, RoomID: "ab12", Mode: lobbyModeVsBot,
		Red: "alice", Blue: "easy-ab12",
		PairingRed: "alice", PairingBlue: "easy-ab12",
		TCIdx: 0, BOLen: config.SeriesBO3,
		Outcome: OutcomeRed,
	}
}

func twoLines() []GameStat {
	return []GameStat{
		{MoveNo: 2, Line: "M2, Blue, P12, d=9"},
		{MoveNo: 4, Line: "M4, Blue, P8, d=9"},
	}
}

func readCasualFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read casual log %s: %v", path, err)
	}
	return string(b)
}

func TestCasualLogWritesHeaderLinesAndSeparator(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	h := casualHeader()
	if err := c.WriteGame(h, twoLines()); err != nil {
		t.Fatalf("write: %v", err)
	}
	tc := config.TimeControls[h.TCIdx]
	want := strings.Join([]string{
		fmt.Sprintf("game %d room %s mode %s", h.GameNo, h.RoomID, h.Mode),
		fmt.Sprintf("pairing %s vs %s", h.Red, h.Blue),
		fmt.Sprintf("tc %d+%d bo%d outcome %s", tc.InitialMin, tc.IncrementSec, h.BOLen, h.Outcome),
		"M2, Blue, P12, d=9",
		"M4, Blue, P8, d=9",
		"",
		"",
	}, "\n")
	if got := readCasualFile(t, casualFilePath(dir, h)); got != want {
		t.Errorf("casual log = %q, want %q", got, want)
	}
}

func TestCasualLogSkipsGamesWithoutStatLines(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "casual")
	c := newCasualLogs(dir, casualTestFormat)
	if err := c.WriteGame(casualHeader(), nil); err != nil {
		t.Fatalf("write without stats: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dir after a stats-less game = %v, want never created", err)
	}
}

func TestCasualLogAppendsAcrossGames(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	h := casualHeader()
	if err := c.WriteGame(h, twoLines()); err != nil {
		t.Fatalf("game 1: %v", err)
	}
	h.GameNo, h.Outcome = 2, OutcomeBlue
	if err := c.WriteGame(h, twoLines()); err != nil {
		t.Fatalf("game 2: %v", err)
	}
	got := readCasualFile(t, casualFilePath(dir, h))
	if n := strings.Count(got, "game "); n != 2 {
		t.Errorf("blocks = %d, want 2: %q", n, got)
	}
	if !strings.HasPrefix(got, "game 1 room ab12 mode vs bot\n") ||
		!strings.Contains(got, "game 2 room ab12 mode vs bot\n") {
		t.Errorf("append order broken: %q", got)
	}
	if !strings.HasSuffix(got, "M4, Blue, P8, d=9\n\n") {
		t.Errorf("tail after game 2 = %q, want the blank separator", got)
	}
}

func TestCasualLogTruncatesTornTailOnReopen(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	h := casualHeader()
	if err := c.WriteGame(h, twoLines()[:1]); err != nil {
		t.Fatalf("first game: %v", err)
	}
	path := casualFilePath(dir, h)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("seed torn tail: %v", err)
	}
	if _, err := f.WriteString("M6, Blue, N16, d=1"); err != nil {
		t.Fatalf("write torn tail: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close torn tail: %v", err)
	}

	h.GameNo, h.Outcome = 2, OutcomeDraw
	if err := c.WriteGame(h, twoLines()); err != nil {
		t.Fatalf("game 2 over the torn tail: %v", err)
	}
	got := readCasualFile(t, path)
	if strings.Contains(got, "d=1") || strings.Contains(got, "M6") {
		t.Errorf("torn fragment survived: %q", got)
	}
	if n := strings.Count(got, "game "); n != 2 {
		t.Errorf("blocks = %d, want 2: %q", n, got)
	}
	if !strings.HasSuffix(got, "M4, Blue, P8, d=9\n\n") {
		t.Errorf("tail after recovery = %q, want a whole block", got)
	}
}

func TestCasualLogTruncatesAFileThatIsOnlyAFragment(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	h := casualHeader()
	path := casualFilePath(dir, h)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("M2, Blu"), 0o644); err != nil {
		t.Fatalf("seed fragment: %v", err)
	}
	if err := c.WriteGame(h, twoLines()[:1]); err != nil {
		t.Fatalf("write over the fragment: %v", err)
	}
	got := readCasualFile(t, path)
	if strings.HasPrefix(got, "M2, Blu") {
		t.Errorf("fragment survived: %q", got)
	}
	if !strings.HasPrefix(got, "game 1 room ab12 mode vs bot\n") {
		t.Errorf("recovered head = %q, want the game header", got)
	}
}

func TestCasualLogSanitizesSeatNamesIntoTheFileName(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	h := casualHeader()
	h.Red, h.Blue = "ali ce", "medium/ab12"
	if err := c.WriteGame(h, twoLines()[:1]); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "s7_ali_ce-vs-medium_ab12.txt")); err != nil {
		t.Errorf("sanitized file: %v", err)
	}
	if h.Red != "ali ce" || h.Blue != "medium/ab12" {
		t.Errorf("header mutated its inputs: %+v", h)
	}
}

func TestCasualLogSeparatesSeriesIntoSeparateFiles(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	first, second := casualHeader(), casualHeader()
	second.SeriesID, second.Blue = 8, "medium-ab12"
	if err := c.WriteGame(first, twoLines()[:1]); err != nil {
		t.Fatalf("first series: %v", err)
	}
	if err := c.WriteGame(second, twoLines()[:1]); err != nil {
		t.Fatalf("second series: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("list dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("files = %d, want one per series", len(entries))
	}
}

func TestCasualLogMkdirFailureIsAnError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	c := newCasualLogs(blocker, casualTestFormat)
	if err := c.WriteGame(casualHeader(), twoLines()); err == nil ||
		!strings.Contains(err.Error(), blocker) {
		t.Errorf("write over a blocked dir = %v, want an error naming %s", err, blocker)
	}
}

func TestCasualLogOpenFailureIsAnError(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	h := casualHeader()
	if err := os.MkdirAll(casualFilePath(dir, h), 0o755); err != nil {
		t.Fatalf("seed directory over the file path: %v", err)
	}
	if err := c.WriteGame(h, twoLines()); err == nil {
		t.Error("write onto a directory path = nil, want an error")
	}
}

func TestCasualLogConcurrentAppendsPreserveEveryBlock(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	const writers = 8
	h := casualHeader()
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g := h
			g.GameNo = i + 1
			g.Outcome = OutcomeDraw
			if err := c.WriteGame(g, twoLines()); err != nil {
				t.Errorf("writer %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	got := readCasualFile(t, casualFilePath(dir, h))
	if n := strings.Count(got, "game "); n != writers {
		t.Errorf("blocks = %d, want %d", n, writers)
	}
	if n := strings.Count(got, "M2, Blue, P12, d=9"); n != writers {
		t.Errorf("intact M2 lines = %d, want %d", n, writers)
	}
	if n := strings.Count(got, "M4, Blue, P8, d=9"); n != writers {
		t.Errorf("intact M4 lines = %d, want %d", n, writers)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("tail = %q, want the blank separator", got[len(got)-4:])
	}
}

func FuzzCasualLogWriteGame(f *testing.F) {
	f.Add("ab12", "vs bot", "alice", "easy-ab12", "red", "M2, Blue, P12, d=9")
	f.Add("", "", "..\\slash", "x/y/z", "draw", "\nM2")
	f.Add("room", "bot vs bot", strings.Repeat("n", 300), "blue", "blue", "line with spaces and , commas")
	f.Fuzz(func(t *testing.T, roomID, mode, red, blue, outcome, line string) {
		dir := filepath.Join(t.TempDir(), "casual")
		c := newCasualLogs(dir, casualTestFormat)
		h := casualGameHeader{
			SeriesID: 3, GameNo: 2, RoomID: roomID, Mode: mode,
			Red: red, Blue: blue, TCIdx: 0, BOLen: config.SeriesBO3,
			Outcome: outcome,
		}
		stats := []GameStat{{MoveNo: 2, Line: line}, {MoveNo: 4, Line: line}}
		if err := c.WriteGame(h, stats); err != nil {
			t.Fatalf("write: %v", err)
		}
		got := readCasualFile(t, casualFilePath(dir, h))
		if !strings.HasSuffix(got, "\n\n") {
			t.Errorf("tail = %q, want every block blank-separated", got[len(got)-8:])
		}
		if n := strings.Count(got, line); n < 2 {
			t.Errorf("line copies = %d, want 2", n)
		}
		if strings.Count(got, "game 2 room") != 1 || strings.Count(got, "pairing ") != 1 {
			t.Errorf("header torn by input: %q", got)
		}
		if strings.ContainsAny(filepath.Base(casualFilePath(dir, h)), `/\:`) {
			t.Errorf("file name leaks path separators: %s", filepath.Base(casualFilePath(dir, h)))
		}
		if name := filepath.Base(casualFilePath(dir, h)); len(name) > casualNameMaxRunes*2+24 {
			t.Errorf("file name length unbounded: %d", len(name))
		}
	})
}

func TestCasualLogRejectsBoundaryTimeControlIndexes(t *testing.T) {
	c, _ := newTestCasualLogs(t)
	for _, tc := range []int{-1, len(config.TimeControls)} {
		h := casualHeader()
		h.TCIdx = tc
		err := c.WriteGame(h, twoLines()[:1])
		if err == nil || !strings.Contains(err.Error(), "tc index") {
			t.Errorf("tc index %d = %v, want the rejection naming the index", tc, err)
		}
	}
}

func TestCasualLogRendersHeaderNumbersInBaseTen(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	h := casualHeader()
	h.GameNo = 10
	h.TCIdx = 3
	h.BOLen = 10
	if err := c.WriteGame(h, twoLines()[:1]); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readCasualFile(t, casualFilePath(dir, h))
	for _, want := range []string{"game 10 room ab12", "tc 10+5 bo10 outcome red"} {
		if !strings.Contains(got, want) {
			t.Errorf("header %q missing from %q", want, got)
		}
	}
}

func TestTrimTornTailKeepsWholeLinesAndDropsFragments(t *testing.T) {
	for _, row := range []struct {
		name string
		seed string
		want string
	}{
		{"a complete tail", "x\n", "x\n"},
		{"a fragment after the last line", "xx\ny", "xx\n"},
		{"a newline first in the window", "\ny", "\n"},
		{"earlier lines survive the cut", "a\nb\ncz", "a\nb\n"},
		{"an empty file", "", ""},
		{"a single byte fragment", "q", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tail.txt")
			if err := os.WriteFile(path, []byte(row.seed), 0o644); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if err := trimTornTail(path); err != nil {
				t.Fatalf("trim: %v", err)
			}
			if got := readCasualFile(t, path); got != row.want {
				t.Errorf("tail after trim = %q, want %q", got, row.want)
			}
		})
	}
}

func TestTrimTornTailCutsExactlyTheWindowWhenNoNewlineRemains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.txt")
	seed := strings.Repeat("x", 4608)
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := trimTornTail(path); err != nil {
		t.Fatalf("trim: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if want := int64(len(seed) - 4096); info.Size() != want {
		t.Errorf("size after trim = %d, want %d", info.Size(), want)
	}
}

func TestCasualLogWriteIsQuietWhenCloseSucceeds(t *testing.T) {
	c, _ := newTestCasualLogs(t)
	out := filepath.Join(t.TempDir(), "stderr.txt")
	f, err := os.Create(out)
	if err != nil {
		t.Fatalf("capture stderr: %v", err)
	}
	saved := os.Stderr
	os.Stderr = f
	writeErr := c.WriteGame(casualHeader(), twoLines())
	os.Stderr = saved
	if err := f.Close(); err != nil {
		t.Fatalf("close capture: %v", err)
	}
	if writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}
	if strings.Contains(string(b), "casual log close") {
		t.Errorf("stderr after a clean close = %q, want silence", string(b))
	}
}

func TestSanitizeLogNameBoundaryRunesAndLength(t *testing.T) {
	for _, row := range []struct {
		name string
		in   string
		want string
	}{
		{"every allowed rune", "azAZ09.-_", "azAZ09.-_"},
		{"spacing and punctuation fold", "Carol vs Bot!", "Carol_vs_Bot_"},
		{"exactly the cap", strings.Repeat("a", 64), strings.Repeat("a", 64)},
		{"over the cap", strings.Repeat("b", 66), strings.Repeat("b", 64)},
		{"wide runes count once", strings.Repeat("é", 65), strings.Repeat("_", 64)},
	} {
		if got := sanitizeLogName(row.in); got != row.want {
			t.Errorf("%s: sanitize %q = %q, want %q", row.name, row.in, got, row.want)
		}
	}
}

func TestLogCasualCompletionSkipsGamesWithoutStatLinesSilently(t *testing.T) {
	buf := &lockedBuffer{}
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}
	base := Game{
		RedUser: alice.ID, BlueUser: bob.ID, Outcome: OutcomeRed,
		Moves: []byte{1, 0}, FullTurns: 1,
	}
	twoStat, oneStat, noStats := base, base, base
	twoStat.SeriesID, twoStat.RoomID, twoStat.IdxInSeries, twoStat.StatLines = sr.ID, "ab12", 0, twoLines()
	oneStat.SeriesID, oneStat.RoomID, oneStat.IdxInSeries, oneStat.StatLines = sr.ID, "cd34", 1, twoLines()[:1]
	noStats.SeriesID = sr.ID + 424242
	s.logCasualCompletion(context.Background(), Completion{Games: []Game{twoStat, noStats, oneStat}})
	if out := buf.String(); out != "" {
		t.Errorf("log over a game without stat lines = %q, want silence", out)
	}
	body := readCasualFile(t, filepath.Join(config.CasualLogRoot,
		fmt.Sprintf("s%d_%s-vs-%s.txt", sr.ID, "alice", "bob")))
	if n := strings.Count(body, "game "); n != 2 {
		t.Errorf("blocks = %d, want the two games with stat lines: %q", n, body)
	}
	if !strings.Contains(body, "game 2 room cd34") {
		t.Errorf("single-stat game missing: %q", body)
	}
	if n := strings.Count(body, "M2, Blue, P12, d=9"); n != 2 {
		t.Errorf("M2 lines = %d, want 2: %q", n, body)
	}
}

func TestLogCasualCompletionNumbersASkippedGameByItsSeriesIndex(t *testing.T) {
	buf := &lockedBuffer{}
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	s.logCasualCompletion(context.Background(), Completion{Games: []Game{{
		SeriesID: 424242, IdxInSeries: 4, RedUser: alice.ID, BlueUser: alice.ID,
		Outcome: OutcomeRed, Moves: []byte{1, 0}, FullTurns: 1, StatLines: twoLines()[:1],
	}}})
	if out := buf.String(); !strings.Contains(out, "series 424242 game 5") {
		t.Errorf("skip log = %q, want the game numbered 5", out)
	}
}

func TestApplyCompletionWritesTheCasualTextFile(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	botID, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	sr, err := s.CreateSeries(context.Background(), 2, config.SeriesBO5, alice.ID, botID)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}
	botName := botDisplayName(config.TierEasy.Name, "0123456789abcdef0123456789abcdef")
	lines := []GameStat{
		{MoveNo: 2, Line: "M2, Blue, P12, d=9"},
		{MoveNo: 4, Line: "M4, Blue, P8, d=9"},
	}
	if err := s.ApplyCompletion(context.Background(), Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: botID,
			Outcome: OutcomeRed, Moves: []byte{1, 0}, FullTurns: 1,
			RoomID: "room42", BotName: botName, StatLines: lines,
		}},
	}); err != nil {
		t.Fatalf("apply completion: %v", err)
	}
	path := filepath.Join(config.CasualLogRoot,
		fmt.Sprintf("s%d_%s-vs-%s.txt", sr.ID, "alice", botName))
	var body string
	waitFor(t, func() bool {
		b, err := os.ReadFile(path)
		body = string(b)
		return err == nil
	})
	tc := config.TimeControls[2]
	want := strings.Join([]string{
		"game 1 room room42 mode vs bot",
		"pairing alice vs " + botName,
		fmt.Sprintf("tc %d+%d bo%d outcome red", tc.InitialMin, tc.IncrementSec, config.SeriesBO5),
		"M2, Blue, P12, d=9",
		"M4, Blue, P8, d=9",
		"",
		"",
	}, "\n")
	if body != want {
		t.Errorf("casual file = %q, want %q", body, want)
	}
	rows := gameStatRows(t, s, onlyGameID(t, s, sr.ID))
	for i, row := range rows {
		if !strings.Contains(body, row.Line+"\n") {
			t.Errorf("sqlite stat %d %q missing from the text side", i, row.Line)
		}
	}
}

func TestApplyCompletionHumanGamesWriteNoCasualFile(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}
	if err := s.ApplyCompletion(context.Background(), Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
			Outcome: OutcomeDraw, Moves: []byte{1, 0}, FullTurns: 1,
		}},
	}); err != nil {
		t.Fatalf("apply completion: %v", err)
	}
	if entries, _ := os.ReadDir(config.CasualLogRoot); len(entries) != 0 {
		t.Errorf("casual files after a pvp game = %d, want 0", len(entries))
	}
}

func TestApplyCompletionCasualLogFailureNeverFailsTheUnit(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	buf := &lockedBuffer{}
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	saved := config.CasualLogRoot
	config.CasualLogRoot = blocker
	t.Cleanup(func() { config.CasualLogRoot = saved })

	s := mustOpen(t, filepath.Join(t.TempDir(), "caro.sqlite"))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	botID, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, botID)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}

	if err := s.ApplyCompletion(context.Background(), Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: botID,
			Outcome: OutcomeRed, Moves: []byte{1, 0}, FullTurns: 1,
			BotName: "easy-x", StatLines: twoLines(),
		}},
	}); err != nil {
		t.Fatalf("apply over the blocked log dir = %v, want the sqlite side to win", err)
	}
	if rows := gameStatRows(t, s, onlyGameID(t, s, sr.ID)); len(rows) != 2 {
		t.Errorf("stat rows over the blocked log dir = %d, want 2", len(rows))
	}
	if out := buf.String(); !strings.Contains(out, "casual log") || !strings.Contains(out, blocker) {
		t.Errorf("log over the blocked dir = %q, want the loud failure naming %s", out, blocker)
	}
}

func TestForfeitAfterALiveGameKeepsOneCasualFile(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	botInfo, _ := r.Info()
	botName := botInfo.GuestBotName
	injectPerGameBot(t, r, botLoseScripts)
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	for _, name := range humanWinSides[0] {
		waitFor(t, func() bool { return humanTurn(r, alice.ID) })
		if err := r.PlayMove(alice.ID, mustCellT(t, name)); err != nil {
			t.Fatalf("game 1 human move %s: %v", name, err)
		}
	}
	waitFor(t, func() bool { return humanTurn(r, alice.ID) })
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("game 2 human move D4: %v", err)
	}
	waitFor(t, func() bool { return humanTurn(r, alice.ID) })
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit in game 2: %v", err)
	}
	drainEvents(sub)

	entries, err := os.ReadDir(config.CasualLogRoot)
	if err != nil {
		t.Fatalf("list casual dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("casual files after the forfeit = %d, want 1: %v", len(entries), entries)
	}
	want := fmt.Sprintf("s%d_alice-vs-%s.txt", r.SeriesID(), botName)
	if entries[0].Name() != want {
		t.Errorf("casual file = %s, want %s", entries[0].Name(), want)
	}
	body := readCasualFile(t, filepath.Join(config.CasualLogRoot, entries[0].Name()))
	if n := strings.Count(body, "game "); n != 2 {
		t.Errorf("blocks = %d, want the live game and the forfeited game: %q", n, body)
	}
	if n := strings.Count(body, "room "+r.ID()+" mode vs bot\n"); n != 2 {
		t.Errorf("headers carrying the room id = %d, want 2: %q", n, body)
	}
	textLines := 0
	for row := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(row, "M") {
			textLines++
		}
	}
	sqliteLines := 0
	for _, id := range gameIDs(t, s) {
		for _, row := range gameStatRows(t, s.store, id) {
			sqliteLines++
			if !strings.Contains(body, row.Line+"\n") {
				t.Errorf("sqlite stat %q missing from the text side", row.Line)
			}
		}
	}
	if textLines != sqliteLines {
		t.Errorf("M-lines text %d vs sqlite %d, want equal sets", textLines, sqliteLines)
	}
}

func TestCasualLogConcurrentInstancesOnOneRootLoseNothing(t *testing.T) {
	dir := t.TempDir()
	const instances = 4
	const perInstance = 50
	h := casualHeader()
	var wg sync.WaitGroup
	for i := range instances {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := newCasualLogs(dir, casualTestFormat)
			for g := range perInstance {
				gm := h
				gm.GameNo = i*perInstance + g + 1
				gm.Outcome = OutcomeDraw
				if err := c.WriteGame(gm, twoLines()); err != nil {
					t.Errorf("instance %d game %d: %v", i, g+1, err)
				}
			}
		}(i)
	}
	wg.Wait()
	got := readCasualFile(t, casualFilePath(dir, h))
	if n := strings.Count(got, "game "); n != instances*perInstance {
		t.Errorf("blocks = %d, want %d", n, instances*perInstance)
	}
	if n := strings.Count(got, "M2, Blue, P12, d=9"); n != instances*perInstance {
		t.Errorf("intact M2 lines = %d, want %d", n, instances*perInstance)
	}
	if n := strings.Count(got, "M4, Blue, P8, d=9"); n != instances*perInstance {
		t.Errorf("intact M4 lines = %d, want %d", n, instances*perInstance)
	}
}

func TestCasualLogKeepsEarlierGamesWhenTheTailWindowHasNoNewline(t *testing.T) {
	c, dir := newTestCasualLogs(t)
	h := casualHeader()
	if err := c.WriteGame(h, twoLines()); err != nil {
		t.Fatalf("game 1: %v", err)
	}
	path := casualFilePath(dir, h)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("seed long torn tail: %v", err)
	}
	if _, err := f.WriteString(strings.Repeat("x", casualTailScan+512)); err != nil {
		t.Fatalf("write long torn tail: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close long torn tail: %v", err)
	}
	h.GameNo, h.Outcome = 2, OutcomeDraw
	if err := c.WriteGame(h, twoLines()[:1]); err != nil {
		t.Fatalf("game 2 over the long torn tail: %v", err)
	}
	got := readCasualFile(t, path)
	if n := strings.Count(got, "game "); n != 2 {
		t.Errorf("blocks = %d, want both games: %q", n, got)
	}
	if n := strings.Count(got, "M4, Blue, P8, d=9"); n != 1 {
		t.Errorf("game 1 M4 lines = %d, want the earlier game intact: %q", n, got)
	}
}

func TestApplyCompletionSurvivesACanceledContextAfterCommit(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	botID, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, botID)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}
	botName := botDisplayName(config.TierEasy.Name, "0123456789abcdef0123456789abcdef")
	unit := func(idx int) Completion {
		return Completion{Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: idx, RedUser: alice.ID, BlueUser: botID,
			Outcome: OutcomeRed, Moves: []byte{1, 0}, FullTurns: 1,
			BotName: botName, StatLines: twoLines(),
		}}}
	}
	if err := s.ApplyCompletion(context.Background(), unit(0)); err != nil {
		t.Fatalf("game 1: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	s.logCasualCompletion(canceled, unit(1))

	path := filepath.Join(config.CasualLogRoot,
		fmt.Sprintf("s%d_%s-vs-%s.txt", sr.ID, "alice", botName))
	body := readCasualFile(t, path)
	if n := strings.Count(body, "game "); n != 2 {
		t.Errorf("blocks after a canceled post-commit context = %d, want 2: %q", n, body)
	}
}

func TestCreateSeriesRejectsAnUnconfiguredTimeControl(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	for _, tc := range []int{-1, len(config.TimeControls), 99} {
		if _, err := s.CreateSeries(context.Background(), tc, config.SeriesBO3, alice.ID, bob.ID); err == nil {
			t.Errorf("tc index %d created a series, want rejection", tc)
		}
	}
	if _, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, bob.ID); err != nil {
		t.Fatalf("tc index 0 rejected: %v", err)
	}
}

func TestApplyCompletionSkipsTheTextSideLoudlyOnACorruptTCIndex(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	botID, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, botID)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}
	bad := len(config.TimeControls) + 50
	if _, err := s.db.Exec(`UPDATE series SET tc_idx = ? WHERE id = ?`, bad, sr.ID); err != nil {
		t.Fatalf("corrupt tc index: %v", err)
	}
	buf := &lockedBuffer{}
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	if err := s.ApplyCompletion(context.Background(), Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: botID,
			Outcome: OutcomeRed, Moves: []byte{1, 0}, FullTurns: 1,
			BotName: "easy-x", StatLines: twoLines(),
		}},
	}); err != nil {
		t.Fatalf("apply over a corrupt tc index = %v, want the sqlite side to win", err)
	}
	if rows := gameStatRows(t, s, onlyGameID(t, s, sr.ID)); len(rows) != 2 {
		t.Errorf("stat rows over a corrupt tc index = %d, want 2", len(rows))
	}
	if entries, _ := os.ReadDir(config.CasualLogRoot); len(entries) != 0 {
		t.Errorf("casual files over a corrupt tc index = %d, want the skip to write nothing", len(entries))
	}
	if out := buf.String(); !strings.Contains(out, "casual log") || !strings.Contains(out, "tc index") {
		t.Errorf("log over a corrupt tc index = %q, want the loud skip naming the index", out)
	}
}

func TestCasualRoomSweepRecordsBothStorageSides(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	botInfo, _ := r.Info()
	botName := botInfo.GuestBotName
	injectPerGameBot(t, r, botLoseScripts)
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	driveHumanWonBotSweep(t, r, alice, sub)

	path := filepath.Join(config.CasualLogRoot,
		fmt.Sprintf("s%d_%s-vs-%s.txt", r.SeriesID(), "alice", botName))
	var body string
	waitFor(t, func() bool {
		b, err := os.ReadFile(path)
		body = string(b)
		return err == nil
	})
	if n := strings.Count(body, "game "); n != 2 {
		t.Fatalf("blocks = %d, want 2: %q", n, body)
	}
	head := fmt.Sprintf("game 1 room %s mode vs bot\npairing alice vs %s\n", r.ID(), botName)
	if !strings.HasPrefix(body, head) {
		t.Errorf("header = %q, want %q", body[:min(len(body), len(head))], head)
	}
	if !strings.Contains(body, "game 2 room "+r.ID()+" mode vs bot\npairing "+botName+" vs alice\n") {
		t.Errorf("game 2 header wrong in %q", body)
	}
	tc := config.TimeControls[0]
	if !strings.Contains(body, fmt.Sprintf("tc %d+%d bo%d outcome red\n", tc.InitialMin, tc.IncrementSec, config.SeriesBO3)) ||
		!strings.Contains(body, fmt.Sprintf("tc %d+%d bo%d outcome blue\n", tc.InitialMin, tc.IncrementSec, config.SeriesBO3)) {
		t.Errorf("tc or outcome lines wrong in %q", body)
	}
	ids := gameIDs(t, s)
	if len(ids) != 2 {
		t.Fatalf("persisted games = %d, want 2", len(ids))
	}
	textLines := 0
	for row := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(row, "M") {
			textLines++
		}
	}
	sqliteLines := 0
	for _, id := range ids {
		for _, row := range gameStatRows(t, s.store, id) {
			sqliteLines++
			if !strings.Contains(body, row.Line+"\n") {
				t.Errorf("sqlite stat %q missing from the text side", row.Line)
			}
		}
	}
	if textLines != sqliteLines {
		t.Errorf("M-lines text %d vs sqlite %d, want equal sets", textLines, sqliteLines)
	}
}
