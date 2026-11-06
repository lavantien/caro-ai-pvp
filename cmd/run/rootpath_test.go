package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRootChildJoinsOneSeparator(t *testing.T) {
	rows := []struct{ root, name, want string }{
		{"repo", ".gotmp", filepath.Join("repo", ".gotmp")},
		{"repo" + string(os.PathSeparator), ".gotmp", filepath.Join("repo", ".gotmp")},
	}
	if runtime.GOOS == "windows" {
		rows = append(rows, struct{ root, name, want string }{"C:\\", "go.mod", "C:\\go.mod"})
	} else {
		rows = append(rows, struct{ root, name, want string }{"/", ".gotmp", "/.gotmp"})
	}
	for _, tc := range rows {
		if got := rootChild(tc.root, tc.name); got != tc.want {
			t.Errorf("rootChild(%q, %q) = %q, want %q", tc.root, tc.name, got, tc.want)
		}
	}
}
