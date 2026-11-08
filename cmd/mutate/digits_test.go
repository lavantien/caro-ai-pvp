package main

import (
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestIsDigitsClassifiesTokens(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"0", true},
		{"007", true},
		{"12a", false},
		{" 1", false},
		{"-1", false},
	}
	for _, tc := range cases {
		if got := isDigits(tc.in); got != tc.want {
			t.Fatalf("isDigits(%q) = %t, want %t", tc.in, got, tc.want)
		}
	}
}

func TestOutputTailTruncatesLongOutput(t *testing.T) {
	if got := outputTail("short"); got != "short" {
		t.Fatalf("outputTail(short) = %q", got)
	}
	long := strings.Repeat("x", config.MutateRunErrTailBytes+50)
	got := outputTail(long)
	if !strings.HasPrefix(got, "...") || len(got) != config.MutateRunErrTailBytes+3 {
		t.Fatalf("outputTail truncation = prefix %q len %d, want ... prefix and %d kept bytes", got[:6], len(got), config.MutateRunErrTailBytes)
	}
}
