package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const defaultFloorOutRoot = "playground/arena/out"

const (
	extensionNone     = "not needed, interval covers 0.5"
	extensionNeeded   = "needed, run another block"
	extensionMidBlock = "hold, current block incomplete"
	extensionCap      = "blocked, block cap reached"
)

type floorConfig struct {
	name, seat                 string
	tcIdx, gamesPerDir         int
	dirs, blockDirs, maxBlocks int
	z                          float64
}

type floorGameResult struct {
	redWon, drawn bool
	moves         int
}

type dirTally struct {
	games, redWins, blueWins, draws int
	badLines, unfinished            int
}

type floorAgg struct {
	seat                     string
	dirs, games              int
	redWins, blueWins, draws int
	badLines, unfinished     int
	cfg                      floorConfig
}

func (a floorAgg) decisive() int {
	return a.redWins + a.blueWins
}

func (a floorAgg) coversHalf() bool {
	_, lower, upper, ok := waldInterval(a.redWins, a.decisive(), a.cfg.z)
	return ok && lower <= 0.5 && upper >= 0.5
}

func (a floorAgg) extension() string {
	if a.coversHalf() {
		return extensionNone
	}
	if a.dirs%a.cfg.blockDirs != 0 {
		return extensionMidBlock
	}
	if a.dirs/a.cfg.blockDirs >= a.cfg.maxBlocks {
		return extensionCap
	}
	return extensionNeeded
}

func validateFloorConfig(cfg floorConfig, tcCount int) error {
	if cfg.name == "" {
		return fmt.Errorf("floor name is empty")
	}
	if _, ok := seats[cfg.seat]; !ok {
		return fmt.Errorf("floor seat %q is not a known seat", cfg.seat)
	}
	if cfg.tcIdx < 0 || cfg.tcIdx >= tcCount {
		return fmt.Errorf("floor tc %d is out of range", cfg.tcIdx)
	}
	if cfg.gamesPerDir < 1 {
		return fmt.Errorf("floor games per dir %d, want at least 1", cfg.gamesPerDir)
	}
	if cfg.dirs < 0 {
		return fmt.Errorf("floor dirs %d, want at least 0", cfg.dirs)
	}
	if cfg.blockDirs < 1 {
		return fmt.Errorf("floor blockDirs %d, want at least 1", cfg.blockDirs)
	}
	if cfg.maxBlocks < 1 {
		return fmt.Errorf("floor maxBlocks %d, want at least 1", cfg.maxBlocks)
	}
	if cfg.z <= 0 {
		return fmt.Errorf("floor z %v, want positive", cfg.z)
	}
	return nil
}

func waldInterval(redWins, decisive int, z float64) (frac, lower, upper float64, ok bool) {
	if decisive < 1 || redWins < 0 || redWins > decisive || z <= 0 {
		return 0, 0, 0, false
	}
	frac = float64(redWins) / float64(decisive)
	half := z * math.Sqrt(frac*(1-frac)/float64(decisive))
	return frac, frac - half, frac + half, true
}

func floorGameLine(seat string, game int, r floorGameResult) string {
	switch {
	case r.drawn:
		return fmt.Sprintf("game %d: %s (red) vs %s (blue), drawn at %d", game, seat, seat, r.moves)
	case r.redWon:
		return fmt.Sprintf("game %d: %s (red) beat %s (blue) at %d", game, seat, seat, r.moves)
	default:
		return fmt.Sprintf("game %d: %s (blue) beat %s (red) at %d", game, seat, seat, r.moves)
	}
}

func parseFloorGameLine(line string) (floorGameResult, bool) {
	rest, ok := strings.CutPrefix(line, "game ")
	if !ok {
		return floorGameResult{}, false
	}
	i := strings.Index(rest, ": ")
	if i < 0 {
		return floorGameResult{}, false
	}
	if _, err := strconv.Atoi(rest[:i]); err != nil {
		return floorGameResult{}, false
	}
	body := rest[i+2:]
	moves, ok := trailingMoves(body)
	if !ok {
		return floorGameResult{}, false
	}
	switch {
	case strings.Contains(body, " (red) beat "):
		return floorGameResult{redWon: true, moves: moves}, true
	case strings.Contains(body, " (blue) beat "):
		return floorGameResult{moves: moves}, true
	case strings.Contains(body, ", drawn at "):
		return floorGameResult{drawn: true, moves: moves}, true
	}
	return floorGameResult{}, false
}

func trailingMoves(body string) (int, bool) {
	j := strings.LastIndex(body, " at ")
	if j < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(body[j+len(" at "):])
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseFloorHeader(line string) (name, seat string, games int, ok bool) {
	parts := strings.Fields(line)
	if len(parts) != 8 || parts[0] != "floor" || parts[2] != "tc" || parts[4] != "seat" || parts[6] != "games" {
		return "", "", 0, false
	}
	games, err := strconv.Atoi(parts[7])
	if err != nil {
		return "", "", 0, false
	}
	return parts[1], parts[5], games, true
}

func parseFloorDir(path string, cfg floorConfig) (dirTally, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dirTally{}, fmt.Errorf("read %s: %w", path, err)
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return dirTally{}, fmt.Errorf("%s: file is empty", path)
	}
	lines := strings.Split(text, "\n")
	name, seat, wantGames, ok := parseFloorHeader(lines[0])
	if !ok || name != cfg.name || seat != cfg.seat {
		return dirTally{}, fmt.Errorf("%s: header %q does not match pool %s seat %s", path, lines[0], cfg.name, cfg.seat)
	}
	var t dirTally
	for _, line := range lines[1:] {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "summary ") {
			continue
		}
		r, ok := parseFloorGameLine(line)
		if !ok {
			t.badLines++
			continue
		}
		t.games++
		switch {
		case r.drawn:
			t.draws++
		case r.redWon:
			t.redWins++
		default:
			t.blueWins++
		}
	}
	if short := wantGames - t.games; short > 0 {
		t.unfinished = short
	}
	return t, nil
}

func floorDirIndex(name string) (int, bool) {
	if !strings.HasPrefix(name, "d") || !strings.HasSuffix(name, ".txt") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "d"), ".txt"))
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

func aggregateFloorDirs(poolDir string, cfg floorConfig) floorAgg {
	a := floorAgg{seat: cfg.seat, cfg: cfg}
	entries, err := os.ReadDir(poolDir)
	if err != nil {
		return a
	}
	files := map[int]string{}
	var idxs []int
	for _, e := range entries {
		if n, ok := floorDirIndex(e.Name()); ok {
			idxs = append(idxs, n)
			files[n] = filepath.Join(poolDir, e.Name())
		}
	}
	sort.Ints(idxs)
	for _, n := range idxs {
		t, err := parseFloorDir(files[n], cfg)
		if err != nil {
			a.badLines++
			continue
		}
		a.dirs++
		a.games += t.games
		a.redWins += t.redWins
		a.blueWins += t.blueWins
		a.draws += t.draws
		a.badLines += t.badLines
		a.unfinished += t.unfinished
	}
	return a
}

func runFloorPool(cfg floorConfig, poolDir string, drive func(game int, redFirstSeat bool) (floorGameResult, error)) error {
	if err := os.MkdirAll(poolDir, 0o755); err != nil {
		return err
	}
	next := 1
	if entries, err := os.ReadDir(poolDir); err == nil {
		for _, e := range entries {
			if n, ok := floorDirIndex(e.Name()); ok && n >= next {
				next = n + 1
			}
		}
	}
	for d := next; d < next+cfg.dirs; d++ {
		path := filepath.Join(poolDir, fmt.Sprintf("d%d.txt", d))
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		_, werr := fmt.Fprintf(f, "floor %s tc %d seat %s games %d\n", cfg.name, cfg.tcIdx, cfg.seat, cfg.gamesPerDir)
		if werr == nil {
			var red, blue, draws int
			for g := 1; g <= cfg.gamesPerDir && werr == nil; g++ {
				var res floorGameResult
				res, werr = drive(g, g%2 == 1)
				if werr != nil {
					break
				}
				_, werr = fmt.Fprintln(f, floorGameLine(cfg.seat, g, res))
				if werr != nil {
					break
				}
				switch {
				case res.drawn:
					draws++
				case res.redWon:
					red++
				default:
					blue++
				}
			}
			if werr == nil {
				_, werr = fmt.Fprintf(f, "summary d%d: %d games, red %d, blue %d, draws %d\n", d, cfg.gamesPerDir, red, blue, draws)
			}
		}
		cerr := f.Close()
		if werr != nil {
			return werr
		}
		if cerr != nil {
			return cerr
		}
	}
	return nil
}

func renderFloorArtifact(w *bytes.Buffer, a floorAgg) {
	fmt.Fprintf(w, "# floor %s\n\n", a.cfg.name)
	fmt.Fprintf(w, "integrity: %d dirs, bad lines %d, unfinished %d\n\n", a.dirs, a.badLines, a.unfinished)
	fmt.Fprintln(w, "| seat | dirs | games | decisive | frac | 95% ci | covers 0.5 |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|---|")
	frac, lower, upper, ok := waldInterval(a.redWins, a.decisive(), a.cfg.z)
	fracText, ciText := "n a", "n a"
	if ok {
		fracText = fmt.Sprintf("%.3f", frac)
		ciText = fmt.Sprintf("%.3f to %.3f", lower, upper)
	}
	fmt.Fprintf(w, "| %s | %d | %d | %d | %s | %s | %t |\n\n", a.seat, a.dirs, a.games, a.decisive(), fracText, ciText, a.coversHalf())
	fmt.Fprintf(w, "extend: %s\n", a.extension())
}

func runFloorMode(name, seat, outRoot string, tcIdx, gamesPerDir, dirs, blockDirs, maxBlocks int, z float64) error {
	cfg := floorConfig{name: name, seat: seat, tcIdx: tcIdx, gamesPerDir: gamesPerDir, dirs: dirs, blockDirs: blockDirs, maxBlocks: maxBlocks, z: z}
	if err := validateFloorConfig(cfg, len(config.TimeControls)); err != nil {
		return err
	}
	poolDir := filepath.Join(outRoot, name)
	if cfg.dirs > 0 {
		drive := func(game int, redFirstSeat bool) (floorGameResult, error) {
			first := &tally{name: seat}
			second := &tally{name: seat}
			if !redFirstSeat {
				first, second = second, first
			}
			side, drawn, moves := play(first, second, cfg.tcIdx, game)
			if drawn {
				return floorGameResult{drawn: true, moves: moves}, nil
			}
			return floorGameResult{redWon: side == rules.Red, moves: moves}, nil
		}
		if err := runFloorPool(cfg, poolDir, drive); err != nil {
			return err
		}
	}
	agg := aggregateFloorDirs(poolDir, cfg)
	var buf bytes.Buffer
	renderFloorArtifact(&buf, agg)
	if _, err := os.Stdout.Write(buf.Bytes()); err != nil {
		return err
	}
	if err := os.MkdirAll(outRoot, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outRoot, name+".md"), buf.Bytes(), 0o644)
}
