package qrcode

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestPNG(t *testing.T) {
	data, err := PNG("https://example.test", 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("not a PNG: %x", data[:8])
	}
}

func TestPNGWithBorderUsesRequestedQuietZone(t *testing.T) {
	const modulePixels = 3
	const borderModules = 1
	data, err := PNGWithBorder("https://example.test", modulePixels, borderModules)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	minX, minY, _, _ := blackBounds(t, img)
	want := modulePixels * borderModules
	if minX != want || minY != want {
		t.Fatalf("black bounds started at (%d,%d), want (%d,%d)", minX, minY, want, want)
	}
}

func blackBounds(t *testing.T, img image.Image) (int, int, int, int) {
	t.Helper()
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 > 80 || g>>8 > 80 || b>>8 > 80 {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if !found {
		t.Fatal("QR image has no black pixels")
	}
	return minX, minY, maxX, maxY
}
