//go:build windows

package main

import (
	"encoding/csv"
	"fmt"
	"strings"
)

func imageMatches(image, comm string) bool {
	return strings.EqualFold(image, comm)
}

func listRunningProcesses() ([]runningProcess, error) {
	out, code, err := execCapture("tasklist", []string{"/fo", "csv", "/nh"}, nil, "")
	if err != nil || code != 0 {
		return nil, fmt.Errorf("tasklist: %v (exit %d)", err, code)
	}
	return parseTasklistCSV(out), nil
}

func parseTasklistCSV(raw string) []runningProcess {
	var procs []runningProcess
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return procs
	}
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		if row[0] == "" || row[1] == "" {
			continue
		}
		procs = append(procs, runningProcess{Image: row[0], PID: row[1]})
	}
	return procs
}
