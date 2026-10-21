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
