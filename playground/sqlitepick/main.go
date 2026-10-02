// Command sqlitepick measures the M6a SQLite driver candidates under the
// workload the server will run: WAL mode, a single writer goroutine batching
// row inserts in transactions, concurrent indexed point-lookup readers, cold open
// cost, and a coarse peak memory proxy.
//
// Each candidate runs in its own child process (the -all mode re-execs this
// binary per driver) so runtime.MemStats.Sys stays attributable to exactly one
// driver. Every candidate gets the same schema, statements, pragmas, and pool
// shape. Run with CGO_ENABLED=1: mattn needs it, and modernc (CGO-free) must
// still build in the environment the project ships.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// Single config hub for every tunable. All candidates read only these values.
const (
	totalRows   = 1_000_000 // rows written by the insert test, and again by the concurrent test
	rowsPerTx   = 100       // inserted rows per transaction
	numMatches  = 2000      // pre-seeded matches that moves rows reference
	numReaders  = 4         // reader goroutines during the concurrent test
	openIters   = 10        // cold-open samples, median reported
	slowReadMs  = 10        // reads slower than this count as blocked-behind-writer
	busyTimeout = 5 * time.Second
)

const (
	mattnName   = "mattn"
	moderncName = "modernc"
	zzName      = "zombiezen"
)

// Identical SQL for every candidate.
var (
	schemaStmts = []string{
		`CREATE TABLE matches (
			id INTEGER PRIMARY KEY,
			created_ts INTEGER NOT NULL,
			p1 TEXT NOT NULL,
			p2 TEXT NOT NULL
		)`,
		`CREATE TABLE moves (
			match_id INTEGER NOT NULL REFERENCES matches(id),
			ply INTEGER NOT NULL,
			side INTEGER NOT NULL,
			cell INTEGER NOT NULL,
			ts INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_moves_match_ply ON moves(match_id, ply)`,
	}
	insertMatchSQL = `INSERT INTO matches (id, created_ts, p1, p2) VALUES (?, ?, ?, ?)`
	insertMoveSQL  = `INSERT INTO moves (match_id, ply, side, cell, ts) VALUES (?, ?, ?, ?, ?)`
	lookupSQL      = `SELECT side, cell, ts FROM moves WHERE match_id = ? AND ply = ?`
	countSQL       = `SELECT count(*) FROM matches`
	beginSQL       = `BEGIN`
	commitSQL      = `COMMIT`
)

// mattn and modernc share the mattn-style DSN shorthand syntax, so one string
// configures both identically.
func dsn(path string) string {
	return filepath.ToSlash(path) + "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=on"
}

type result struct {
	Driver         string  `json:"driver"`
	InsRowsPerSec  float64 `json:"ins_rows_per_sec"`
	InsSeconds     float64 `json:"ins_seconds"`
	ConcWriterRPS  float64 `json:"conc_writer_rows_per_sec"`
	ConcWriterSecs float64 `json:"conc_writer_seconds"`
	ReaderQPS      float64 `json:"reader_qps"`
	ReaderHits     int64   `json:"reader_hits"`
	SlowReads      int64   `json:"slow_reads"`
	MaxReadMs      float64 `json:"max_read_ms"`
	ReadErrors     int64   `json:"read_errors"`
	OpenMedianMs   float64 `json:"open_median_ms"`
	PeakSysMB      float64 `json:"peak_sys_mb"`
}

type readStats struct {
	count atomic.Int64
	hits  atomic.Int64
	slow  atomic.Int64
	errs  atomic.Int64
	maxNs atomic.Int64
}

func (s *readStats) record(start time.Time, hit bool, err error) {
	if err != nil {
		s.errs.Add(1)
		return
	}
	elapsed := time.Since(start)
	s.count.Add(1)
	if hit {
		s.hits.Add(1)
	}
	if elapsed.Milliseconds() >= slowReadMs {
		s.slow.Add(1)
	}
	for {
		old := s.maxNs.Load()
		if int64(elapsed) <= old || s.maxNs.CompareAndSwap(old, int64(elapsed)) {
			break
		}
	}
}

type candidate interface {
	setup(path string) error
	openOnce(path string) (time.Duration, error)
	openWriter(path string) error
	openReaders(path string, maxPly int) error
	writeRows(rows int) error
	readLoop(done <-chan struct{}, wg *sync.WaitGroup, stats *readStats)
	closeAll()
}

func main() {
	defaultRows := totalRows
	all := flag.Bool("all", false, "run every candidate in a child process, then print the summary table")
	driver := flag.String("driver", "", "candidate to run in this process: mattn|modernc|zombiezen")
	rows := flag.Int("rows", defaultRows, "rows to write per measurement phase")
	jsonOut := flag.Bool("json", false, "print the result as JSON on stdout (child mode)")
	flag.Parse()

	if *all {
		runAll(*rows)
		return
	}

	names := map[string]bool{mattnName: true, moderncName: true, zzName: true}
	if !names[*driver] {
		fmt.Fprintf(os.Stderr, "unknown or missing -driver %q\n", *driver)
		os.Exit(2)
	}
	res, err := runOne(*driver, *rows)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", *driver, err)
		os.Exit(1)
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(os.Stderr, "%s: encode result: %v\n", *driver, err)
			os.Exit(1)
		}
	} else {
		printTable(os.Stdout, []result{*res}, *rows)
	}
}

func runAll(rows int) {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate self: %v\n", err)
		os.Exit(1)
	}
	results := make([]result, 0, 3)
	for _, name := range []string{mattnName, moderncName, zzName} {
		fmt.Fprintf(os.Stderr, "== running %s (%d rows per phase) ==\n", name, rows)
		cmd := exec.Command(self, "-driver", name, "-rows", strconv.Itoa(rows), "-json")
		cmd.Stderr = os.Stderr
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "%s child failed: %v\n", name, err)
			os.Exit(1)
		}
		var res result
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			fmt.Fprintf(os.Stderr, "%s child output: %v\n", name, err)
			os.Exit(1)
		}
		results = append(results, res)
	}
	printTable(os.Stdout, results, rows)
}

func printTable(w *os.File, results []result, rows int) {
	fmt.Fprintf(w, "sqlitepick  go=%s rows/phase=%d readers=%d tx=%d rows  CGO_ENABLED=1\n",
		runtime.Version(), rows, numReaders, rowsPerTx)
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "driver\tins rows/s\tins s\tconc wr rows/s\treader qps\tslow>10ms\tmax rd ms\topen med ms\tSys MB")
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%.0f\t%.1f\t%.0f\t%.0f\t%d\t%.1f\t%.1f\t%.1f\n",
			r.Driver, r.InsRowsPerSec, r.InsSeconds, r.ConcWriterRPS, r.ReaderQPS,
			r.SlowReads, r.MaxReadMs, r.OpenMedianMs, r.PeakSysMB)
	}
	tw.Flush()
}

func runOne(name string, rows int) (*result, error) {
	dir, err := os.MkdirTemp("", "sqlitepick-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	var c candidate
	switch name {
	case mattnName:
		c = &sqlBench{driver: "sqlite3"}
	case moderncName:
		c = &sqlBench{driver: "sqlite"}
	case zzName:
		c = &zzBench{}
	default:
		return nil, fmt.Errorf("unknown candidate %q", name)
	}

	insertDB := filepath.Join(dir, "insert.db")
	concDB := filepath.Join(dir, "conc.db")
	if err := c.setup(insertDB); err != nil {
		return nil, fmt.Errorf("setup insert db: %w", err)
	}

	// Startup cost: cold open of an existing WAL database, median of N.
	opens := make([]time.Duration, 0, openIters)
	for range openIters {
		d, err := c.openOnce(insertDB)
		if err != nil {
			return nil, fmt.Errorf("openOnce: %w", err)
		}
		opens = append(opens, d)
	}
	sort.Slice(opens, func(i, j int) bool { return opens[i] < opens[j] })
	openMedian := opens[len(opens)/2]

	// Phase 1: single-writer batched INSERT throughput.
	if err := c.openWriter(insertDB); err != nil {
		return nil, fmt.Errorf("open writer (insert phase): %w", err)
	}
	progress(name, "insert phase: writing %d rows", rows)
	start := time.Now()
	if err := c.writeRows(rows); err != nil {
		return nil, fmt.Errorf("insert phase: %w", err)
	}
	insElapsed := time.Since(start)
	c.closeAll()

	// Phase 2: writer plus concurrent readers on a fresh database.
	if err := c.setup(concDB); err != nil {
		return nil, fmt.Errorf("setup concurrent db: %w", err)
	}
	if err := c.openWriter(concDB); err != nil {
		return nil, fmt.Errorf("open writer (concurrent phase): %w", err)
	}
	if err := c.openReaders(concDB, max(1, rows/numMatches+1)); err != nil {
		return nil, fmt.Errorf("open readers: %w", err)
	}
	stats := &readStats{}
	done := make(chan struct{})
	var wg sync.WaitGroup
	for range numReaders {
		wg.Add(1)
		go c.readLoop(done, &wg, stats)
	}
	progress(name, "concurrent phase: writing %d rows under %d readers", rows, numReaders)
	start = time.Now()
	if err := c.writeRows(rows); err != nil {
		return nil, fmt.Errorf("concurrent phase: %w", err)
	}
	concElapsed := time.Since(start)
	close(done)
	wg.Wait()
	c.closeAll()

	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	return &result{
		Driver:         name,
		InsRowsPerSec:  float64(rows) / insElapsed.Seconds(),
		InsSeconds:     insElapsed.Seconds(),
		ConcWriterRPS:  float64(rows) / concElapsed.Seconds(),
		ConcWriterSecs: concElapsed.Seconds(),
		ReaderQPS:      float64(stats.count.Load()) / concElapsed.Seconds(),
		ReaderHits:     stats.hits.Load(),
		SlowReads:      stats.slow.Load(),
		MaxReadMs:      float64(stats.maxNs.Load()) / float64(time.Millisecond),
		ReadErrors:     stats.errs.Load(),
		OpenMedianMs:   float64(openMedian) / float64(time.Millisecond),
		PeakSysMB:      float64(ms.Sys) / (1 << 20),
	}, nil
}

func progress(name, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", name, fmt.Sprintf(format, args...))
}

// sqlBench measures a database/sql driver: mattn (CGO) and modernc (pure Go).
//
// Pool shape mirrors the planned M6a server: writes funnel through the custom
// message/worker queue onto exactly one connection, so the writer pool is
// capped at 1 and reads use a separate pool sized to the reader goroutines.
type sqlBench struct {
	driver  string
	writeDB *sql.DB
	readDB  *sql.DB
	maxPly  int64
}

func (b *sqlBench) setup(path string) error {
	db, err := sql.Open(b.driver, dsn(path))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return initDB(db)
}

func initDB(db *sql.DB) error {
	if err := db.Ping(); err != nil {
		return err
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		return err
	}
	if mode != "wal" {
		return fmt.Errorf("journal_mode = %q, want wal", mode)
	}
	for _, stmt := range schemaStmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%q: %w", stmt, err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(insertMatchSQL)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i := range int64(numMatches) {
		if _, err := stmt.Exec(i, i, "p1-"+strconv.FormatInt(i, 10), "p2-"+strconv.FormatInt(i, 10)); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (b *sqlBench) openOnce(path string) (time.Duration, error) {
	start := time.Now()
	db, err := sql.Open(b.driver, dsn(path))
	if err != nil {
		return 0, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		return 0, err
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		return 0, err
	}
	if mode != "wal" {
		return 0, fmt.Errorf("journal_mode = %q, want wal", mode)
	}
	var n int
	if err := db.QueryRow(countSQL).Scan(&n); err != nil {
		return 0, err
	}
	if n != numMatches {
		return 0, fmt.Errorf("matches = %d, want %d", n, numMatches)
	}
	return time.Since(start), nil
}

func (b *sqlBench) openWriter(path string) error {
	db, err := sql.Open(b.driver, dsn(path))
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	b.writeDB = db
	return db.Ping()
}

func (b *sqlBench) openReaders(path string, maxPly int) error {
	b.maxPly = int64(maxPly)
	db, err := sql.Open(b.driver, dsn(path))
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(numReaders)
	db.SetMaxIdleConns(numReaders)
	db.SetConnMaxLifetime(0)
	b.readDB = db
	return db.Ping()
}

func (b *sqlBench) writeRows(rows int) error {
	ctx := context.Background()
	for base := 0; base < rows; base += rowsPerTx {
		tx, err := b.writeDB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		stmt, err := tx.Prepare(insertMoveSQL)
		if err != nil {
			tx.Rollback()
			return err
		}
		for j := 0; j < rowsPerTx; j++ {
			i := base + j
			if _, err := stmt.Exec(i%numMatches, i/numMatches, i%2, i%361, i); err != nil {
				tx.Rollback()
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (b *sqlBench) readLoop(done <-chan struct{}, wg *sync.WaitGroup, stats *readStats) {
	defer wg.Done()
	stmt, err := b.readDB.Prepare(lookupSQL)
	if err != nil {
		stats.record(time.Now(), false, err)
		return
	}
	defer stmt.Close()
	var side, cell, ts int64
	for {
		select {
		case <-done:
			return
		default:
		}
		m := rand.Int64N(numMatches)
		p := rand.Int64N(b.maxPly)
		start := time.Now()
		err := stmt.QueryRow(m, p).Scan(&side, &cell, &ts)
		hit := err == nil
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		}
		stats.record(start, hit, err)
	}
}

func (b *sqlBench) closeAll() {
	if b.readDB != nil {
		b.readDB.Close()
		b.readDB = nil
	}
	if b.writeDB != nil {
		b.writeDB.Close()
		b.writeDB = nil
	}
}

// zzBench measures zombiezen.com/go/sqlite, the maintained crawshaw-style
// low-level API on top of modernc.org/sqlite. Same pool shape: one dedicated
// writer connection plus a 4-connection reader pool.
type zzBench struct {
	writeConn *sqlite.Conn
	pool      *sqlitex.Pool
	maxPly    int64
}

func zzOpen(path string) (*sqlite.Conn, error) {
	conn, err := sqlite.OpenConn(path, sqlite.OpenReadWrite, sqlite.OpenCreate, sqlite.OpenURI)
	if err != nil {
		return nil, err
	}
	conn.SetBusyTimeout(busyTimeout)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
	} {
		if err := sqlitex.Execute(conn, pragma, nil); err != nil {
			conn.Close()
			return nil, fmt.Errorf("%q: %w", pragma, err)
		}
	}
	return conn, nil
}

func zzInit(conn *sqlite.Conn) error {
	var mode string
	err := sqlitex.Execute(conn, "PRAGMA journal_mode", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			mode = stmt.ColumnText(0)
			return nil
		},
	})
	if err != nil {
		return err
	}
	if mode != "wal" {
		return fmt.Errorf("journal_mode = %q, want wal", mode)
	}
	for _, stmt := range schemaStmts {
		if err := sqlitex.Execute(conn, stmt, nil); err != nil {
			return fmt.Errorf("%q: %w", stmt, err)
		}
	}
	if err := sqlitex.Execute(conn, beginSQL, nil); err != nil {
		return err
	}
	for i := range int64(numMatches) {
		if err := sqlitex.Execute(conn, insertMatchSQL, &sqlitex.ExecOptions{
			Args: []any{i, i, "p1-" + strconv.FormatInt(i, 10), "p2-" + strconv.FormatInt(i, 10)},
		}); err != nil {
			return err
		}
	}
	return sqlitex.Execute(conn, commitSQL, nil)
}

func (b *zzBench) setup(path string) error {
	conn, err := zzOpen(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	return zzInit(conn)
}

func (b *zzBench) openOnce(path string) (time.Duration, error) {
	start := time.Now()
	conn, err := zzOpen(path)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	var n int
	err = sqlitex.Execute(conn, countSQL, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			n = stmt.ColumnInt(0)
			return nil
		},
	})
	if err != nil {
		return 0, err
	}
	if n != numMatches {
		return 0, fmt.Errorf("matches = %d, want %d", n, numMatches)
	}
	return time.Since(start), nil
}

func (b *zzBench) openWriter(path string) error {
	conn, err := zzOpen(path)
	if err != nil {
		return err
	}
	b.writeConn = conn
	return nil
}

func (b *zzBench) openReaders(path string, maxPly int) error {
	b.maxPly = int64(maxPly)
	pool, err := sqlitex.NewPool(path, sqlitex.PoolOptions{
		PoolSize: numReaders,
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenURI,
		PrepareConn: func(conn *sqlite.Conn) error {
			conn.SetBusyTimeout(busyTimeout)
			for _, pragma := range []string{"PRAGMA synchronous=NORMAL", "PRAGMA foreign_keys=ON"} {
				if err := sqlitex.Execute(conn, pragma, nil); err != nil {
					return err
				}
			}
			return nil
		},
	})
	if err != nil {
		return err
	}
	b.pool = pool
	return nil
}

func (b *zzBench) writeRows(rows int) error {
	for base := 0; base < rows; base += rowsPerTx {
		if err := sqlitex.Execute(b.writeConn, beginSQL, nil); err != nil {
			return err
		}
		for j := 0; j < rowsPerTx; j++ {
			i := base + j
			if err := sqlitex.Execute(b.writeConn, insertMoveSQL, &sqlitex.ExecOptions{
				Args: []any{i % numMatches, i / numMatches, i % 2, i % 361, i},
			}); err != nil {
				return err
			}
		}
		if err := sqlitex.Execute(b.writeConn, commitSQL, nil); err != nil {
			return err
		}
	}
	return nil
}

func (b *zzBench) readLoop(done <-chan struct{}, wg *sync.WaitGroup, stats *readStats) {
	defer wg.Done()
	for {
		select {
		case <-done:
			return
		default:
		}
		m := rand.Int64N(numMatches)
		p := rand.Int64N(b.maxPly)
		start := time.Now()
		conn, err := b.pool.Take(context.Background())
		if err != nil {
			stats.record(start, false, err)
			return
		}
		hit := false
		err = sqlitex.Execute(conn, lookupSQL, &sqlitex.ExecOptions{
			Args: []any{m, p},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				hit = true
				_, _, _ = stmt.ColumnInt64(0), stmt.ColumnInt64(1), stmt.ColumnInt64(2)
				return nil
			},
		})
		b.pool.Put(conn)
		stats.record(start, hit, err)
	}
}

func (b *zzBench) closeAll() {
	if b.pool != nil {
		b.pool.Close()
		b.pool = nil
	}
	if b.writeConn != nil {
		b.writeConn.Close()
		b.writeConn = nil
	}
}
