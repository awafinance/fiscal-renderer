package qrcode

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

import goqrcode "github.com/skip2/go-qrcode"

func PNG(data string, size int) ([]byte, error) {
	return goqrcode.Encode(data, goqrcode.Low, size)
}

func BitmapWithoutBorder(data string) ([][]bool, error) {
	qr, err := goqrcode.New(data, goqrcode.Low)
	if err != nil {
		return nil, err
	}
	qr.DisableBorder = true
	bitmap := qr.Bitmap()
	if len(bitmap) == 0 || len(bitmap[0]) == 0 {
		return nil, fmt.Errorf("empty QR bitmap")
	}
	return bitmap, nil
}

func PNGWithBorder(data string, modulePixels, borderModules int) ([]byte, error) {
	if modulePixels <= 0 {
		return nil, fmt.Errorf("module pixels must be positive")
	}
	if borderModules < 0 {
		return nil, fmt.Errorf("border modules cannot be negative")
	}
	bitmap, err := BitmapWithoutBorder(data)
	if err != nil {
		return nil, err
	}

	quiet := borderModules * modulePixels
	size := (len(bitmap) + 2*borderModules) * modulePixels
	img := image.NewPaletted(
		image.Rect(0, 0, size, size),
		color.Palette{color.White, color.Black},
	)
	for y, row := range bitmap {
		for x, black := range row {
			if !black {
				continue
			}
			x0 := quiet + x*modulePixels
			y0 := quiet + y*modulePixels
			for py := y0; py < y0+modulePixels; py++ {
				offset := img.PixOffset(x0, py)
				for px := 0; px < modulePixels; px++ {
					img.Pix[offset+px] = 1
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
