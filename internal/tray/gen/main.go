//go:build ignore

// Command gen renders the monochrome macOS template icons for the menu bar
// from assets/brand/icon.png: the alpha channel of the brand emblem becomes
// a black shape on a transparent background at 22 px and 44 px (@2x).
//
//	go run ./internal/tray/gen/main.go
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	src := flagOr(1, "assets/brand/icon.png")
	outDir := flagOr(2, "internal/tray/assets")
	f, err := os.Open(src)
	if err != nil {
		fail(err)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		fail(err)
	}
	crop := alphaBounds(img)
	for _, size := range []int{22, 44} {
		out := render(img, crop, size)
		name := filepath.Join(outDir, fmt.Sprintf("tray-template-%d.png", size))
		w, err := os.Create(name)
		if err != nil {
			fail(err)
		}
		if err := png.Encode(w, out); err != nil {
			fail(err)
		}
		w.Close()
		fmt.Println("wrote", name)
	}
}

func flagOr(i int, def string) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return def
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen:", err)
	os.Exit(1)
}

// alphaBounds is the square, centered on the opaque pixels' bounding box,
// that contains every pixel with alpha above a small threshold.
func alphaBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0x1000 {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if minX > maxX {
		return b
	}
	w, h := maxX-minX+1, maxY-minY+1
	side := max(w, h)
	cx, cy := minX+w/2, minY+h/2
	return image.Rect(cx-side/2, cy-side/2, cx-side/2+side, cy-side/2+side)
}

// render box-filters the alpha of crop into a size x size black template
// image with a one pixel transparent margin per 22 px.
func render(img image.Image, crop image.Rectangle, size int) *image.NRGBA {
	margin := size / 22
	inner := size - 2*margin
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := float64(crop.Dx()) / float64(inner)
	for oy := 0; oy < inner; oy++ {
		for ox := 0; ox < inner; ox++ {
			x0 := crop.Min.X + int(float64(ox)*scale)
			y0 := crop.Min.Y + int(float64(oy)*scale)
			x1 := crop.Min.X + int(float64(ox+1)*scale)
			y1 := crop.Min.Y + int(float64(oy+1)*scale)
			var sum, n uint64
			for y := y0; y < max(y1, y0+1); y++ {
				for x := x0; x < max(x1, x0+1); x++ {
					_, _, _, a := img.At(x, y).RGBA()
					sum += uint64(a >> 8)
					n++
				}
			}
			out.SetNRGBA(ox+margin, oy+margin, color.NRGBA{A: uint8(sum / n)})
		}
	}
	return out
}
