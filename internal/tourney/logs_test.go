package tourney

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// overrideLogDir points the configured log dir at a test directory.
func overrideLogDir(t *testing.T) string {
	t.Helper()
	saved := config.TournamentLogDir
	dir := t.TempDir()
	config.TournamentLogDir = dir
	t.Cleanup(func() { config.TournamentLogDir = saved })
	return dir
}

func TestLogsLifecycle(t *testing.T) {
	dir := overrideLogDir(t)
	g := NewLogs()
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
}

func TestLogsCloseDrainsEverySeries(t *testing.T) {
	overrideLogDir(t)
	g := NewLogs()
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
	dir := overrideLogDir(t)
	g := NewLogs()
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
	overrideLogDir(t)
	g := NewLogs()
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
	saved := config.TournamentLogDir
	config.TournamentLogDir = filepath.Join(blocker, "under")
	t.Cleanup(func() { config.TournamentLogDir = saved })

	g := NewLogs()
	err := g.WriteSeriesHeader(1, 1, 0, config.SeriesBO3,
		Participant{Name: "a", Tier: "easy"}, Participant{Name: "b", Tier: "easy"})
	if err == nil {
		t.Fatalf("header under a file path succeeded, want the mkdir error")
	}
}
