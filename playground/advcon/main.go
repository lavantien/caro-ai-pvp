package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func section(name string) { fmt.Printf("\n== %s ==\n", name) }

func scanTC(line string) {
	var initial, inc, bo int
	n, err := fmt.Sscanf(line, "tc %d+%d bo%d", &initial, &inc, &bo)
	fmt.Printf("tcLine %-28q -> n=%d err=%v initial=%d inc=%d bo=%d\n", line, n, err, initial, inc, bo)
}

func codecRow(hex *geom.Geometry, name string) {
	hexFlat, hexErr := hex.ParseCell(name)
	sqFlat, sqErr := rules.ParseCell(name)
	fmt.Printf("cell %-5q square=%v(%d) hex=%v(%d)\n", name,
		sqErr == nil, flatOf(int(sqFlat), sqErr), hexErr == nil, hexFlat)
}

func flatOf(c int, err error) int {
	if err != nil {
		return -1
	}
	return c
}

func decode(label string, moves []rules.Move, g *geom.Geometry) {
	blob := server.EncodeMoves(nil, moves)
	got, err := server.DecodeMoves(blob, g)
	fmt.Printf("decode %-42s cells=%3d -> err=%v moves=%v\n", label, g.Cells, err, got)
}

func rawOpen(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		panic(err)
	}
	return db
}

func main() {
	hex := geom.Hex(12)

	section("A tc line trailing token")
	scanTC("tc 1+0 bo3")
	scanTC("tc 1+0 bo3 hex")
	scanTC("tc 1+0 bo3 hexagon")
	scanTC("tc 1+0 bo3  hex")
	scanTC("tc 1+0 bo3 hex outcome red")
	scanTC("tc 1+0 bo3 square")

	section("B cell codec square vs hex")
	for _, n := range []string{"L12", "A12", "W12", "L1", "L23", "P16", "Q16", "W17", "A1", "M12", "I17"} {
		codecRow(hex, n)
	}

	section("C decode bounds")
	hexCenter, _ := hex.ParseCell("L12")
	sqL12, _ := rules.ParseCell("L12")
	sq := geom.Default()
	decode("hex center L12 under square bound", []rules.Move{rules.Move(hexCenter)}, sq)
	decode("hex center L12 under hex bound", []rules.Move{rules.Move(hexCenter)}, hex)
	decode("square L12 under hex bound", []rules.Move{rules.Move(sqL12)}, hex)
	decode("flat 0 hex off-region under hex bound", []rules.Move{0}, hex)
	decode("flat 517 hex max in-region under hex bound", []rules.Move{517}, hex)
	decode("flat 528 beyond hex cells under hex bound", []rules.Move{528}, hex)

	section("D hex id space")
	names := map[string]bool{}
	playable := 0
	maxFlat := -1
	for flat := range hex.Cells {
		if !hex.Contains(flat) {
			continue
		}
		playable++
		maxFlat = max(maxFlat, flat)
		name := string(hex.AppendName(nil, flat))
		if names[name] {
			fmt.Printf("DUPLICATE name %q\n", name)
		}
		names[name] = true
		back, err := hex.ParseCell(name)
		if err != nil || int(back) != flat {
			fmt.Printf("ROUNDTRIP fail flat %d name %q -> %d %v\n", flat, name, back, err)
		}
	}
	offRegionNamed := 0
	for _, n := range []string{"A1", "A23", "W1", "W23", "A12", "W12"} {
		if _, err := hex.ParseCell(n); err == nil {
			offRegionNamed++
			fmt.Printf("hex corner %q ACCEPTED, want off-region reject\n", n)
		}
	}
	fmt.Printf("playable=%d uniqueNames=%d maxFlat=%d cornersRejected=%d/6\n", playable, len(names), maxFlat, 6-offRegionNamed)

	section("E v11 to v12 upgrade with rows")
	dir, err := os.MkdirTemp("", "advcon-*")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	dbPath := filepath.Join(dir, "caro.db")
	st, err := server.Open(dbPath)
	if err != nil {
		panic(err)
	}
	bot, err := st.BotAccountID(config.TierEasy)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	for range 3 {
		sr, serr := st.CreateSeriesFor(ctx, 0, config.SeriesBO3, bot, bot+1, "")
		if serr != nil {
			panic(serr)
		}
		winner := bot
		if uerr := st.UpdateSeries(sr.ID, "finished", &winner, nil); uerr != nil {
			panic(uerr)
		}
	}
	if err := st.Close(); err != nil {
		panic(err)
	}
	raw := rawOpen(dbPath)
	for _, q := range []string{
		`ALTER TABLE series DROP COLUMN variant`,
		`ALTER TABLE tournament_runs DROP COLUMN variant`,
		`INSERT INTO tournament_runs (tc_idx, bo_len, start_rating, status) VALUES (0, 3, 100, 'ongoing')`,
		`DELETE FROM schema_version WHERE version = 12`,
	} {
		if _, qerr := raw.Exec(q); qerr != nil {
			fmt.Printf("craft step %q: %v\n", q, qerr)
		}
	}
	if err := raw.Close(); err != nil {
		panic(err)
	}
	if st2, rerr := server.Open(dbPath); rerr != nil {
		fmt.Printf("reopen after v11 craft: %v\n", rerr)
	} else {
		if cerr := st2.Close(); cerr != nil {
			panic(cerr)
		}
		raw2 := rawOpen(dbPath)
		var rows, sq int
		var hx sql.NullInt64
		if qerr := raw2.QueryRow(`SELECT COUNT(*), SUM(variant = 'square'), SUM(variant = 'hex') FROM series`).Scan(&rows, &sq, &hx); qerr != nil {
			fmt.Printf("post-upgrade series read: %v\n", qerr)
		} else {
			fmt.Printf("after upgrade: series rows=%d square=%d hex=%d\n", rows, sq, hx.Int64)
		}
		var runs, runSq int
		var runHx sql.NullInt64
		if qerr := raw2.QueryRow(`SELECT COUNT(*), SUM(variant = 'square'), SUM(variant = 'hex') FROM tournament_runs`).Scan(&runs, &runSq, &runHx); qerr != nil {
			fmt.Printf("post-upgrade runs read: %v\n", qerr)
		} else {
			fmt.Printf("after upgrade: runs rows=%d square=%d hex=%d\n", runs, runSq, runHx.Int64)
		}
		var ver int
		if qerr := raw2.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&ver); qerr == nil {
			fmt.Printf("schema_version max=%d\n", ver)
		}
		if _, derr := raw2.Exec(`DELETE FROM schema_version WHERE version = 12`); derr != nil {
			fmt.Printf("ledger rollback craft: %v\n", derr)
		}
		if err := raw2.Close(); err != nil {
			panic(err)
		}
		st3, rerr := server.Open(dbPath)
		if rerr != nil {
			fmt.Printf("reopen with columns present at v11: %v\n", rerr)
		} else {
			fmt.Printf("reopen with columns present at v11: NO ERROR\n")
			if err := st3.Close(); err != nil {
				panic(err)
			}
		}
	}
}
