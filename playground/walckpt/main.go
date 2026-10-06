// walckpt folds each database's write-ahead log into its main file and
// truncates the sidecars, so a database at rest commits as one honest file
// instead of a 4 KiB stub whose real data lives in an ignored -wal sidecar.
// SQLite only folds the log on a clean close, so a process killed mid-run
// leaves the split behind forever until something checkpoints it. Idempotent
// on a database whose log is already folded.
//
// Usage:
//
//	walckpt <dir-or-db>...
//
// A directory argument expands to its *.db files. A checkpoint that reports
// busy (another connection holds the log) fails the run: folding under a
// live writer would tear the record.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: walckpt <dir-or-db>...")
		os.Exit(2)
	}
	failed := false
	for _, arg := range os.Args[1:] {
		dbs := []string{arg}
		if info, err := os.Stat(arg); err == nil && info.IsDir() {
			dbs, err = filepath.Glob(filepath.Join(arg, "*.db"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "walckpt: %s: %v\n", arg, err)
				failed = true
				continue
			}
		}
		for _, db := range dbs {
			if err := checkpoint(db); err != nil {
				fmt.Fprintf(os.Stderr, "walckpt: %s: %v\n", db, err)
				failed = true
			}
		}
	}
	if failed {
		os.Exit(1)
	}
}

// checkpoint runs the truncating checkpoint and reports the folded size.
func checkpoint(path string) error {
	before := sidecarSize(path + "-wal")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	var busy, log, ckpt int
	if err := db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &log, &ckpt); err != nil {
		return err
	}
	fmt.Printf("%s: wal %d -> %d bytes (log frames %d, checkpointed %d, busy %d)\n",
		path, before, sidecarSize(path+"-wal"), log, ckpt, busy)
	if busy != 0 {
		return fmt.Errorf("another connection holds the log: the checkpoint reported busy")
	}
	return nil
}

func sidecarSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
