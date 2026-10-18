package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

var seriesFileNameRe = regexp.MustCompile(`^run\d+_s\d+_.+\.txt$`)

type seat struct {
	Name string
	Tier string
}

type gameMove struct {
	Color rules.Color
	Cell  rules.Cell
}

type gameOutcome uint8

const (
	outcomeDraw gameOutcome = iota
	outcomeRedWin
	outcomeBlueWin
)

func (o gameOutcome) winner() (rules.Color, bool) {
	switch o {
	case outcomeRedWin:
		return rules.Red, true
	case outcomeBlueWin:
		return rules.Blue, true
	}
	return 0, false
}

type parsedGame struct {
	Red     seat
	Blue    seat
	Outcome gameOutcome
	Moves   []gameMove
}

type seriesRecord struct {
	Path   string
	Run    int
	Series int
	TC     string
	Red    seat
	Blue   seat
	Games  []parsedGame
}

func parseSeriesFile(path string) (seriesRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return seriesRecord{}, fmt.Errorf("evalfit: read %s: %w", path, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	rec, err := parseSeriesText(path, lines)
	if err != nil {
		return rec, err
	}
	rec.Path = path
	return rec, nil
}

func parseSeriesText(path string, lines []string) (seriesRecord, error) {
	rec := seriesRecord{}
	sawHeader := false
	sawPairing := false
	var pending []gameMove
	flush := func(outcome gameOutcome, red, blue seat) error {
		wantRed, wantBlue := rec.Red, rec.Blue
		if len(rec.Games)%2 == 1 {
			wantRed, wantBlue = wantBlue, wantRed
		}
		if red.Name != wantRed.Name || blue.Name != wantBlue.Name {
			return fmt.Errorf("%s: game %d seats %s vs %s break the pairing color alternation %s vs %s",
				path, len(rec.Games)+1, red.Name, blue.Name, wantRed.Name, wantBlue.Name)
		}
		rec.Games = append(rec.Games, parsedGame{Red: red, Blue: blue, Outcome: outcome, Moves: pending})
		pending = nil
		return nil
	}
	for i, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		lineno := i + 1
		at := func(format string, args ...any) error {
			return fmt.Errorf("%s:%d: %s", path, lineno, fmt.Sprintf(format, args...))
		}
		switch {
		case strings.TrimSpace(line) == "":
			continue
		case strings.HasPrefix(line, "run ") && !sawHeader && !sawPairing:
			n, err := fmt.Sscanf(line, "run %d series %d", &rec.Run, &rec.Series)
			if err != nil || n != 2 {
				return rec, at("unparsable header %q", line)
			}
			sawHeader = true
		case strings.HasPrefix(line, "pairing "):
			if sawPairing {
				return rec, at("second pairing line")
			}
			red, blue, err := parsePairingLine(line)
			if err != nil {
				return rec, at("%v", err)
			}
			rec.Red, rec.Blue, sawPairing = red, blue, true
		case !sawPairing:
			return rec, at("line %q before the pairing line", line)
		case strings.HasPrefix(line, "tc "):
			rec.TC = strings.TrimPrefix(line, "tc ")
		case strings.HasPrefix(line, "M"):
			move, err := parseMoveLine(line)
			if err != nil {
				return rec, at("%v", err)
			}
			pending = append(pending, move)
		case strings.HasPrefix(line, "game "):
			game, outcome, red, blue, err := parseGameLine(line)
			if err != nil {
				return rec, at("%v", err)
			}
			if game != len(rec.Games)+1 {
				return rec, at("game %d out of order after %d games", game, len(rec.Games))
			}
			if err := flush(outcome, red, blue); err != nil {
				return rec, err
			}
		case strings.HasPrefix(line, "series "):
			continue
		default:
			return rec, at("unrecognized line %q", line)
		}
	}
	if !sawPairing {
		return rec, fmt.Errorf("%s: no pairing line", path)
	}
	return rec, nil
}

func parsePairingLine(line string) (seat, seat, error) {
	rest := strings.TrimPrefix(line, "pairing ")
	i := strings.LastIndex(rest, ") vs ")
	if i < 0 {
		return seat{}, seat{}, fmt.Errorf("unparsable pairing line %q", line)
	}
	red, err := parseSeat(rest[:i+1])
	if err != nil {
		return seat{}, seat{}, err
	}
	blue, err := parseSeat(rest[i+len(") vs "):])
	if err != nil {
		return seat{}, seat{}, err
	}
	return red, blue, nil
}

func parseSeat(s string) (seat, error) {
	if !strings.HasSuffix(s, ")") {
		return seat{}, fmt.Errorf("seat %q lacks the (tier) suffix", s)
	}
	j := strings.LastIndex(s, " (")
	if j < 0 || j == 0 {
		return seat{}, fmt.Errorf("seat %q lacks a name before (tier)", s)
	}
	return seat{Name: s[:j], Tier: s[j+2 : len(s)-1]}, nil
}

func parseMoveLine(line string) (gameMove, error) {
	fields := strings.Split(line, ", ")
	if len(fields) < 3 {
		return gameMove{}, fmt.Errorf("M-line %q carries fewer than 3 fields", line)
	}
	if len(fields[0]) < 2 || fields[0][0] != 'M' {
		return gameMove{}, fmt.Errorf("M-line %q lacks the M<number> tag", line)
	}
	if _, err := strconv.Atoi(fields[0][1:]); err != nil {
		return gameMove{}, fmt.Errorf("M-line %q tag is not M<number>", line)
	}
	var color rules.Color
	switch fields[1] {
	case "Red":
		color = rules.Red
	case "Blue":
		color = rules.Blue
	default:
		return gameMove{}, fmt.Errorf("M-line %q color %q is neither Red nor Blue", line, fields[1])
	}
	cell, err := rules.ParseCell(fields[2])
	if err != nil {
		return gameMove{}, fmt.Errorf("M-line %q cell: %v", line, err)
	}
	return gameMove{Color: color, Cell: cell}, nil
}

func parseGameLine(line string) (int, gameOutcome, seat, seat, error) {
	rest := strings.TrimPrefix(line, "game ")
	i := strings.Index(rest, ": ")
	if i < 0 {
		return 0, 0, seat{}, seat{}, fmt.Errorf("game line %q carries no body", line)
	}
	game, err := strconv.Atoi(rest[:i])
	if err != nil || game < 1 {
		return 0, 0, seat{}, seat{}, fmt.Errorf("game line %q number unparsable", line)
	}
	body := rest[i+2:]
	fail := func(role string) error {
		return fmt.Errorf("game %d %s seat unparsable: %q", game, role, body)
	}
	switch {
	case strings.Contains(body, " (red) beat "):
		head, tail, _ := strings.Cut(body, " (red) beat ")
		blueName := nameBefore(tail, " (blue)")
		if head == "" || blueName == "" {
			return 0, 0, seat{}, seat{}, fail("red/blue")
		}
		return game, outcomeRedWin, seat{Name: head}, seat{Name: blueName}, nil
	case strings.Contains(body, " (blue) beat "):
		head, tail, _ := strings.Cut(body, " (blue) beat ")
		redName := nameBefore(tail, " (red)")
		if head == "" || redName == "" {
			return 0, 0, seat{}, seat{}, fail("blue/red")
		}
		return game, outcomeBlueWin, seat{Name: redName}, seat{Name: head}, nil
	case strings.Contains(body, " (red) vs ") && strings.Contains(body, ", drawn at "):
		head, tail, _ := strings.Cut(body, " (red) vs ")
		blueName := nameBefore(tail, " (blue)")
		if head == "" || blueName == "" {
			return 0, 0, seat{}, seat{}, fail("red/blue")
		}
		return game, outcomeDraw, seat{Name: head}, seat{Name: blueName}, nil
	}
	return 0, 0, seat{}, seat{}, fmt.Errorf("game %d body carries no outcome: %q", game, body)
}

func nameBefore(s, marker string) string {
	i := strings.Index(s, marker)
	if i < 0 {
		return ""
	}
	return s[:i]
}

func scanSeriesFiles(roots []string) ([]string, error) {
	var out []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, fmt.Errorf("evalfit: scan %s: %w", root, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || filepath.Ext(name) != ".txt" || !seriesFileNameRe.MatchString(name) {
				continue
			}
			out = append(out, filepath.Join(root, name))
		}
	}
	return out, nil
}
