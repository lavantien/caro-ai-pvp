package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type pairLog struct {
	path         string
	a, b         string
	tcIdx, games int
	done         int
	winners      []string
	complete     bool
	exists       bool
	f            *os.File
}

func parsePairLogHeader(line string) (a, b string, tcIdx, games int, ok bool) {
	rest, ok := strings.CutPrefix(line, "pairlog v1 seats ")
	if !ok {
		return "", "", 0, 0, false
	}
	fields := strings.Fields(rest)
	if len(fields) != 5 || fields[1] != "tc" || fields[3] != "games" {
		return "", "", 0, 0, false
	}
	seats := strings.Split(fields[0], ",")
	if len(seats) != 2 || seats[0] == "" || seats[1] == "" {
		return "", "", 0, 0, false
	}
	tcIdx, err := strconv.Atoi(fields[2])
	if err != nil || tcIdx < 0 {
		return "", "", 0, 0, false
	}
	games, err = strconv.Atoi(fields[4])
	if err != nil || games < 1 {
		return "", "", 0, 0, false
	}
	return seats[0], seats[1], tcIdx, games, true
}

func parsePairLogGameLine(line string) (n int, winner string, moves int, ok bool) {
	fields := strings.Fields(line)
	if len(fields) != 6 || fields[0] != "game" || fields[2] != "winner" || fields[4] != "moves" {
		return 0, "", 0, false
	}
	n, err := strconv.Atoi(fields[1])
	if err != nil || n < 1 {
		return 0, "", 0, false
	}
	switch fields[3] {
	case winnerRed, winnerBlue, winnerDraw:
	default:
		return 0, "", 0, false
	}
	moves, err = strconv.Atoi(fields[5])
	if err != nil || moves < 1 {
		return 0, "", 0, false
	}
	return n, fields[3], moves, true
}

func loadPairLog(path, a, b string, tcIdx, games int) (pairLog, error) {
	pl := pairLog{path: path, a: a, b: b, tcIdx: tcIdx, games: games}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pl, nil
		}
		return pl, fmt.Errorf("read %s: %w", path, err)
	}
	text := string(data)
	if idx := strings.LastIndexByte(text, '\n'); !strings.HasSuffix(text, "\n") {
		if idx < 0 {
			text = ""
		} else {
			text = text[:idx+1]
		}
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return pl, fmt.Errorf("%s: file is empty", path)
	}
	pl.exists = true
	ha, hb, htc, hgames, ok := parsePairLogHeader(lines[0])
	if !ok || ha != a || hb != b || htc != tcIdx || hgames != games {
		return pl, fmt.Errorf("%s: header %q does not match invocation pairlog v1 seats %s,%s tc %d games %d", path, lines[0], a, b, tcIdx, games)
	}
	for i, line := range lines[1:] {
		lineno := i + 2
		if rest, ok := strings.CutPrefix(line, "complete "); ok {
			if pl.done != games {
				return pl, fmt.Errorf("%s:%d: complete marker after %d of %d games", path, lineno, pl.done, games)
			}
			if rest != fmt.Sprintf("%d games", games) {
				return pl, fmt.Errorf("%s:%d: complete marker %q, want %d games", path, lineno, line, games)
			}
			if pl.complete || i+1 != len(lines)-1 {
				return pl, fmt.Errorf("%s:%d: complete marker is not the last line", path, lineno)
			}
			pl.complete = true
			continue
		}
		n, winner, _, ok := parsePairLogGameLine(line)
		if !ok {
			return pl, fmt.Errorf("%s:%d: unparsable game line %q", path, lineno, line)
		}
		if n != pl.done+1 {
			return pl, fmt.Errorf("%s:%d: game %d out of order after %d games", path, lineno, n, pl.done)
		}
		pl.winners = append(pl.winners, winner)
		pl.done++
	}
	return pl, nil
}

func openPairLogForAppend(path, a, b string, tcIdx, games int) (pairLog, error) {
	pl, err := loadPairLog(path, a, b, tcIdx, games)
	if err != nil {
		return pl, err
	}
	if pl.complete {
		return pl, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return pl, fmt.Errorf("open %s: %w", path, err)
	}
	if pl.done == 0 && !pl.exists {
		if _, err := fmt.Fprintf(f, "pairlog v1 seats %s,%s tc %d games %d\n", a, b, tcIdx, games); err != nil {
			f.Close()
			return pl, fmt.Errorf("write %s header: %w", path, err)
		}
	}
	pl.f = f
	return pl, nil
}

func (pl *pairLog) appendGame(n int, winner string, moves int) error {
	if pl.f == nil {
		return fmt.Errorf("pairlog %s is not open for append", pl.path)
	}
	if n != pl.done+1 {
		return fmt.Errorf("pairlog %s game %d out of order after %d games", pl.path, n, pl.done)
	}
	if _, err := fmt.Fprintf(pl.f, "game %d winner %s moves %d\n", n, winner, moves); err != nil {
		return err
	}
	pl.winners = append(pl.winners, winner)
	pl.done++
	return nil
}

func (pl *pairLog) appendComplete() error {
	if pl.f == nil {
		return fmt.Errorf("pairlog %s is not open for append", pl.path)
	}
	if pl.done != pl.games {
		return fmt.Errorf("pairlog %s complete after %d of %d games", pl.path, pl.done, pl.games)
	}
	_, err := fmt.Fprintf(pl.f, "complete %d games\n", pl.games)
	return err
}

func (pl *pairLog) close() error {
	if pl.f == nil {
		return nil
	}
	err := pl.f.Close()
	pl.f = nil
	return err
}
