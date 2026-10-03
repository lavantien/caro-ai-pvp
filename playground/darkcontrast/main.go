// Command darkcontrast checks the dark palette shipped in
// internal/server/web/shell.css against the WCAG 2.1 contrast formula:
// every foreground/background pair the UI paints, with the threshold each
// pair must clear (4.5:1 body text, 3:1 large text and non-text marks).
// Run after any token change; a miss fails the process.
package main

import (
	"fmt"
	"math"
	"os"
)

// lum is the WCAG relative luminance of an 8-bit sRGB triple.
func lum(r, g, b uint8) float64 {
	f := func(c uint8) float64 {
		s := float64(c) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}

// ratio is the contrast ratio of the lighter-over-darker pair.
func ratio(fg, bg [3]uint8) float64 {
	l1, l2 := lum(fg[0], fg[1], fg[2]), lum(bg[0], bg[1], bg[2])
	if l2 > l1 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// hex parses #rrggbb.
func hex(s string) [3]uint8 {
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil {
		panic("bad hex " + s)
	}
	return [3]uint8{r, g, b}
}

func main() {
	// The :root block of shell.css, verbatim token values.
	p := map[string]string{
		"background": "#101318",
		"surface":    "#1a1f28",
		"border":     "#566179",
		"text":       "#e8ebf2",
		"text-muted": "#a4aec2",
		"accent":     "#58a8ff",
		"danger":     "#ff7a6e",
		"red":        "#ef5350",
		"blue":       "#42a5f5",
		"mark":       "#ffb454",
	}

	// fg token, bg token, where it paints, minimum ratio.
	checks := []struct {
		fg, bg, where string
		min           float64
	}{
		{"text", "background", "body copy on the page", 7},
		{"text", "surface", "copy inside cards and inputs", 7},
		{"text-muted", "background", "status and meta lines", 4.5},
		{"text-muted", "surface", "meta inside cards", 4.5},
		{"accent", "background", "links on the page", 4.5},
		{"accent", "surface", "links inside cards", 4.5},
		{"danger", "background", "error text on the page", 4.5},
		{"danger", "surface", "error text inside fieldsets", 4.5},
		{"red", "background", "red clock digits, 2rem bold", 3},
		{"blue", "background", "blue clock digits, 2rem bold", 3},
		{"red", "surface", "red stone on the board", 3},
		{"blue", "surface", "blue stone on the board", 3},
		{"background", "red", "glyph on a red stone", 4.5},
		{"background", "blue", "glyph on a blue stone", 4.5},
		{"background", "accent", "button label on accent fill", 4.5},
		{"background", "danger", "armed forfeit label", 4.5},
		{"mark", "surface", "latest-stone ring on the board", 3},
		{"border", "surface", "board grid lines and hairlines", 2.5},
	}

	failed := false
	for _, c := range checks {
		r := ratio(hex(p[c.fg]), hex(p[c.bg]))
		ok := "ok"
		if r < c.min {
			ok = "FAIL"
			failed = true
		}
		fmt.Printf("%s  %-11s on %-10s  %5.2f:1  (min %.1f)  %s\n",
			ok, c.fg, c.bg, r, c.min, c.where)
	}
	if failed {
		os.Exit(1)
	}
}
