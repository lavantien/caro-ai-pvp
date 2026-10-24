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

const (
	viewBox        = 32.0
	cornerRadius   = 6.0
	redCX          = 12.5
	blueCX         = 20.5
	stoneCY        = 16.0
	stoneRadius    = 7.0
	blueOpacity    = 0.9
	maskableScale  = 0.8
	samplesPerEdge = 4
)

var (
	bgColor   = color.NRGBA{R: 0xf9, G: 0xe8, B: 0xf1, A: 0xff}
	redColor  = color.NRGBA{R: 0xc2, G: 0x18, B: 0x5b, A: 0xff}
	blueColor = color.NRGBA{R: 0x15, G: 0x65, B: 0xc0, A: 0xff}
)

type iconSpec struct {
	name   string
	size   int
	round  bool
	padded bool
}

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
		fmt.Fprintf(os.Stderr, "icongen: %v", err)
		os.Exit(1)
	}
	for _, spec := range specs {
		path := filepath.Join(*out, spec.name)
		if err := writePNG(path, render(spec)); err != nil {
			fmt.Fprintf(os.Stderr, "icongen: %v", err)
			os.Exit(1)
		}
		fmt.Printf("icongen: wrote %s (%dx%d)\n", path, spec.size, spec.size)
	}
}

func sample(x, y float64, spec iconSpec) color.NRGBA {
	inside := x >= 0 && x <= viewBox && y >= 0 && y <= viewBox
	if spec.round {
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

func over(src color.NRGBA, a float64, dst color.NRGBA) color.NRGBA {
	blend := func(s, d uint8) uint8 {
		return uint8(math.Round(a*float64(s) + (1-a)*float64(d)))
	}
	return color.NRGBA{R: blend(src.R, dst.R), G: blend(src.G, dst.G),
		B: blend(src.B, dst.B), A: 0xff}
}

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
			img.Pix[i] = uint8(math.Round(r / a))
			img.Pix[i+1] = uint8(math.Round(g / a))
			img.Pix[i+2] = uint8(math.Round(b / a))
			img.Pix[i+3] = uint8(math.Round(a / n * 0xff))
		}
	}
	return img
}

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
