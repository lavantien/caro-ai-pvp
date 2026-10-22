package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	reHeader = regexp.MustCompile(patArenaHeader)
	reResult = regexp.MustCompile(patArenaResult)
	rePonder = regexp.MustCompile(patPonderLine)
	reMLine  = regexp.MustCompile(patMLine)
)

type arenaGame struct {
	File    string
	GameNo  int
	TC      string
	Red     string
	Blue    string
	Winner  string
	Moves   int
	Opening string
	Seq     string
	HasSeq  bool
}

type ponderCounts struct {
	Arms   int
	Adopt  int
	Reject int
	Miss   int
}

type arenaFile struct {
	Path   string
	Games  []arenaGame
	Ponder map[string]*ponderCounts
}

func parseArena(r io.Reader, path string) (*arenaFile, error) {
	af := &arenaFile{Path: path, Ponder: map[string]*ponderCounts{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	tc := ""
	var seq []string
	for sc.Scan() {
		line := sc.Text()
		if m := reHeader.FindStringSubmatch(line); m != nil {
			tc = m[1]
			continue
		}
		if m := reMLine.FindStringSubmatch(line); m != nil {
			seq = append(seq, m[4])
			continue
		}
		if m := reResult.FindStringSubmatch(line); m != nil {
			moves := 0
			if _, err := fmt.Sscanf(m[5], "%d", &moves); err != nil {
				return nil, fmt.Errorf("%s: bad move count %q: %w", path, m[5], err)
			}
			gameNo := 0
			if _, err := fmt.Sscanf(m[1], "%d", &gameNo); err != nil {
				return nil, fmt.Errorf("%s: bad game number %q: %w", path, m[1], err)
			}
			g := arenaGame{
				File:   filepath.Base(path),
				GameNo: gameNo,
				TC:     tc,
				Red:    m[2],
				Blue:   m[3],
				Winner: m[4],
				Moves:  moves,
			}
			if len(seq) > 0 {
				g.HasSeq = true
				g.Seq = strings.Join(seq, " ")
				opening := seq
				if len(opening) > 3 {
					opening = opening[:3]
				}
				g.Opening = strings.Join(opening, " ")
			}
			seq = seq[:0]
			af.Games = append(af.Games, g)
			continue
		}
		if m := rePonder.FindStringSubmatch(line); m != nil {
			c := af.Ponder[m[2]]
			if c == nil {
				c = &ponderCounts{}
				af.Ponder[m[2]] = c
			}
			c.Arms++
			switch m[4] {
			case "adopt":
				c.Adopt++
			case "reject":
				c.Reject++
			case "miss":
				c.Miss++
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: scan: %w", path, err)
	}
	return af, nil
}

func loadArenaFiles(roots []string) ([]*arenaFile, error) {
	var out []*arenaFile
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			path := filepath.Join(root, name)
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			af, err := parseArena(f, path)
			f.Close()
			if err != nil {
				return nil, err
			}
			if len(af.Games) > 0 {
				out = append(out, af)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func seatBase(name string) string {
	if i := strings.IndexByte(name, '-'); i >= 0 {
		return name[:i]
	}
	return name
}

func bucketOf(red, blue string) string {
	switch {
	case red == blue:
		return bucketIdentical
	case seatBase(red) == seatBase(blue):
		return bucketSameBase
	default:
		return bucketCross
	}
}
