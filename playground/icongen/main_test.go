package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

func faviconSVG(t *testing.T) string {
	t.Helper()
	_, this, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("icongen: cannot locate the test source")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(this), "..", "..",
		"internal", "server", "web", "static", "favicon.svg"))
	if err != nil {
		t.Fatalf("icongen: read favicon: %v", err)
	}
	return string(data)
}

func hexNRGBA(t *testing.T, s string) (r, g, b uint8) {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		t.Fatalf("icongen: parse %q: %v", s, err)
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v)
}

func TestIconColorsMatchFavicon(t *testing.T) {
	svg := faviconSVG(t)
	rect := regexp.MustCompile(`<rect[^>]*fill="#([0-9a-fA-F]{6})"`).FindStringSubmatch(svg)
	if rect == nil {
		t.Fatal("icongen: favicon misses the background rect fill")
	}
	circles := regexp.MustCompile(`<circle[^>]*fill="#([0-9a-fA-F]{6})"([^>]*)>`).FindAllStringSubmatch(svg, -1)
	if len(circles) != 2 {
		t.Fatalf("icongen: favicon circles = %d, want the red and blue stones", len(circles))
	}

	br, bg, bb := hexNRGBA(t, rect[1])
	if bgColor.R != br || bgColor.G != bg || bgColor.B != bb {
		t.Errorf("icongen: bgColor = %02x%02x%02x, favicon bg = %02x%02x%02x",
			bgColor.R, bgColor.G, bgColor.B, br, bg, bb)
	}
	rr, rg, rb := hexNRGBA(t, circles[0][1])
	if redColor.R != rr || redColor.G != rg || redColor.B != rb {
		t.Errorf("icongen: redColor = %02x%02x%02x, favicon red = %02x%02x%02x",
			redColor.R, redColor.G, redColor.B, rr, rg, rb)
	}
	bir, big, bib := hexNRGBA(t, circles[1][1])
	if blueColor.R != bir || blueColor.G != big || blueColor.B != bib {
		t.Errorf("icongen: blueColor = %02x%02x%02x, favicon blue = %02x%02x%02x",
			blueColor.R, blueColor.G, blueColor.B, bir, big, bib)
	}
	op := regexp.MustCompile(`fill-opacity="([0-9.]+)"`).FindStringSubmatch(circles[1][2])
	if op == nil {
		t.Fatal("icongen: favicon blue circle misses fill-opacity")
	}
	faviconOpacity, err := strconv.ParseFloat(op[1], 64)
	if err != nil {
		t.Fatalf("icongen: parse fill-opacity %q: %v", op[1], err)
	}
	if blueOpacity != faviconOpacity {
		t.Errorf("icongen: blueOpacity = %v, favicon = %v", blueOpacity, faviconOpacity)
	}
}
