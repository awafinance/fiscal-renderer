package qrcode

import goqrcode "github.com/skip2/go-qrcode"

func PNG(data string, size int) ([]byte, error) {
	return goqrcode.Encode(data, goqrcode.Low, size)
}
