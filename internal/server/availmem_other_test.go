//go:build !windows

package server

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestParseMemAvailRoundTripAndRejections(t *testing.T) {
	for _, kb := range []uint64{0, 1, 1024, math.MaxUint64 / 1024} {
		input := "MemTotal:       16384000 kB\nMemAvailable:   " +
			strconv.FormatUint(kb, 10) + " kB\nBuffers:          100 kB\n"
		got, err := parseMemAvail([]byte(input))
		if err != nil {
			t.Fatalf("parse %d kB: %v", kb, err)
		}
		if got != kb*1024 {
			t.Errorf("parse %d kB = %d, want %d", kb, got, kb*1024)
		}
	}
	for name, input := range map[string]string{
		"missing":    "MemTotal: 100 kB\n",
		"no unit":    "MemAvailable: 100\n",
		"wrong unit": "MemAvailable: 100 mB\n",
		"extra":      "MemAvailable: 100 kB extra\n",
		"negative":   "MemAvailable: -1 kB\n",
		"overflow":   "MemAvailable: 18446744073709551616 kB\n",
		"text":       "MemAvailable: lots kB\n",
	} {
		if _, err := parseMemAvail([]byte(input)); err == nil {
			t.Errorf("parse %s input succeeded, want the rejection", name)
		}
	}
	got, err := parseMemAvail([]byte("MemTotal: 100 kB\nMemAvailable: 7 kB\nMemAvailable: 9 kB\n"))
	if err != nil {
		t.Fatalf("parse two MemAvailable lines: %v", err)
	}
	if got != 7*1024 {
		t.Errorf("parse two MemAvailable lines = %d, want the first line value %d", got, 7*1024)
	}
}

func FuzzParseMemAvail(f *testing.F) {
	f.Add([]byte("MemAvailable:   123 kB\n"))
	f.Add([]byte("garbage\x00\xff\nMemAvailable: 0 kB"))
	f.Add([]byte(strings.Repeat("MemAvailable: 1 kB\n", 8)))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := parseMemAvail(data)
		if err != nil {
			return
		}
		want := ""
		for _, line := range strings.Split(string(data), "\n") {
			rest, ok := strings.CutPrefix(line, "MemAvailable:")
			if !ok {
				continue
			}
			if fields := strings.Fields(rest); len(fields) > 0 {
				want = fields[0]
			}
			break
		}
		if want != strconv.FormatUint(got/1024, 10) {
			t.Fatalf("parse of %q = %d, want the value from the first MemAvailable line", data, got)
		}
	})
}
