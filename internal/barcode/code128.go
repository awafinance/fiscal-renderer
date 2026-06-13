package barcode

import (
	"bytes"
	"image/png"

	boombarcode "github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
)

func Code128PNG(data string, width, height int) ([]byte, error) {
	code, err := code128.Encode(data)
	if err != nil {
		return nil, err
	}
	scaled, err := boombarcode.Scale(code, width, height)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
