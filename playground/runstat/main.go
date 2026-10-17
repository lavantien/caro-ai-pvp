package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", os.Args[1])
	if err != nil {
		panic(err)
	}
	defer db.Close()
	rows, err := db.Query(`
	SELECT r.id, r.label, r.status, datetime(r.created_at, 'unixepoch', 'localtime'),
	       (SELECT COUNT(*) FROM tournament_series s WHERE s.run_id = r.id AND s.finished_at IS NOT NULL),
	       (SELECT COUNT(*) FROM tournament_series s WHERE s.run_id = r.id)
	FROM tournament_runs r ORDER BY r.id`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var label, status, createdAt string
		var settled, total int
		if err := rows.Scan(&id, &label, &status, &createdAt, &settled, &total); err != nil {
			panic(err)
		}
		fmt.Printf("run %d label=%s status=%s created=%s series=%d/%d\n", id, label, status, createdAt, settled, total)
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}
}
