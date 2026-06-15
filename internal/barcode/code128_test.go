package barcode

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"testing"
)

func TestCode128PNGPreservesPythonSVGVerticalPadding(t *testing.T) {
	pngBytes, err := Code128PNG("422403024845550003435700200003396861082134657", 430, 70)
	if err != nil {
		t.Fatal(err)
	}
	minX, minY, maxX, maxY := mustBlackBounds(t, pngBytes)
	assertBetween(t, "default minX", minX, 60, 70)
	assertBetween(t, "default maxX", maxX, 360, 370)
	assertBetween(t, "default minY", minY, 3, 6)
	assertBetween(t, "default maxY", maxY, 63, 67)
}

func TestCode128PNGWithExactGeometryMatchesPythonSVGQuietZoneGeometry(t *testing.T) {
	pngBytes, err := Code128PNGWithExactGeometry("422403024845550003435700200003396861082134657", 430, 70, 0.2, 17, 1, 15)
	if err != nil {
		t.Fatal(err)
	}
	minX, minY, maxX, maxY := mustBlackBounds(t, pngBytes)
	assertBetween(t, "exact minX", minX, 15, 20)
	assertBetween(t, "exact maxX", maxX, 410, 416)
	assertBetween(t, "exact minY", minY, 3, 6)
	assertBetween(t, "exact maxY", maxY, 63, 67)
}

func TestCode128PNGWithExactGeometryMatchesDAMDFESVGGeometry(t *testing.T) {
	pngBytes, err := Code128PNGWithExactGeometry("4224068528977500019558001000000151986453241", 430, 85, 0.3, 23.764, 1, 15)
	if err != nil {
		t.Fatal(err)
	}
	minX, minY, maxX, maxY := mustBlackBounds(t, pngBytes)
	assertBetween(t, "damdfe minX", minX, 10, 15)
	assertBetween(t, "damdfe maxX", maxX, 416, 421)
	assertBetween(t, "damdfe minY", minY, 3, 6)
	assertBetween(t, "damdfe maxY", maxY, 55, 60)
}

func TestCode128BarsMatchPythonSVGRectGeometry(t *testing.T) {
	bars, totalWidth, err := Code128Bars("42240802582975000109580010000000151908765432", 0.3, 1, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 76 {
		t.Fatalf("bars = %d, want 76", len(bars))
	}
	if math.Abs(totalWidth-88.18) > 0.001 {
		t.Fatalf("totalWidth = %f, want 88.18", totalWidth)
	}
	first := bars[0]
	if math.Abs(first.X-2.54) > 0.001 || math.Abs(first.Width-0.6) > 0.001 || math.Abs(first.Y-1) > 0.001 || math.Abs(first.Height-15) > 0.001 {
		t.Fatalf("first bar = %+v, want Python SVG x=2.54 width=0.6 y=1 height=15", first)
	}
}

func mustBlackBounds(t *testing.T, pngBytes []byte) (minX, minY, maxX, maxY int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	bounds := img.Bounds()
	minX, minY = bounds.Max.X, bounds.Max.Y
	maxX, maxY = bounds.Min.X-1, bounds.Min.Y-1
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if isBlack(img, x, y) {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX || maxY < minY {
		t.Fatal("barcode image has no black pixels")
	}
	return minX, minY, maxX, maxY
}

func isBlack(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r < 0x8000 || g < 0x8000 || b < 0x8000
}

func assertBetween(t *testing.T, name string, got, min, max int) {
	t.Helper()
	if got < min || got > max {
		t.Fatalf("%s = %d, want %d..%d", name, got, min, max)
	}
}
