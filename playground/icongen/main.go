// icongen rasterizes the favicon geometry into the committed PWA icons.
//
// The single source of the brand mark is internal/server/web/static/
// favicon.svg: a 32-unit rounded square of the page background with the red
// and blue stone discs over it, the blue one at 0.9 opacity. This program
// reproduces that geometry analytically (no SVG parser, stdlib only) and
// writes four PNG files under -out:
//
//	icon-192.png, icon-512.png  the favicon geometry verbatim
//	apple-touch-icon.png        180 px, full-bleed square: iOS masks the
//	                             corners itself and pre-rounded corners
//	                             would show whatever sits behind them
//	icon-512-maskable.png       512 px, full-bleed square with the stones
//	                             shrunk onto the 80% maskable safe zone
//
// Run through `make icons`, which points -out at the committed icons dir.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// The favicon's geometry in its 32-unit viewBox, mirrored from favicon.svg.
const (
	viewBox      = 32.0
	cornerRadius = 6.0
	redCX        = 12.5
	blueCX       = 20.5
	stoneCY      = 16.0
	stoneRadius  = 7.0
	blueOpacity  = 0.9
	// The maskable safe zone is the centered 80% circle; the stones shrink
	// onto it by this factor so any launcher mask crop keeps them whole.
	maskableScale = 0.8
	// Supersampling grid per output pixel edge.
	samplesPerEdge = 4
)

// The three favicon fills, from the same committed token values as
// shell.css.
var (
	bgColor   = color.NRGBA{R: 0x10, G: 0x13, B: 0x18, A: 0xff}
	redColor  = color.NRGBA{R: 0xef, G: 0x53, B: 0x50, A: 0xff}
	blueColor = color.NRGBA{R: 0x42, G: 0xa5, B: 0xf5, A: 0xff}
)

// iconSpec is one output file: its pixel size and the two geometry flags.
type iconSpec struct {
	name   string
	size   int
	round  bool // rounded background corners (transparent corners)
	padded bool // stones shrunk onto the maskable safe zone
}

// specs is the committed icon set; names are the URLs the manifest and the
// service worker allowlist pin.
var specs = []iconSpec{
	{name: "icon-192.png", size: 192, round: true},
	{name: "icon-512.png", size: 512, round: true},
	{name: "apple-touch-icon.png", size: 180},
	{name: "icon-512-maskable.png", size: 512, padded: true},
}

func main() {
	out := flag.String("out", "", "output directory for the PNG icons")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "icongen: -out is required")
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "icongen: %v\n", err)
		os.Exit(1)
	}
	for _, spec := range specs {
		path := filepath.Join(*out, spec.name)
		if err := writePNG(path, render(spec)); err != nil {
			fmt.Fprintf(os.Stderr, "icongen: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("icongen: wrote %s (%dx%d)\n", path, spec.size, spec.size)
	}
}

// sample resolves one subpixel's color at the viewBox point (x, y):
// background, then the red disc, then the blue disc over it at its opacity.
func sample(x, y float64, spec iconSpec) color.NRGBA {
	inside := x >= 0 && x <= viewBox && y >= 0 && y <= viewBox
	if spec.round {
		// Rounded-rect membership: the distance from the point to the
		// corner-radius core it clamps onto.
		cx := math.Min(math.Max(x, cornerRadius), viewBox-cornerRadius)
		cy := math.Min(math.Max(y, cornerRadius), viewBox-cornerRadius)
		inside = inside && math.Hypot(x-cx, y-cy) <= cornerRadius
	}
	c := bgColor
	if !inside {
		c.A = 0
		return c
	}
	scale := 1.0
	if spec.padded {
		scale = maskableScale
	}
	stone := func(cx float64) bool {
		px := viewBox/2 + (cx-viewBox/2)*scale
		return math.Hypot(x-px, y-stoneCY) <= stoneRadius*scale
	}
	if stone(redCX) {
		c = redColor
	}
	if stone(blueCX) {
		c = over(blueColor, blueOpacity, c)
	}
	return c
}

// over composites src over dst at alpha a; dst is opaque wherever the
// stones sit, so the straight per-channel blend is exact.
func over(src color.NRGBA, a float64, dst color.NRGBA) color.NRGBA {
	blend := func(s, d uint8) uint8 {
		return uint8(math.Round(a*float64(s) + (1-a)*float64(d)))
	}
	return color.NRGBA{R: blend(src.R, dst.R), G: blend(src.G, dst.G),
		B: blend(src.B, dst.B), A: 0xff}
}

// render rasterizes one spec with straight-alpha supersampling: each output
// pixel averages samplesPerEdge^2 subpixels, so the disc and corner edges
// land smooth without an antialiasing pass.
func render(spec iconSpec) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, spec.size, spec.size))
	pixel := viewBox / float64(spec.size)
	for py := 0; py < spec.size; py++ {
		for px := 0; px < spec.size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < samplesPerEdge; sy++ {
				for sx := 0; sx < samplesPerEdge; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/samplesPerEdge) * pixel
					y := (float64(py) + (float64(sy)+0.5)/samplesPerEdge) * pixel
					c := sample(x, y, spec)
					af := float64(c.A) / 0xff
					r += af * float64(c.R)
					g += af * float64(c.G)
					b += af * float64(c.B)
					a += af
				}
			}
			n := float64(samplesPerEdge * samplesPerEdge)
			i := img.PixOffset(px, py)
			if a == 0 {
				continue
			}
			// Straight alpha: the only transparent texels are the rounded
			// corners, which carry no color to premultiply.
			img.Pix[i] = uint8(math.Round(r / a))
			img.Pix[i+1] = uint8(math.Round(g / a))
			img.Pix[i+2] = uint8(math.Round(b / a))
			img.Pix[i+3] = uint8(math.Round(a / n * 0xff))
		}
	}
	return img
}

// writePNG encodes img to path, creating or truncating it.
func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return f.Close()
}
