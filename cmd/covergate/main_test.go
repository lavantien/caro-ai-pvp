package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const passFixture = `mode: atomic
github.com/lavantien/caro-ai-pvp/internal/rules/win.go:10.2,11.10 2 1
github.com/lavantien/caro-ai-pvp/internal/rules/board.go:5.2,6.8 3 3
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:3.2,4.9 100 100
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:8.2,9.9 5 0
`

const failOverallFixture = `mode: atomic
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:3.2,4.9 50 30
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:8.2,9.9 10 0
`

const failCoreFixture = `mode: atomic
github.com/lavantien/caro-ai-pvp/internal/rules/win.go:10.2,11.10 4 0
github.com/lavantien/caro-ai-pvp/internal/engine/search.go:3.2,4.9 196 196
`

const absentCoreFixture = `mode: set
github.com/lavantien/caro-ai-pvp/cmd/caro/main.go:3.2,4.9 20 20
github.com/lavantien/caro-ai-pvp/cmd/covergate/main.go:5.2,6.9 30 30
`

func TestParseProfile(t *testing.T) {
	files, err := parseProfile(strings.NewReader(passFixture))
	if err != nil {
		t.Fatalf("parseProfile err = %v", err)
	}
	win := files["github.com/lavantien/caro-ai-pvp/internal/rules/win.go"]
	if win.total != 2 || win.covered != 2 {
		t.Errorf("win.go = %+v, want total 2 covered 2", win)
	}
	main := files["github.com/lavantien/caro-ai-pvp/cmd/caro/main.go"]
	if main.total != 105 || main.covered != 100 {
		t.Errorf("main.go = %+v, want total 105 covered 100", main)
	}
	if len(files) != 3 {
		t.Errorf("len(files) = %d, want 3", len(files))
	}
}

func TestParseProfileErrors(t *testing.T) {
	for name, in := range map[string]string{
		"short line":     "win.go:1.1,2.2 1",
		"bad statements": "win.go:1.1,2.2 x 1",
		"bad count":      "win.go:1.1,2.2 1 x",
		"no range":       "win.go 1 1",
	} {
		if _, err := parseProfile(strings.NewReader(in)); err == nil {
			t.Errorf("%s: parseProfile err = nil, want error", name)
		}
	}
}

func TestGatePass(t *testing.T) {
	if err := gate(mustParse(t, passFixture)); err != nil {
		t.Errorf("gate err = %v, want nil", err)
	}
	if err := gate(mustParse(t, absentCoreFixture)); err != nil {
		t.Errorf("gate without core packages err = %v, want nil", err)
	}
}

func TestGateFailures(t *testing.T) {
	err := gate(mustParse(t, failOverallFixture))
	if err == nil || !strings.Contains(err.Error(), "overall") {
		t.Errorf("gate err = %v, want overall failure", err)
	}
	err = gate(mustParse(t, failCoreFixture))
	if err == nil || !strings.Contains(err.Error(), "internal/rules") {
		t.Errorf("gate err = %v, want internal/rules failure", err)
	}
	err = gate(mustParse(t, "mode: set\n"))
	if err == nil || !strings.Contains(err.Error(), "empty profile") {
		t.Errorf("gate err = %v, want empty profile failure", err)
	}
}

func TestRunCLI(t *testing.T) {
	if code := runCLI(nil); code != 2 {
		t.Errorf("runCLI(nil) = %d, want 2", code)
	}
	if code := runCLI([]string{"a", "b"}); code != 2 {
		t.Errorf("runCLI(two args) = %d, want 2", code)
	}
	if code := runCLI([]string{filepath.Join(t.TempDir(), "missing.out")}); code != 1 {
		t.Errorf("runCLI(missing) = %d, want 1", code)
	}
	if code := runCLI([]string{writeProfile(t, passFixture)}); code != 0 {
		t.Errorf("runCLI(pass) = %d, want 0", code)
	}
	if code := runCLI([]string{writeProfile(t, failOverallFixture)}); code != 1 {
		t.Errorf("runCLI(fail) = %d, want 1", code)
	}
}

func TestPctZeroTotal(t *testing.T) {
	if got := pct(3, 0); got != 0 {
		t.Errorf("pct(3, 0) = %v, want 0", got)
	}
}

func TestCheckParseError(t *testing.T) {
	if err := check(writeProfile(t, "mode: set\nnot a profile line\n")); err == nil {
		t.Error("check(malformed) err = nil, want error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestParseProfileReadError(t *testing.T) {
	if _, err := parseProfile(errReader{}); err == nil || !strings.Contains(err.Error(), "read failed") {
		t.Errorf("parseProfile(errReader) err = %v, want read failure", err)
	}
}

func mustParse(t *testing.T, in string) map[string]fileCov {
	t.Helper()
	files, err := parseProfile(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parseProfile err = %v", err)
	}
	return files
}

func writeProfile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile err = %v", err)
	}
	return path
}
