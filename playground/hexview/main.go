package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func formatPoints(pts [6][2]float64) string {
	parts := make([]string, len(pts))
	for i, p := range pts {
		parts[i] = fmt.Sprintf("%.2f,%.2f", p[0], p[1])
	}
	return strings.Join(parts, " ")
}

func renderSVG(m boardModel) string {
	var b strings.Builder
	w := m.ViewBox[2] - m.ViewBox[0]
	h := m.ViewBox[3] - m.ViewBox[1]
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"%.2f %.2f %.2f %.2f\" role=\"img\" aria-label=\"hex variant board\">\n", m.ViewBox[0], m.ViewBox[1], w, h)
	b.WriteString("<g>\n")
	for _, c := range m.Cells {
		fmt.Fprintf(&b, "<polygon class=\"cell\" id=\"c-%s\" data-cell=\"%s\" points=\"%s\" fill=\"%s\" stroke=\"%s\" stroke-width=\"%.2f\"/>\n",
			c.Name, c.Name, formatPoints(c.Points), colSurface, colBorder, strokeCellW)
	}
	b.WriteString("</g>\n")
	fmt.Fprintf(&b, "<g font-family=\"system-ui, sans-serif\" font-size=\"%.2f\" fill=\"%s\">\n", labelSize, colTextMuted)
	for _, l := range m.ColLabels {
		fmt.Fprintf(&b, "<text x=\"%.2f\" y=\"%.2f\" text-anchor=\"%s\">%s</text>\n", l.X, l.Y, l.Anchor, l.Text)
	}
	for _, l := range m.RowLabels {
		fmt.Fprintf(&b, "<text x=\"%.2f\" y=\"%.2f\" text-anchor=\"%s\">%s</text>\n", l.X, l.Y, l.Anchor, l.Text)
	}
	b.WriteString("</g>\n")
	b.WriteString("<g>\n")
	for _, s := range m.Stones {
		fmt.Fprintf(&b, "<circle class=\"stone %s\" cx=\"%.2f\" cy=\"%.2f\" r=\"%.2f\" fill=\"%s\"/>\n", stoneClass(s.Fill), s.X, s.Y, stoneR, s.Fill)
	}
	for _, c := range m.Cells {
		if !latestStone(m, c.Name) {
			continue
		}
		fmt.Fprintf(&b, "<polygon class=\"latest-ring\" points=\"%s\" fill=\"none\" stroke=\"%s\" stroke-width=\"%.2f\"/>\n", formatPoints(c.Points), colMark, latestRingW)
	}
	b.WriteString("</g>\n")
	b.WriteString("</svg>\n")
	return b.String()
}

func latestStone(m boardModel, name string) bool {
	for _, s := range m.Stones {
		if s.Latest && s.Name == name {
			return true
		}
	}
	return false
}

func stoneClass(fill string) string {
	if fill == colRed {
		return "red"
	}
	return "blue"
}

func wrapHTML(svg string) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>hex variant board prototype</title>\n<style>\n")
	b.WriteString(":root { color-scheme: light }\n")
	fmt.Fprintf(&b, "body { margin: 0; min-height: 100vh; background: %s; display: flex; flex-direction: column; align-items: center; gap: .75rem; padding: 1rem 0; font-family: system-ui, sans-serif }\n", colBackground)
	fmt.Fprintf(&b, "h1 { font-size: .9rem; font-weight: 400; color: %s; margin: 0 }\n", colTextMuted)
	b.WriteString("figure { margin: 0; width: min(100%, 44rem); padding: 0 1rem; box-sizing: border-box }\n")
	b.WriteString("svg { display: block; width: 100%; height: auto }\n")
	b.WriteString("</style>\n</head>\n<body>\n")
	b.WriteString("<h1>hex variant, side 12, 397 cells</h1>\n<figure>\n")
	b.WriteString(svg)
	b.WriteString("</figure>\n</body>\n</html>\n")
	return b.String()
}

func main() {
	log.SetPrefix("hexview: ")
	log.SetFlags(log.LstdFlags)
	log.Printf("start")
	outDir := flag.String("out", "out", "output directory")
	flag.Parse()
	m := buildBoard()
	svg := renderSVG(m)
	html := wrapHTML(svg)
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *outDir, err)
	}
	svgPath := filepath.Join(*outDir, "board.svg")
	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
		log.Fatalf("write %s: %v", svgPath, err)
	}
	log.Printf("wrote %s (%d bytes, %d cells)", svgPath, len(svg), len(m.Cells))
	htmlPath := filepath.Join(*outDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		log.Fatalf("write %s: %v", htmlPath, err)
	}
	log.Printf("wrote %s (%d bytes)", htmlPath, len(html))
	log.Printf("done")
}
