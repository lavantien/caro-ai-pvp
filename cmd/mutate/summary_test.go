package main

import (
	"errors"
	"strings"
	"testing"
)

func TestSummaryLineSuppressedOnErrorAndEmptyTotals(t *testing.T) {
	t.Parallel()
	res := result{run: 1384, total: 2763, killed: 1324, survived: 18, allowed: 35}
	if s := summaryLine(res, errors.New("isolate vanished")); s != "" {
		t.Fatalf("summaryLine on infra error = %q, want empty so crash detectors keep resuming", s)
	}
	if s := summaryLine(result{run: 3, total: 0}, nil); s != "" {
		t.Fatalf("summaryLine on zero total = %q, want empty", s)
	}
	s := summaryLine(res, nil)
	if !strings.HasPrefix(s, "mutate: 1384/2763 run, 1324 killed, 18 survived, 35 allowed\n") {
		t.Fatalf("summaryLine = %q, want the verdict line", s)
	}
}
