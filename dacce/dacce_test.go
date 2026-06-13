package dacce

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awafinance/fiscal-renderer/internal/golden"
)

func TestDefaultIssuerMatchesPythonCLI(t *testing.T) {
	issuer := DefaultIssuer()
	if issuer.Name != "EMPRESA LTDA" || issuer.Address != "AV. TEST, 100" || issuer.CEP != "88888-88" {
		t.Fatalf("issuer = %#v", issuer)
	}
}

func TestInvoiceNumberMatchesPythonFormatting(t *testing.T) {
	key := "99999999999999999999999990000152939999999999"
	if got := invoiceNumber(key); got != "000.015.293" {
		t.Fatalf("invoiceNumber = %q", got)
	}
}

func TestWriteOutputsPDF(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacce", "xml_cce_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	logoBytes, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := New(string(xmlContent), &Config{ImageBytes: logoBytes})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := doc.Write(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF-")) {
		t.Fatalf("Write output is not a PDF: %q", out.String())
	}
}

func TestFixtureMatchesGoldenShape(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacce", "xml_cce_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "cce.pdf")
	doc, err := New(string(xmlContent), &Config{
		Issuer: DefaultIssuer(),
		Image:  filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	if err := golden.IsPDF(out); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join("..", "tests", "generated", "dacce", "cce.pdf")
	if err := golden.SamePageCount(out, expected); err != nil {
		t.Fatal(err)
	}
	if golden.PDFInfoAvailable() {
		if err := golden.SamePageGeometry(out, expected, 0.01); err != nil {
			t.Fatal(err)
		}
	}
	if golden.PDFTextAvailable() {
		if err := golden.SameExtractedText(out, expected); err != nil {
			t.Fatal(err)
		}
	}
	if golden.PDFToPPMAvailable() {
		diffs, err := golden.RasterDiffPages(out, expected, 72)
		if err != nil {
			t.Fatal(err)
		}
		if max := golden.MaxMeanAbsoluteError(diffs); max > 0.005 {
			t.Fatalf("raster diff too high: max=%f pages=%#v", max, diffs)
		}
	}
}
