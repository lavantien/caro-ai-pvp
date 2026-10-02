package tourney

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Logs writes the Scenario 2 txt run logs: one file per series under
// config.TournamentLogDir, named by config.TournamentSeriesLogFormat with
// sanitized display names. The header block opens a series' file lazily,
// every line appends in arrival order, and CloseSeries ends the file's
// lifecycle. Errors return, never swallow.
type Logs struct {
	mu   sync.Mutex
	dir  string
	open map[seriesKey]*os.File
}

// seriesKey identifies one series' log file.
type seriesKey struct {
	run    int64
	series int64
}

// NewLogs points the writer at the configured log dir.
func NewLogs() *Logs {
	return &Logs{dir: config.TournamentLogDir, open: make(map[seriesKey]*os.File)}
}

// sanitizeName folds everything outside the filename-safe charset into "_"
// so a display name can never escape the log directory.
func sanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
}

// WriteSeriesHeader opens the series' file (lazily, like any first line)
// and writes the header block: run and series, the pairing with tiers, the
// time control, and the best-of length.
func (g *Logs) WriteSeriesHeader(run, series int64, tcIdx, boLen int, red, blue Participant) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	tc := config.TimeControls[tcIdx]
	lines := []string{
		fmt.Sprintf("run %d series %d", run, series),
		fmt.Sprintf("pairing %s (%s) vs %s (%s)", red.Name, red.Tier, blue.Name, blue.Tier),
		fmt.Sprintf("tc %d+%d bo%d", tc.InitialSec, tc.IncrementSec, boLen),
	}
	return g.writeLines(seriesKey{run, series}, red.Name, blue.Name, lines)
}

// WriteSeriesLine appends one event line to the series' log in arrival
// order: a move or the raw M-line telemetry of a bot turn, exactly as the
// caller hands it over. The header must have opened the file.
func (g *Logs) WriteSeriesLine(run, series int64, line string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.writeLines(seriesKey{run, series}, "", "", []string{line})
}

// writeLines appends lines to the series' open file, opening it on first
// use when the header carries the display names.
func (g *Logs) writeLines(k seriesKey, redName, blueName string, lines []string) error {
	f, ok := g.open[k]
	if !ok {
		if redName == "" {
			return fmt.Errorf("tourney: series log run %d series %d has no header yet", k.run, k.series)
		}
		var err error
		f, err = g.create(k, redName, blueName)
		if err != nil {
			return err
		}
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("tourney: series log run %d series %d: %w", k.run, k.series, err)
		}
	}
	return nil
}

// create makes the log directory and the series' file, recording the handle.
func (g *Logs) create(k seriesKey, redName, blueName string) (*os.File, error) {
	if err := os.MkdirAll(g.dir, 0o755); err != nil {
		return nil, fmt.Errorf("tourney: mkdir %s: %w", g.dir, err)
	}
	path := filepath.Join(g.dir, fmt.Sprintf(
		config.TournamentSeriesLogFormat, k.run, k.series, sanitizeName(redName), sanitizeName(blueName)))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("tourney: open series log %s: %w", path, err)
	}
	g.open[k] = f
	return f, nil
}

// CloseSeries closes the series' file: its lifecycle ends, later writes
// error instead of silently reopening.
func (g *Logs) CloseSeries(run, series int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	k := seriesKey{run, series}
	f, ok := g.open[k]
	if !ok {
		return fmt.Errorf("tourney: series log run %d series %d is not open", run, series)
	}
	delete(g.open, k)
	if err := f.Close(); err != nil {
		return fmt.Errorf("tourney: close series log run %d series %d: %w", run, series, err)
	}
	return nil
}

// Close closes every open series file, the run or process boundary.
func (g *Logs) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var err error
	for k, f := range g.open {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("tourney: close series log run %d series %d: %w", k.run, k.series, cerr)
		}
		delete(g.open, k)
	}
	return err
}
