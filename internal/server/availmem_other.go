//go:build !windows

package server

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func hostAvailMem() (uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	return parseMemAvail(data)
}

func parseMemAvail(data []byte) (uint64, error) {
	for _, line := range strings.Split(string(data), "\n") {
		value, ok := strings.CutPrefix(line, "MemAvailable:")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) != 2 || fields[1] != "kB" {
			return 0, fmt.Errorf("parse MemAvailable %q", line)
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse MemAvailable %q: %w", line, err)
		}
		return kb * 1024, nil
	}
	return 0, errors.New("/proc/meminfo carries no MemAvailable line")
}
