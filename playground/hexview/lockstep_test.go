package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var shellCSS = filepath.Join("..", "..", "internal", "server", "web", "shell.css")

func rootColor(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller cannot locate the test source")
	}
	css, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), shellCSS))
	if err != nil {
		t.Fatalf("read shell.css: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s*--` + name + `:\s*(#[0-9a-fA-F]{6})\s*;`)
	m := re.FindSubmatch(css)
	if m == nil {
		t.Fatalf("shell.css :root does not carry a hex --%s anchor", name)
	}
	return string(m[1])
}

func TestPaletteMatchesShellCSS(t *testing.T) {
	for _, c := range []struct {
		name string
		have string
	}{
		{"background", colBackground},
		{"surface", colSurface},
		{"border", colBorder},
		{"text-muted", colTextMuted},
		{"red", colRed},
		{"blue", colBlue},
		{"mark", colMark},
	} {
		want := strings.ToLower(rootColor(t, c.name))
		if have := strings.ToLower(c.have); have != want {
			t.Errorf("%s = %s, shell.css --%s = %s", c.name, have, c.name, want)
		}
	}
}
