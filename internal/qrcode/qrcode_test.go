package qrcode

import "testing"

func TestPNG(t *testing.T) {
	data, err := PNG("https://example.test", 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("not a PNG: %x", data[:8])
	}
}
