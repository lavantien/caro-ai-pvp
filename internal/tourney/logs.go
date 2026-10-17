package tourney

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type Logs struct {
	mu   sync.Mutex
	dir  string
	open map[seriesKey]*os.File
}

type seriesKey struct {
	run    int64
	series int64
}

func NewLogs(dir string) *Logs {
	return &Logs{dir: dir, open: make(map[seriesKey]*os.File)}
}

func RunDirName(label string, at time.Time) string {
	return filepath.Join(config.TournamentLogRoot,
		at.Format(config.TournamentRunDirFormat)+"-"+sanitizeName(label))
}

func (g *Logs) WriteRunSummary(run Run, roster []Participant, board []Standings) error {
	name := make(map[int]Participant, len(roster))
	for _, p := range roster {
		name[p.Slot] = p
	}
	var sb strings.Builder
	tc := config.TimeControls[run.TCIdx]
	fmt.Fprintf(&sb, "run %d\ntc %d+%d bo%d start rating %d\nfinished %s\n\n",
		run.ID, tc.InitialMin, tc.IncrementSec, run.BOLen, run.StartRating,
		time.Now().Format(config.TournamentRunDirFormat))
	sb.WriteString("rank\tparticipant\ttier\trating\tseries\twins\tlosses\tdraws\tgames\n")
	for rank, st := range board {
		p, ok := name[st.Slot]
		if !ok {
			return fmt.Errorf("tourney: summary slot %d is outside the roster", st.Slot)
		}
		fmt.Fprintf(&sb, "%d\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\n",
			rank+1, p.Name, p.Tier, st.Rating, st.SeriesWon,
			st.Wins, st.Losses, st.Draws, st.GamesPlayed)
	}
	path := filepath.Join(g.dir, config.TournamentSummaryName)
	if err := os.MkdirAll(g.dir, 0o755); err != nil {
		return fmt.Errorf("tourney: create run folder %s: %w", g.dir, err)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("tourney: write run summary %s: %w", path, err)
	}
	return nil
}

func (g *Logs) WriteProgress(line string) (err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := os.MkdirAll(g.dir, 0o755); err != nil {
		return fmt.Errorf("tourney: mkdir %s: %w", g.dir, err)
	}
	path := filepath.Join(g.dir, config.TournamentProgressName)
	f, oerr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if oerr != nil {
		return fmt.Errorf("tourney: open progress %s: %w", path, oerr)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("tourney: close progress %s: %w", path, cerr)
		}
	}()
	if _, err = fmt.Fprintf(f, "%s %s\n", time.Now().Format(config.TournamentRunDirFormat), line); err != nil {
		return fmt.Errorf("tourney: write progress %s: %w", path, err)
	}
	return nil
}

func sanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
}

func (g *Logs) WriteSeriesHeader(run, series int64, tcIdx, boLen int, red, blue Participant) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if tcIdx < 0 || tcIdx >= len(config.TimeControls) {
		return fmt.Errorf("tourney: tc_idx %d outside the %d configured time controls", tcIdx, len(config.TimeControls))
	}
	tc := config.TimeControls[tcIdx]
	lines := []string{
		fmt.Sprintf("run %d series %d", run, series),
		fmt.Sprintf("pairing %s (%s) vs %s (%s)", red.Name, red.Tier, blue.Name, blue.Tier),
		fmt.Sprintf("tc %d+%d bo%d", tc.InitialMin, tc.IncrementSec, boLen),
	}
	return g.writeLines(seriesKey{run, series}, red.Name, blue.Name, lines)
}

func (g *Logs) WriteSeriesLine(run, series int64, line string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.writeLines(seriesKey{run, series}, "", "", []string{line})
}

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
	} else if redName != "" {
		return fmt.Errorf("tourney: series log run %d series %d already has a header", k.run, k.series)
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("tourney: series log run %d series %d: %w", k.run, k.series, err)
		}
	}
	return nil
}

func (g *Logs) create(k seriesKey, redName, blueName string) (*os.File, error) {
	if err := os.MkdirAll(g.dir, 0o755); err != nil {
		return nil, fmt.Errorf("tourney: mkdir %s: %w", g.dir, err)
	}
	path := filepath.Join(g.dir, fmt.Sprintf(
		config.TournamentSeriesLogFormat, k.run, k.series, sanitizeName(redName), sanitizeName(blueName)))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("tourney: open series log %s: %w", path, err)
	}
	g.open[k] = f
	return f, nil
}

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
