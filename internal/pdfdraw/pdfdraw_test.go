package pdfdraw

import "testing"

func TestEncodeTranslatesUTF8ToCoreFontCodePage(t *testing.T) {
	pdf := NewPDF("P", "mm", "A4", "")
	got := pdf.Encode("Representação")
	expected := string([]byte{'R', 'e', 'p', 'r', 'e', 's', 'e', 'n', 't', 'a', 0xe7, 0xe3, 'o'})
	if got != expected {
		t.Fatalf("Encode() bytes = % x, expected % x", []byte(got), []byte(expected))
	}
}
