package barcode

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"github.com/boombuler/barcode/code128"
)

type Bar struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

func Code128PNG(data string, width, height int) ([]byte, error) {
	return Code128PNGWithGeometry(data, width, height, 0.2, 17, 1, 15)
}

func Code128PNGWithModuleWidth(data string, width, height int, moduleWidthMM float64) ([]byte, error) {
	return Code128PNGWithGeometry(data, width, height, moduleWidthMM, 17, 1, 15)
}

func Code128PNGWithGeometry(data string, width, height int, moduleWidthMM, svgHeightMM, barYMM, barHeightMM float64) ([]byte, error) {
	code, err := code128.Encode(data)
	if err != nil {
		return nil, err
	}
	innerWidth, quietLeft := code128QuietZoneSizing(code.Bounds().Dx(), width, moduleWidthMM)
	barTop, barHeight := code128VerticalSizing(height, svgHeightMM, barYMM, barHeightMM)
	scaled := scaleCode128Integer(code, innerWidth, barHeight)
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(quietLeft, barTop, quietLeft+innerWidth, barTop+barHeight), scaled, scaled.Bounds().Min, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Code128PNGWithExactGeometry(data string, width, height int, moduleWidthMM, svgHeightMM, barYMM, barHeightMM float64) ([]byte, error) {
	code, err := code128.Encode(data)
	if err != nil {
		return nil, err
	}
	barTop, barHeight := code128VerticalSizing(height, svgHeightMM, barYMM, barHeightMM)
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	drawCode128Bars(canvas, code, width, barTop, barHeight, moduleWidthMM)
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Code128Bars(data string, moduleWidthMM, barYMM, barHeightMM float64) ([]Bar, float64, error) {
	code, err := code128.Encode(data)
	if err != nil {
		return nil, 0, err
	}
	bounds := code.Bounds()
	moduleCount := bounds.Dx()
	if moduleCount <= 0 || moduleWidthMM <= 0 || barHeightMM <= 0 {
		return nil, 0, nil
	}
	const quietZoneMM = 2.54
	svgWidthMM := float64(moduleCount)*moduleWidthMM + 2*quietZoneMM
	bars := make([]Bar, 0, moduleCount/2)
	runStart := -1
	for module := 0; module < moduleCount; module++ {
		isBlack := code128ModuleBlack(code, bounds.Min.X+module, bounds.Min.Y)
		if isBlack && runStart == -1 {
			runStart = module
		}
		if (!isBlack || module == moduleCount-1) && runStart != -1 {
			runEnd := module
			if isBlack && module == moduleCount-1 {
				runEnd = module + 1
			}
			bars = append(bars, Bar{
				X:      quietZoneMM + float64(runStart)*moduleWidthMM,
				Y:      barYMM,
				Width:  float64(runEnd-runStart) * moduleWidthMM,
				Height: barHeightMM,
			})
			runStart = -1
		}
	}
	return bars, svgWidthMM, nil
}

func scaleCode128Integer(code image.Image, width, height int) image.Image {
	bounds := code.Bounds()
	moduleCount := bounds.Dx()
	if moduleCount <= 0 || width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, maxInt(width, 1), maxInt(height, 1)))
	}
	factor := int(float64(width) / float64(moduleCount))
	if factor < 1 {
		factor = 1
	}
	drawnWidth := moduleCount * factor
	offsetX := (width - drawnWidth) / 2
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	black := image.NewUniform(color.Black)
	for module := 0; module < moduleCount; module++ {
		if !code128ModuleBlack(code, bounds.Min.X+module, bounds.Min.Y) {
			continue
		}
		x1 := offsetX + module*factor
		draw.Draw(dst, image.Rect(x1, 0, x1+factor, height), black, image.Point{}, draw.Src)
	}
	return dst
}

func drawCode128Bars(dst draw.Image, code image.Image, width, barTop, barHeight int, moduleWidthMM float64) {
	bounds := code.Bounds()
	moduleCount := bounds.Dx()
	if moduleCount <= 0 || width <= 0 || barHeight <= 0 || moduleWidthMM <= 0 {
		return
	}
	const quietZoneMM = 2.54
	svgWidthMM := float64(moduleCount)*moduleWidthMM + 2*quietZoneMM
	black := image.NewUniform(color.Black)
	runStart := -1
	for module := 0; module < moduleCount; module++ {
		isBlack := code128ModuleBlack(code, bounds.Min.X+module, bounds.Min.Y)
		if isBlack && runStart == -1 {
			runStart = module
		}
		if (!isBlack || module == moduleCount-1) && runStart != -1 {
			runEnd := module
			if isBlack && module == moduleCount-1 {
				runEnd = module + 1
			}
			x1 := int(math.Round(float64(width) * (quietZoneMM + float64(runStart)*moduleWidthMM) / svgWidthMM))
			x2 := int(math.Round(float64(width) * (quietZoneMM + float64(runEnd)*moduleWidthMM) / svgWidthMM))
			if x2 <= x1 {
				x2 = x1 + 1
			}
			draw.Draw(dst, image.Rect(x1, barTop, x2, barTop+barHeight), black, image.Point{}, draw.Src)
			runStart = -1
		}
	}
}

func code128ModuleBlack(code image.Image, x, y int) bool {
	r, g, b, _ := code.At(x, y).RGBA()
	return r < 0x8000 || g < 0x8000 || b < 0x8000
}

func code128QuietZoneSizing(moduleCount, width int, moduleWidthMM float64) (innerWidth int, quietLeft int) {
	if moduleCount <= 0 || width <= 0 || moduleWidthMM <= 0 {
		return width, 0
	}
	const quietZoneMM = 2.54
	barcodeMM := float64(moduleCount) * moduleWidthMM
	totalMM := barcodeMM + 2*quietZoneMM
	innerWidth = int(math.Round(float64(width) * barcodeMM / totalMM))
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerWidth > width {
		innerWidth = width
	}
	quietLeft = (width - innerWidth) / 2
	return innerWidth, quietLeft
}

func code128VerticalSizing(height int, svgHeightMM, barYMM, barHeightMM float64) (barTop int, barHeight int) {
	if height <= 0 || svgHeightMM <= 0 || barHeightMM <= 0 {
		return 0, height
	}
	barTop = int(math.Round(float64(height) * barYMM / svgHeightMM))
	barHeight = int(math.Round(float64(height) * barHeightMM / svgHeightMM))
	if barHeight < 1 {
		barHeight = 1
	}
	if barTop < 0 {
		barTop = 0
	}
	if barTop+barHeight > height {
		barHeight = height - barTop
	}
	if barHeight < 1 {
		return 0, height
	}
	return barTop, barHeight
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
