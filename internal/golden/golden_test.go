package golden

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-pdf/fpdf"
)

func TestIsPDFAndPageCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one-page.pdf")
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.AddPage()
	pdf.SetFont("Times", "", 12)
	pdf.Cell(40, 10, "hello")
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatal(err)
	}
	if err := IsPDF(path); err != nil {
		t.Fatal(err)
	}
	count, err := PageCount(path)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("PageCount = %d", count)
	}
}

func TestIsPDFRejectsNonPDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.pdf")
	if err := os.WriteFile(path, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := IsPDF(path); err == nil {
		t.Fatal("expected non-PDF error")
	}
}

func TestSamePageGeometryWithPDFInfo(t *testing.T) {
	if !PDFInfoAvailable() {
		t.Skip("pdfinfo not available")
	}
	path := filepath.Join(t.TempDir(), "one-page.pdf")
	writeTestPDF(t, path)
	if err := SamePageGeometry(path, path, 0.01); err != nil {
		t.Fatal(err)
	}
	info, err := PDFInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Pages != 1 {
		t.Fatalf("Pages = %d", info.Pages)
	}
	if info.PageWidthPts <= 0 || info.PageHeightPts <= 0 {
		t.Fatalf("invalid page size: %#v", info)
	}
}

func TestRasterDiffFirstPageIdenticalPDF(t *testing.T) {
	if !PDFToPPMAvailable() {
		t.Skip("pdftoppm not available")
	}
	path := filepath.Join(t.TempDir(), "one-page.pdf")
	writeTestPDF(t, path)
	diff, err := RasterDiffFirstPage(path, path, 72)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Width <= 0 || diff.Height <= 0 {
		t.Fatalf("invalid raster dimensions: %#v", diff)
	}
	if diff.MeanAbsoluteError != 0 || diff.MaxChannelError != 0 {
		t.Fatalf("identical PDF raster diff = %#v", diff)
	}
}

func TestRasterDiffPagesIdenticalMultipagePDF(t *testing.T) {
	if !PDFToPPMAvailable() {
		t.Skip("pdftoppm not available")
	}
	path := filepath.Join(t.TempDir(), "two-page.pdf")
	writeTestPDFWithPages(t, path, 2)
	diffs, err := RasterDiffPages(path, path, 72)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 2 {
		t.Fatalf("page diffs = %d", len(diffs))
	}
	for i, diff := range diffs {
		if diff.Page != i+1 {
			t.Fatalf("diff page = %d, want %d", diff.Page, i+1)
		}
		if diff.Width <= 0 || diff.Height <= 0 {
			t.Fatalf("invalid raster dimensions: %#v", diff)
		}
		if diff.MeanAbsoluteError != 0 || diff.MaxChannelError != 0 {
			t.Fatalf("identical PDF raster diff = %#v", diff)
		}
	}
	if max := MaxMeanAbsoluteError(diffs); max != 0 {
		t.Fatalf("MaxMeanAbsoluteError = %f", max)
	}
}

func TestSameExtractedTextIdenticalPDF(t *testing.T) {
	if !PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	path := filepath.Join(t.TempDir(), "one-page.pdf")
	writeTestPDF(t, path)
	if err := SameExtractedText(path, path); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeExtractedText(t *testing.T) {
	got := NormalizeExtractedText("  Representação\n\tGráfica   de CC-e\f")
	if got != "Representação Gráfica de CC-e" {
		t.Fatalf("NormalizeExtractedText = %q", got)
	}
}

func writeTestPDF(t *testing.T, path string) {
	t.Helper()
	writeTestPDFWithPages(t, path, 1)
}

func writeTestPDFWithPages(t *testing.T, path string, pages int) {
	t.Helper()
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetFont("Times", "", 12)
	for page := 1; page <= pages; page++ {
		pdf.AddPage()
		pdf.Cell(40, 10, "hello")
	}
	if err := pdf.OutputFileAndClose(path); err != nil {
		t.Fatal(err)
	}
}
