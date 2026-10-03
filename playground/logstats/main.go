// Command logstats parses every per-series txt tournament log in a directory
// (the files internal/tourney's Logs writes under config.TournamentLogDir)
// and renders the markdown evidence report of the v0.20 chain: inventory,
// per-participant standings folded through the runner's seat law, per-tier
// telemetry from the M-lines, a strength verdict with tier-inversion lines,
// and a parse-integrity ledger that counts and lists every line the parsers
// rejected. Deterministic: same logs in, same bytes out.
//
// Run from the repo root through the make target:
//
//	make logstats ARGS="--dir tourney-logs --out report.md"
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func main() {
	flagDir := flag.String("dir", "", "directory holding one run's per-series txt logs (default: the newest run folder under config.TournamentLogRoot)")
	out := flag.String("out", "", "write the markdown report to this file instead of stdout")
	flag.Parse()

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "logstats: "+format+"\n", args...)
		os.Exit(2)
	}
	dir := *flagDir
	if dir == "" {
		var err error
		dir, err = latestRunDir(config.TournamentLogRoot)
		if err != nil {
			fail("no --dir and no run folder under %s: %v", config.TournamentLogRoot, err)
		}
	}
	paths, err := readSeriesLogs(dir)
	if err != nil {
		fail("%v", err)
	}
	files := make([]seriesFile, 0, len(paths))
	for _, path := range paths {
		f, err := parseFile(path)
		if err != nil {
			fail("%v", err)
		}
		files = append(files, f)
	}
	var report strings.Builder
	renderReport(&report, analyze(dir, files))
	if *out == "" {
		fmt.Print(report.String())
		return
	}
	if err := os.WriteFile(*out, []byte(report.String()), 0o644); err != nil {
		fail("write %s: %v", *out, err)
	}
}

// readSeriesLogs lists the dir's plain .txt files in name order: one series
// log per file under the tourney writer's naming.
func readSeriesLogs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".txt") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths, nil
}

// latestRunDir picks the lexicographically newest run folder under root:
// the TournamentRunDirFormat timestamp prefix sorts chronologically.
func latestRunDir(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	newest := ""
	for _, e := range entries {
		if e.IsDir() && e.Name() > newest {
			newest = e.Name()
		}
	}
	if newest == "" {
		return "", fmt.Errorf("no run folders under %s", root)
	}
	return filepath.Join(root, newest), nil
}
