package tourney

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func runLogDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestLogsReplayReplacesStaleSeriesFile(t *testing.T) {
	dir := runLogDir(t)
	red := Participant{Slot: 0, Name: "easy-1", Tier: "easy"}
	blue := Participant{Slot: 1, Name: "medium-1", Tier: "medium"}

	g := NewLogs(dir)
	if err := g.WriteSeriesHeader(7, 3, 1, config.SeriesBO3, red, blue); err != nil {
		t.Fatalf("header: %v", err)
	}
	if err := g.WriteSeriesLine(7, 3, "stale partial line"); err != nil {
		t.Fatalf("stale line: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Fatalf("close the aborted drive's logs: %v", err)
	}

	h := NewLogs(dir)
	if err := h.WriteSeriesHeader(7, 3, 1, config.SeriesBO3, red, blue); err != nil {
		t.Fatalf("replay header: %v", err)
	}
	if err := h.WriteSeriesLine(7, 3, "fresh line"); err != nil {
		t.Fatalf("replay line: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("close the replaying logs: %v", err)
	}

	name := fmt.Sprintf(config.TournamentSeriesLogFormat, 7, 3, "easy-1", "medium-1")
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	want := "run 7 series 3\n" +
		"pairing easy-1 (easy) vs medium-1 (medium)\n" +
		"tc 2+1 bo3\n" +
		"fresh line\n"
	if string(data) != want {
		t.Errorf("replayed log = %q, want %q", data, want)
	}
}

func TestLogsLifecycle(t *testing.T) {
	dir := runLogDir(t)
	g := NewLogs(dir)
	t.Cleanup(func() { _ = g.Close() })
	red := Participant{Slot: 0, Name: "hard-1", Tier: "hard"}
	blue := Participant{Slot: 1, Name: "easy 2", Tier: "easy"}

	if err := g.WriteSeriesLine(7, 3, "M1, Red, H8"); err == nil {
		t.Fatalf("line before header accepted, want the no-header error")
	}
	if err := g.WriteSeriesHeader(7, 3, 1, config.SeriesBO3, red, blue); err != nil {
		t.Fatalf("header: %v", err)
	}
	if err := g.WriteSeriesLine(7, 3, "M1, Red, H8, d=12"); err != nil {
		t.Fatalf("line 1: %v", err)
	}
	if err := g.WriteSeriesLine(7, 3, "H8"); err != nil {
		t.Fatalf("line 2: %v", err)
	}

	name := fmt.Sprintf(config.TournamentSeriesLogFormat, 7, 3, "hard-1", "easy_2")
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	want := "run 7 series 3\n" +
		"pairing hard-1 (hard) vs easy 2 (easy)\n" +
		"tc 2+1 bo3\n" +
		"M1, Red, H8, d=12\n" +
		"H8\n"
	if string(data) != want {
		t.Errorf("log = %q, want %q", data, want)
	}

	if err := g.CloseSeries(7, 3); err != nil {
		t.Fatalf("close series: %v", err)
	}
	if err := g.WriteSeriesLine(7, 3, "late"); err == nil {
		t.Errorf("line after close accepted, want the closed error")
	}
	if err := g.CloseSeries(7, 3); err == nil {
		t.Errorf("double close accepted, want the not-open error")
	}

	fresh := NewLogs(dir)
	t.Cleanup(func() { _ = fresh.Close() })
	if err := fresh.WriteSeriesHeader(8, 1, 0, config.SeriesBO3, red, blue); err != nil {
		t.Fatalf("second header setup: %v", err)
	}
	if err := fresh.WriteSeriesHeader(8, 1, 0, config.SeriesBO3, red, blue); err == nil {
		t.Errorf("duplicate header accepted, want the already-has-a-header error")
	}
}

func TestLogsCloseDrainsEverySeries(t *testing.T) {
	g := NewLogs(runLogDir(t))
	red := Participant{Slot: 0, Name: "a", Tier: "easy"}
	blue := Participant{Slot: 1, Name: "b", Tier: "easy"}
	for _, id := range []int64{1, 2} {
		if err := g.WriteSeriesHeader(1, id, 0, config.SeriesBO3, red, blue); err != nil {
			t.Fatalf("header %d: %v", id, err)
		}
	}
	if err := g.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Errorf("second close = %v, want nil (idempotent drain)", err)
	}
}

func TestLogsSanitizeKeepsNamesInsideTheDir(t *testing.T) {
	dir := runLogDir(t)
	g := NewLogs(dir)
	t.Cleanup(func() { _ = g.Close() })
	red := Participant{Slot: 0, Name: "../evil/spot", Tier: "hard"}
	blue := Participant{Slot: 1, Name: "plain", Tier: "easy"}
	if err := g.WriteSeriesHeader(2, 1, 0, config.SeriesBO3, red, blue); err != nil {
		t.Fatalf("header: %v", err)
	}
	name := fmt.Sprintf(config.TournamentSeriesLogFormat, 2, 1, ".._evil_spot", "plain")
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Errorf("log file: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("dir holds %d files, want 1 (no traversal)", len(entries))
	}
}

func TestLogsHeaderRejectsBadTimeControl(t *testing.T) {
	g := NewLogs(runLogDir(t))
	t.Cleanup(func() { _ = g.Close() })
	err := g.WriteSeriesHeader(1, 1, len(config.TimeControls), config.SeriesBO3,
		Participant{Name: "a", Tier: "easy"}, Participant{Name: "b", Tier: "easy"})
	if err == nil {
		t.Fatalf("header with tc out of range accepted, want rejection")
	}
}

func TestLogsSurfacesDirFailures(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	g := NewLogs(filepath.Join(blocker, "under"))
	err := g.WriteSeriesHeader(1, 1, 0, config.SeriesBO3,
		Participant{Name: "a", Tier: "easy"}, Participant{Name: "b", Tier: "easy"})
	if err == nil {
		t.Fatalf("header under a file path succeeded, want the mkdir error")
	}
}

func TestLogsCreateFaults(t *testing.T) {
	dir := runLogDir(t)
	red := Participant{Name: "a", Tier: "easy"}
	blue := Participant{Name: "b", Tier: "easy"}
	target := fmt.Sprintf(config.TournamentSeriesLogFormat, 3, 1, "a", "b")
	if err := os.Mkdir(filepath.Join(dir, target), 0o755); err != nil {
		t.Fatalf("seed the target directory: %v", err)
	}

	g := NewLogs(dir)
	err := g.WriteSeriesHeader(3, 1, 0, config.SeriesBO3, red, blue)
	if err == nil || !strings.Contains(err.Error(), "open series log") {
		t.Fatalf("header onto a directory = %v, want the open failure", err)
	}
}

func TestLogsWriteAndCloseFaults(t *testing.T) {
	dir := runLogDir(t)
	red := Participant{Name: "a", Tier: "easy"}
	blue := Participant{Name: "b", Tier: "easy"}
	g := NewLogs(dir)
	for _, id := range []int64{1, 2} {
		if err := g.WriteSeriesHeader(9, id, 0, config.SeriesBO3, red, blue); err != nil {
			t.Fatalf("header %d: %v", id, err)
		}
	}

	if err := g.open[seriesKey{9, 1}].Close(); err != nil {
		t.Fatalf("kill handle 1: %v", err)
	}
	err := g.WriteSeriesLine(9, 1, "M1")
	if err == nil || !strings.Contains(err.Error(), "series log run 9 series 1") {
		t.Fatalf("write onto a dead handle = %v, want the wrapped write failure", err)
	}
	if err := g.CloseSeries(9, 1); err == nil || !strings.Contains(err.Error(), "close series log run 9 series 1") {
		t.Fatalf("double close = %v, want the wrapped close failure", err)
	}

	if err := g.open[seriesKey{9, 2}].Close(); err != nil {
		t.Fatalf("kill handle 2: %v", err)
	}
	if err := g.Close(); err == nil || !strings.Contains(err.Error(), "series log run 9 series 2") {
		t.Fatalf("drain over a dead handle = %v, want the wrapped drain failure", err)
	}
	if len(g.open) != 0 {
		t.Errorf("open map holds %d handles after the drain, want it drained", len(g.open))
	}
}

func TestRunDirName(t *testing.T) {
	saved := config.TournamentLogRoot
	config.TournamentLogRoot = t.TempDir()
	t.Cleanup(func() { config.TournamentLogRoot = saved })
	at := time.Date(2026, 10, 4, 15, 4, 5, 0, time.UTC)
	got := RunDirName("smoke 32", at)
	want := filepath.Join(config.TournamentLogRoot, "20261004-150405-smoke_32")
	if got != want {
		t.Fatalf("RunDirName = %q, want %q", got, want)
	}
}

func TestWriteRunSummary(t *testing.T) {
	dir := runLogDir(t)
	g := NewLogs(dir)
	t.Cleanup(func() { _ = g.Close() })
	roster := []Participant{
		{Slot: 0, Name: "hard-2", Tier: "hard"},
		{Slot: 1, Name: "easy-1", Tier: "easy"},
	}
	board := []Standings{
		{Slot: 0, Rating: 1105, Wins: 16, Losses: 10, Draws: 0, SeriesWon: 7, GamesPlayed: 26},
		{Slot: 1, Rating: 769, Wins: 6, Losses: 16, Draws: 2, SeriesWon: 2, GamesPlayed: 24},
	}
	if err := g.WriteRunSummary(Run{ID: 9, TCIdx: 2, BOLen: config.SeriesBO3, StartRating: 1000}, roster, board); err != nil {
		t.Fatalf("summary: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, config.TournamentSummaryName))
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"run 9\n", "tc 3+2 bo3 start rating 1000\n",
		"rank\tparticipant\ttier\trating\tseries\twins\tlosses\tdraws\tgames\n",
		"1\thard-2\thard\t1105\t7\t16\t10\t0\t26\n",
		"2\teasy-1\teasy\t769\t2\t6\t16\t2\t24\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("summary misses %q:\n%s", want, text)
		}
	}
	if err := g.WriteRunSummary(Run{ID: 9}, []Participant{{Slot: 5, Name: "x", Tier: "easy"}}, board); err == nil {
		t.Error("summary with a slot outside the roster accepted, want rejection")
	}
}

func TestLogsWriteProgressAppendsTimestampedLines(t *testing.T) {
	dir := runLogDir(t)
	g := NewLogs(dir)
	t.Cleanup(func() { _ = g.Close() })
	if err := g.WriteProgress("run 5 series 1/2 started: easy-1 vs medium-1"); err != nil {
		t.Fatalf("progress 1: %v", err)
	}
	if err := g.WriteProgress("run 5 series 1/2 settled: easy-1 2-0 medium-1"); err != nil {
		t.Fatalf("progress 2: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, config.TournamentProgressName))
	if err != nil {
		t.Fatalf("read progress: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("progress lines = %d, want 2 (append semantics):\n%s", len(lines), data)
	}
	for i, want := range []string{
		"run 5 series 1/2 started: easy-1 vs medium-1",
		"run 5 series 1/2 settled: easy-1 2-0 medium-1",
	} {
		if len(lines[i]) < 17 || lines[i][15] != ' ' {
			t.Errorf("progress line %d = %q, want a leading 20060102-150405 stamp", i, lines[i])
			continue
		}
		if got := lines[i][16:]; got != want {
			t.Errorf("progress line %d = %q, want %q", i, got, want)
		}
	}
}

func TestLogsWriteProgressSurfacesDirFailures(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	g := NewLogs(filepath.Join(blocker, "under"))
	if err := g.WriteProgress("line"); err == nil {
		t.Fatalf("progress under a file path succeeded, want the mkdir error")
	}
}
