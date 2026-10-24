//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const commLimit = 15

func imageMatches(image, comm string) bool {
	name := image
	if len(name) > commLimit {
		name = name[:commLimit]
	}
	return strings.EqualFold(name, comm)
}

func listRunningProcesses() ([]runningProcess, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}
	var procs []runningProcess
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid := e.Name()
		if _, err := parsePid(pid); err != nil {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", pid, "comm"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(comm))
		if name == "" {
			continue
		}
		procs = append(procs, runningProcess{Image: name, PID: pid})
	}
	return procs, nil
}

func parsePid(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
