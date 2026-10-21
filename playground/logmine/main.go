package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	dbs := flag.String("db", "", "comma-separated paths of caro sqlite databases to mine")
	dir := flag.String("dir", "", "mine every per-series txt log under this tournament log root instead of databases")
	ponder := flag.Bool("ponder", false, "with -dir, emit the ponder adoption artifact")
	runaway := flag.Bool("runaway", false, "with -dir, emit the easy-tier depth runaway artifact")
	hitrate := flag.Bool("hitrate", false, "with -dir, emit the ponder hit rate artifact")
	outDir := flag.String("out", "out", "directory for the markdown artifacts")
	flag.Parse()
	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "logmine: "+format+"\n", args...)
		os.Exit(2)
	}
	if strings.TrimSpace(*dbs) == "" && strings.TrimSpace(*dir) == "" {
		fail("no -db or -dir given")
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fail("out dir: %v", err)
	}
	if strings.TrimSpace(*dir) != "" {
		res, err := mineLogDir(*dir)
		if err != nil {
			fail("%v", err)
		}
		m := mined{
			tournament: res.tournament, firstWins: res.firstWins, outcomes: res.outcomes,
			rejects: res.rejects, files: res.files,
		}
		report := renderReport(filepath.Base(*dir), *dir, m)
		outPath := filepath.Join(*outDir, sanitizeName(*dir)+"-logmine.md")
		if err := os.WriteFile(outPath, []byte(report), 0o644); err != nil {
			fail("write %s: %v", outPath, err)
		}
		fmt.Fprintf(os.Stderr, "logmine: dir %s: %d files, %d moves, %d rejects -> %s\n",
			*dir, res.files, len(res.tournament), res.rejects, outPath)
		if *ponder {
			pOut := filepath.Join(*outDir, "ponder-adoption-logmine.md")
			if err := os.WriteFile(pOut, []byte(renderPonderReport(*dir, res.files, ponderStats(res.tournament))), 0o644); err != nil {
				fail("write %s: %v", pOut, err)
			}
			fmt.Fprintf(os.Stderr, "logmine: ponder report -> %s\n", pOut)
		}
		if *runaway {
			rOut := filepath.Join(*outDir, "easy-depth-runaway-logmine.md")
			if err := os.WriteFile(rOut, []byte(renderRunawayReport(*dir, res.files, mineRunaway(res.tournament))), 0o644); err != nil {
				fail("write %s: %v", rOut, err)
			}
			fmt.Fprintf(os.Stderr, "logmine: runaway report -> %s\n", rOut)
		}
		if *hitrate {
			hOut := filepath.Join(*outDir, "ponder-hitrate-logmine.md")
			if err := os.WriteFile(hOut, []byte(renderHitReport(*dir, res.files, hitStats(res.tournament))), 0o644); err != nil {
				fail("write %s: %v", hOut, err)
			}
			fmt.Fprintf(os.Stderr, "logmine: hitrate report -> %s\n", hOut)
		}
		return
	}
	for _, raw := range strings.Split(*dbs, ",") {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "logmine: entry %s\n", path)
		db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
		if err != nil {
			fmt.Fprintf(os.Stderr, "logmine: open %s: %v\n", path, err)
			continue
		}
		m := mine(db)
		_ = db.Close()
		report := renderReport(filepath.Base(path), path, m)
		outPath := filepath.Join(*outDir, sanitizeName(path)+"-logmine.md")
		if err := os.WriteFile(outPath, []byte(report), 0o644); err != nil {
			fail("write %s: %v", outPath, err)
		}
		fmt.Fprintf(os.Stderr, "logmine: exit %s: %d tournament moves, %d casual moves, %d rejects -> %s\n",
			path, len(m.tournament), len(m.casual), m.rejects, outPath)
	}
}
