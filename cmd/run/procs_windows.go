//go:build windows

package main

import (
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
