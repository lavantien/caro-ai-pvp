package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var (
	reCorpusF = regexp.MustCompile(patCorpusF)
	reCorpusG = regexp.MustCompile(patCorpusG)
	reCorpusP = regexp.MustCompile(patCorpusP)
	reCorpusE = regexp.MustCompile(patCorpusE)
)

type corpusGame struct {
	File  string
	Index int
	Plies int
}

type corpusFile struct {
	Path        string
	Fingerprint string
	Games       []corpusGame
}

type keyAgg struct {
	Visits int
	Moves  map[string]int
	Games  map[string]bool
	Files  map[string]bool
	Ply    int
}

type corpusStats struct {
	Files     []corpusFile
	TotalP    int
	Keys      map[string]*keyAgg
	Plies     []int
	PlyUniq   map[int]int
	PlyVisits map[int]int
	MaxPly    int
}

func parseCorpusOnce(r io.Reader, path string, cs *corpusStats) (*corpusFile, error) {
	cf := &corpusFile{Path: path}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	gameID := ""
	ply := 0
	inGame := false
	for sc.Scan() {
		line := sc.Text()
		switch {
		case reCorpusF.MatchString(line):
			if m := reCorpusF.FindStringSubmatch(line); m != nil {
				cf.Fingerprint = m[1]
			}
		case reCorpusG.MatchString(line):
			m := reCorpusG.FindStringSubmatch(line)
			idx := 0
			if _, err := fmt.Sscanf(m[1], "%d", &idx); err != nil {
				return nil, fmt.Errorf("%s: bad game index %q: %w", path, m[1], err)
			}
			cf.Games = append(cf.Games, corpusGame{File: filepath.Base(path), Index: idx})
			gameID = path + "#" + m[1]
			ply = 0
			inGame = true
		case reCorpusP.MatchString(line) && inGame:
			m := reCorpusP.FindStringSubmatch(line)
			key, move := m[1], m[2]
			agg := cs.Keys[key]
			if agg == nil {
				agg = &keyAgg{Moves: map[string]int{}, Games: map[string]bool{}, Files: map[string]bool{}}
				cs.Keys[key] = agg
				agg.Ply = ply
				cs.PlyUniq[ply]++
			}
			agg.Visits++
			agg.Moves[move]++
			agg.Games[gameID] = true
			agg.Files[path] = true
			cs.TotalP++
			cs.PlyVisits[ply]++
			if ply > cs.MaxPly {
				cs.MaxPly = ply
			}
			ply++
		case reCorpusE.MatchString(line):
			inGame = false
			if len(cf.Games) > 0 {
				cf.Games[len(cf.Games)-1].Plies = ply
				cs.Plies = append(cs.Plies, ply)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: scan: %w", path, err)
	}
	return cf, nil
}

func loadCorpus(dir string) (*corpusStats, error) {
	cs := &corpusStats{Keys: map[string]*keyAgg{}, PlyUniq: map[int]int{}, PlyVisits: map[int]int{}}
	paths := []string{filepath.Join(dir, corpusMainFile)}
	instDirs, err := filepath.Glob(filepath.Join(dir, "inst*"))
	if err != nil {
		return nil, err
	}
	sort.Strings(instDirs)
	stem := corpusMainFile[:len(corpusMainFile)-len(".txt")]
	for _, d := range instDirs {
		shards, err := filepath.Glob(filepath.Join(d, stem+"-*.txt"))
		if err != nil {
			return nil, err
		}
		sort.Strings(shards)
		paths = append(paths, shards...)
	}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		cf, err := parseCorpusOnce(f, p, cs)
		f.Close()
		if err != nil {
			return nil, err
		}
		cs.Files = append(cs.Files, *cf)
	}
	return cs, nil
}

func (a *keyAgg) topCounts() []int {
	counts := make([]int, 0, len(a.Moves))
	for _, c := range a.Moves {
		counts = append(counts, c)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(counts)))
	return counts
}

func sortedKeys(m map[string]*keyAgg) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
