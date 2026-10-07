// Command darkcontrast checks the dark palette shipped in
// internal/server/web/shell.css against the WCAG 2.1 contrast formula:
// every foreground/background pair the UI paints, with the threshold each
// pair must clear (4.5:1 body text, 3:1 large text and non-text marks).
// The tokens parse out of the stylesheet's :root block at run time, so the
// checker reads the shipped values and never a copy that can drift. make
// darkcontrast runs it (and ci runs that); a miss fails the process.
package main

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
)

// shellCSS is the token hub, relative to the playground module the tool
// always runs from (`go -C playground run ./darkcontrast`, its one entry
// point).
const shellCSS = "../internal/server/web/shell.css"

var (
	declRE  = regexp.MustCompile(`^--([a-z0-9-]+):\s*(.+)$`)
	colorRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
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

// rootColors returns the color tokens of shell.css's :root block. The block
// carries non-token properties (color-scheme) and fonts and sizes beside
// the hex anchors; those skip the color map. A --declaration that does not
// parse is an error, and so is a block that is absent or never closes:
// quiet parsing would silently turn the checker back into a hand copy.
func rootColors(path string) (map[string]string, error) {
	css, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src := string(css)
	i := strings.Index(src, ":root {")
	if i < 0 {
		return nil, fmt.Errorf("%s: no :root block", path)
	}
	body := src[i+len(":root {"):]
	j := strings.Index(body, "}")
	if j < 0 {
		return nil, fmt.Errorf("%s: :root block never closes", path)
	}
	colors := make(map[string]string)
	for _, decl := range strings.Split(body[:j], ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		m := declRE.FindStringSubmatch(decl)
		if m == nil {
			if strings.HasPrefix(decl, "--") {
				return nil, fmt.Errorf("%s: unparsable :root declaration %q", path, decl)
			}
			continue
		}
		if v := strings.TrimSpace(m[2]); colorRE.MatchString(v) {
			colors[m[1]] = v
		}
	}
	return colors, nil
}

func main() {
	p, err := rootColors(shellCSS)
	if err != nil {
		fmt.Fprintln(os.Stderr, "darkcontrast:", err)
		os.Exit(1)
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
		{"background", "accent-strong", "button label on the hover fill", 4.5},
		{"background", "danger", "armed forfeit label", 4.5},
		{"text", "accent-soft", "chip label on the accent tint", 4.5},
		{"mark", "surface", "latest-stone ring on the board", 3},
		{"mark", "background", "live-tournament line text", 3},
		{"accent-strong", "background", "focus ring over the page", 3},
		{"text-muted", "background", "chip state boundary on the page", 3},
		{"text-muted", "surface", "chip state boundary inside cards", 3},
		{"text-muted", "accent-soft", "live chip state boundary", 3},
		{"border", "surface", "board grid lines and hairlines", 2.5},
	}

	failed := false
	for _, c := range checks {
		fg, bg := p[c.fg], p[c.bg]
		if fg == "" || bg == "" {
			fmt.Printf("FAIL  %-11s on %-10s  token missing from %s :root\n",
				c.fg, c.bg, shellCSS)
			failed = true
			continue
		}
		r := ratio(hex(fg), hex(bg))
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
