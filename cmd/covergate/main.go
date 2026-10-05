package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

var corePackages = config.CorePackages

type fileCov struct {
	total   int64
	covered int64
}

func parseProfile(r io.Reader) (map[string]fileCov, error) {
	files := make(map[string]fileCov)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("bad profile line %q", line)
		}
		stmts, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad statement count in %q", line)
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad execution count in %q", line)
		}
		file := fileOf(fields[0])
		if file == "" {
			return nil, fmt.Errorf("bad block range in %q", line)
		}
		fc := files[file]
		fc.total += stmts
		if count > 0 {
			fc.covered += stmts
		}
		files[file] = fc
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func fileOf(loc string) string {
	i := strings.LastIndex(loc, ":")
	if i < 0 {
		return ""
	}
	return loc[:i]
}

func corePackage(file string) string {
	for _, p := range corePackages {
		if strings.Contains(file, "/"+p+"/") {
			return p
		}
	}
	return ""
}

func pct(covered, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total) * 100
}

// coreCoverage folds the per-file coverage into per-core-package totals.
func coreCoverage(files map[string]fileCov) map[string]*fileCov {
	core := make(map[string]*fileCov)
	for file, fc := range files {
		if p := corePackage(file); p != "" {
			c, ok := core[p]
			if !ok {
				c = &fileCov{}
				core[p] = c
			}
			c.total += fc.total
			c.covered += fc.covered
		}
	}
	return core
}

func gate(files map[string]fileCov) error {
	var total, covered int64
	for _, fc := range files {
		total += fc.total
		covered += fc.covered
	}
	if total == 0 {
		return fmt.Errorf("empty profile, no statements counted")
	}
	var errs []string
	overall := pct(covered, total)
	fmt.Printf("covergate: overall %.1f%% (min %.0f%%)\n", overall, config.QualityCoverageOverallMin)
	if overall+1e-9 < config.QualityCoverageOverallMin {
		errs = append(errs, fmt.Sprintf("overall %.1f%% below %.0f%%", overall, config.QualityCoverageOverallMin))
	}
	for _, p := range corePackages {
		c, ok := coreCoverage(files)[p]
		if !ok || c.total == 0 {
			continue
		}
		cp := pct(c.covered, c.total)
		fmt.Printf("covergate: %s %.1f%% (min %.0f%%)\n", p, cp, config.QualityCoverageCoreMin)
		if cp+1e-9 < config.QualityCoverageCoreMin {
			errs = append(errs, fmt.Sprintf("%s %.1f%% below %.0f%%", p, cp, config.QualityCoverageCoreMin))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("coverage gates failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// pcts reports the two badge numbers and the two thresholds they are judged
// against, so the badge publisher colors from the config hub's own values:
// global over the whole profile, core as the weakest core package (the
// all-must-pass gate's honest aggregate). A profile with no core statements
// reports core 0.0 rather than an error: reporting, not judging.
func pcts(files map[string]fileCov, w io.Writer) error {
	var total, covered int64
	for _, fc := range files {
		total += fc.total
		covered += fc.covered
	}
	if total == 0 {
		return fmt.Errorf("empty profile, no statements counted")
	}
	core := 0.0
	coreSet := false
	for _, c := range coreCoverage(files) {
		if c.total == 0 {
			continue
		}
		if cp := pct(c.covered, c.total); !coreSet || cp < core {
			core, coreSet = cp, true
		}
	}
	_, err := fmt.Fprintf(w, "global=%.1f core=%.1f global-min=%.1f core-min=%.1f\n",
		pct(covered, total), core, config.QualityCoverageOverallMin, config.QualityCoverageCoreMin)
	return err
}

func check(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	files, err := parseProfile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return gate(files)
}

// report prints the badge percentages without judging them.
func report(path string, w io.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	files, err := parseProfile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return pcts(files, w)
}

func runCLI(args []string) int {
	if len(args) == 2 && args[0] == "-pcts" {
		if err := report(args[1], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "covergate:", err)
			return 1
		}
		return 0
	}
	if len(args) != 1 || args[0] == "-pcts" {
		fmt.Fprintln(os.Stderr, "usage: covergate [-pcts] <coverprofile>")
		return 2
	}
	if err := check(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, "covergate:", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(runCLI(os.Args[1:]))
}
