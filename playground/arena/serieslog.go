package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type seriesLog struct {
	path                  string
	redFirst, blueFirst   string
	games                 []Result
	hasVerdict            bool
	verdictWinner         string
	firstWins, secondWins int
}

func parseSeriesLog(path string) (seriesLog, error) {
	sl := seriesLog{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return sl, fmt.Errorf("read %s: %w", path, err)
	}
	sawPairing := false
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		lineno := i + 1
		switch {
		case strings.HasPrefix(line, "pairing "):
			if sawPairing {
				return sl, fmt.Errorf("%s:%d: second pairing line", path, lineno)
			}
			rn, bn, ok := parsePairingLine(line)
			if !ok {
				return sl, fmt.Errorf("%s:%d: unparsable pairing line %q", path, lineno, line)
			}
			sl.redFirst, sl.blueFirst, sawPairing = rn, bn, true
		case strings.HasPrefix(line, "game "):
			r, ok, err2 := parseGameResult(line)
			if err2 != nil {
				return sl, fmt.Errorf("%s:%d: %w", path, lineno, err2)
			}
			if !ok {
				continue
			}
			if r.game != len(sl.games)+1 {
				return sl, fmt.Errorf("%s:%d: game %d out of order after %d games", path, lineno, r.game, len(sl.games))
			}
			if sawPairing {
				if err2 := sl.checkSeats(r); err2 != nil {
					return sl, fmt.Errorf("%s:%d: %w", path, lineno, err2)
				}
			}
			sl.games = append(sl.games, r.result)
		case strings.HasPrefix(line, "series "):
			winner, first, second, ok := parseVerdictLine(line)
			if !ok {
				return sl, fmt.Errorf("%s:%d: unparsable series line %q", path, lineno, line)
			}
			sl.hasVerdict, sl.verdictWinner, sl.firstWins, sl.secondWins = true, winner, first, second
		}
	}
	if !sawPairing {
		return sl, fmt.Errorf("%s: no pairing line", path)
	}
	return sl, nil
}

func parsePairingLine(line string) (redName, blueName string, ok bool) {
	rest, ok := strings.CutPrefix(line, "pairing ")
	if !ok {
		return "", "", false
	}
	i := strings.LastIndex(rest, ") vs ")
	if i < 0 {
		return "", "", false
	}
	redName, ok = cutTierName(rest[:i+1])
	if !ok {
		return "", "", false
	}
	blueName, ok = cutTierName(rest[i+len(") vs "):])
	return redName, blueName, ok
}

func cutTierName(s string) (string, bool) {
	if !strings.HasSuffix(s, ")") {
		return "", false
	}
	j := strings.LastIndex(s, " (")
	if j < 0 {
		return "", false
	}
	name := s[:j]
	if name == "" {
		return "", false
	}
	return name, true
}

type gameLineResult struct {
	game              int
	result            Result
	redName, blueName string
}

func (sl seriesLog) checkSeats(r gameLineResult) error {
	wantRed, wantBlue := sl.redFirst, sl.blueFirst
	if r.game%2 == 0 {
		wantRed, wantBlue = wantBlue, wantRed
	}
	if r.redName != wantRed || r.blueName != wantBlue {
		return fmt.Errorf("game %d seats %s (red) vs %s (blue), want the pairing's color alternation %s (red) vs %s (blue)",
			r.game, r.redName, r.blueName, wantRed, wantBlue)
	}
	return nil
}

func parseGameResult(line string) (gameLineResult, bool, error) {
	out := gameLineResult{}
	rest, ok := strings.CutPrefix(line, "game ")
	if !ok {
		return out, false, nil
	}
	i := strings.Index(rest, ": ")
	if i < 0 {
		return out, false, fmt.Errorf("game line carries no body: %q", line)
	}
	game, err := strconv.Atoi(rest[:i])
	if err != nil || game < 1 {
		return out, false, fmt.Errorf("game line number unparsable: %q", line)
	}
	out.game = game
	body := rest[i+2:]
	var redWon, drawn bool
	switch {
	case strings.Contains(body, " (red) beat "):
		head, tail, _ := strings.Cut(body, " (red) beat ")
		redWon = true
		out.redName = head
		out.blueName = nameBefore(tail, " (blue)")
	case strings.Contains(body, " (blue) beat "):
		head, tail, _ := strings.Cut(body, " (blue) beat ")
		out.blueName = head
		out.redName = nameBefore(tail, " (red)")
	case strings.Contains(body, " (red) vs ") && strings.Contains(body, ", drawn at "):
		head, tail, _ := strings.Cut(body, " (red) vs ")
		drawn = true
		out.redName = head
		out.blueName = nameBefore(tail, " (blue)")
	default:
		return out, false, fmt.Errorf("game %d body carries no outcome: %q", game, body)
	}
	if out.redName == "" || out.blueName == "" {
		return out, false, fmt.Errorf("game %d names unparsable: %q", game, body)
	}
	switch {
	case drawn:
		out.result = Draw
	case game%2 == 1 && redWon, game%2 == 0 && !redWon:
		out.result = Win
	default:
		out.result = Loss
	}
	return out, true, nil
}

func nameBefore(s, marker string) string {
	i := strings.Index(s, marker)
	if i < 0 {
		return ""
	}
	return s[:i]
}

func parseVerdictLine(line string) (winner string, firstWins, secondWins int, ok bool) {
	rest, ok := strings.CutPrefix(line, "series ")
	if !ok {
		return "", 0, 0, false
	}
	i := strings.LastIndex(rest, " ")
	if i <= 0 {
		return "", 0, 0, false
	}
	winner = rest[:i]
	score := strings.SplitN(rest[i+1:], "-", 2)
	if len(score) != 2 || winner == "" {
		return "", 0, 0, false
	}
	firstWins, err := strconv.Atoi(score[0])
	if err != nil {
		return "", 0, 0, false
	}
	secondWins, err = strconv.Atoi(score[1])
	if err != nil {
		return "", 0, 0, false
	}
	return winner, firstWins, secondWins, true
}

func replaySeriesPtnml(sl seriesLog) Ptnml {
	var p Ptnml
	for _, r := range sl.games {
		p.AddGame(r)
	}
	p.Finish()
	return p
}

func (sl seriesLog) winDrawLoss() (wins, draws, losses int) {
	for _, r := range sl.games {
		switch r {
		case Win:
			wins++
		case Draw:
			draws++
		case Loss:
			losses++
		}
	}
	return wins, draws, losses
}

const tourneyLogsEnv = "CARO_TOURNY_LOGS"

func tourneyLogRoots() []string {
	var roots []string
	add := func(path string) {
		for _, seen := range roots {
			if seen == path {
				return
			}
		}
		roots = append(roots, path)
	}
	if v := os.Getenv(tourneyLogsEnv); v != "" {
		add(v)
	}
	wd, err := os.Getwd()
	if err != nil {
		return roots
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if root := filepath.Join(dir, config.TournamentLogRoot); dirIsDir(root) {
			add(root)
		}
		if main := mainCheckoutRoot(dir); main != "" {
			if root := filepath.Join(main, config.TournamentLogRoot); dirIsDir(root) {
				add(root)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return roots
		}
	}
}

func dirIsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func mainCheckoutRoot(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return ""
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir:"))
	if gitdir == "" {
		return ""
	}
	gitdir = filepath.FromSlash(gitdir)
	marker := filepath.Join(".git", "worktrees")
	i := strings.LastIndex(filepath.ToSlash(gitdir), filepath.ToSlash(marker))
	if i < 0 {
		return ""
	}
	return filepath.Dir(gitdir[:i])
}
