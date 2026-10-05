// runstat prints the raw tournament run rows of an evidence db, one-off
// diagnostic for a drive that will not finish.
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
	rows, err := db.Query(`SELECT id, status, finished_at FROM tournament_runs ORDER BY id`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var status string
		var finished sql.NullInt64
		if err := rows.Scan(&id, &status, &finished); err != nil {
			panic(err)
		}
		fmt.Printf("run %d status=%s finished=%v\n", id, status, finished.Int64)
	}
}
