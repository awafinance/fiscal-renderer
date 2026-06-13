package golden

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var pageObjectPattern = regexp.MustCompile(`/Type\s*/Page\b`)
var pdfInfoPagesPattern = regexp.MustCompile(`(?m)^Pages:\s+(\d+)\s*$`)
var pdfInfoPageSizePattern = regexp.MustCompile(`(?m)^Page size:\s+([0-9.]+)\s+x\s+([0-9.]+)\s+pts\b`)

type Info struct {
	Pages         int
	PageWidthPts  float64
	PageHeightPts float64
}

type RasterDiff struct {
	Page              int
	Width             int
	Height            int
	MeanAbsoluteError float64
	MaxChannelError   uint32
}

func IsPDF(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return fmt.Errorf("%s is not a PDF", path)
	}
	return nil
}

func PageCount(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return 0, fmt.Errorf("%s is not a PDF", path)
	}
	return len(pageObjectPattern.FindAll(data, -1)), nil
}

func SamePageCount(actualPath, expectedPath string) error {
	actual, err := PageCount(actualPath)
	if err != nil {
		return err
	}
	expected, err := PageCount(expectedPath)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("page count mismatch: actual=%d expected=%d", actual, expected)
	}
	return nil
}

func PDFInfoAvailable() bool {
	_, err := exec.LookPath("pdfinfo")
	return err == nil
}

func PDFInfo(path string) (Info, error) {
	if !PDFInfoAvailable() {
		return Info{}, errors.New("pdfinfo not available")
	}
	output, err := exec.Command("pdfinfo", path).Output()
	if err != nil {
		return Info{}, err
	}
	pagesMatch := pdfInfoPagesPattern.FindSubmatch(output)
	if len(pagesMatch) != 2 {
		return Info{}, fmt.Errorf("pdfinfo pages not found for %s", path)
	}
	widthHeightMatch := pdfInfoPageSizePattern.FindSubmatch(output)
	if len(widthHeightMatch) != 3 {
		return Info{}, fmt.Errorf("pdfinfo page size not found for %s", path)
	}
	pages, err := strconv.Atoi(string(pagesMatch[1]))
	if err != nil {
		return Info{}, err
	}
	width, err := strconv.ParseFloat(string(widthHeightMatch[1]), 64)
	if err != nil {
		return Info{}, err
	}
	height, err := strconv.ParseFloat(string(widthHeightMatch[2]), 64)
	if err != nil {
		return Info{}, err
	}
	return Info{Pages: pages, PageWidthPts: width, PageHeightPts: height}, nil
}

func SamePageGeometry(actualPath, expectedPath string, tolerancePts float64) error {
	actual, err := PDFInfo(actualPath)
	if err != nil {
		return err
	}
	expected, err := PDFInfo(expectedPath)
	if err != nil {
		return err
	}
	if actual.Pages != expected.Pages {
		return fmt.Errorf("page count mismatch: actual=%d expected=%d", actual.Pages, expected.Pages)
	}
	if math.Abs(actual.PageWidthPts-expected.PageWidthPts) > tolerancePts ||
		math.Abs(actual.PageHeightPts-expected.PageHeightPts) > tolerancePts {
		return fmt.Errorf("page size mismatch: actual=%.2fx%.2f expected=%.2fx%.2f pts", actual.PageWidthPts, actual.PageHeightPts, expected.PageWidthPts, expected.PageHeightPts)
	}
	return nil
}

func PDFToPPMAvailable() bool {
	_, err := exec.LookPath("pdftoppm")
	return err == nil
}

func RasterDiffFirstPage(actualPath, expectedPath string, dpi int) (RasterDiff, error) {
	diffs, err := RasterDiffPagesRange(actualPath, expectedPath, dpi, 1, 1)
	if err != nil {
		return RasterDiff{}, err
	}
	if len(diffs) != 1 {
		return RasterDiff{}, fmt.Errorf("first-page raster diff returned %d pages", len(diffs))
	}
	return diffs[0], nil
}

func RasterDiffPages(actualPath, expectedPath string, dpi int) ([]RasterDiff, error) {
	return RasterDiffPagesRange(actualPath, expectedPath, dpi, 0, 0)
}

func RasterDiffPagesRange(actualPath, expectedPath string, dpi, firstPage, lastPage int) ([]RasterDiff, error) {
	if !PDFToPPMAvailable() {
		return nil, errors.New("pdftoppm not available")
	}
	if dpi <= 0 {
		return nil, errors.New("dpi must be positive")
	}
	if firstPage < 0 || lastPage < 0 || (firstPage == 0 && lastPage != 0) || (firstPage != 0 && lastPage != 0 && lastPage < firstPage) {
		return nil, errors.New("invalid page range")
	}
	dir, err := os.MkdirTemp("", "fiscal-render-pdf-raster-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	actualPNGs, err := renderPagesPNG(actualPath, filepath.Join(dir, "actual"), dpi, firstPage, lastPage)
	if err != nil {
		return nil, fmt.Errorf("render actual: %w", err)
	}
	expectedPNGs, err := renderPagesPNG(expectedPath, filepath.Join(dir, "expected"), dpi, firstPage, lastPage)
	if err != nil {
		return nil, fmt.Errorf("render expected: %w", err)
	}
	if len(actualPNGs) != len(expectedPNGs) {
		return nil, fmt.Errorf("rendered page count mismatch: actual=%d expected=%d", len(actualPNGs), len(expectedPNGs))
	}
	diffs := make([]RasterDiff, 0, len(actualPNGs))
	for i := range actualPNGs {
		actualImage, err := decodePNG(actualPNGs[i])
		if err != nil {
			return nil, fmt.Errorf("decode actual page %d: %w", i+1, err)
		}
		expectedImage, err := decodePNG(expectedPNGs[i])
		if err != nil {
			return nil, fmt.Errorf("decode expected page %d: %w", i+1, err)
		}
		page := i + 1
		if firstPage > 0 {
			page = firstPage + i
		}
		diff, err := rasterDiffImages(actualImage, expectedImage, page)
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, diff)
	}
	return diffs, nil
}

func MaxMeanAbsoluteError(diffs []RasterDiff) float64 {
	max := 0.0
	for _, diff := range diffs {
		if diff.MeanAbsoluteError > max {
			max = diff.MeanAbsoluteError
		}
	}
	return max
}

func PDFTextAvailable() bool {
	_, err := exec.LookPath("pdftotext")
	return err == nil
}

func ExtractText(path string) (string, error) {
	if !PDFTextAvailable() {
		return "", errors.New("pdftotext not available")
	}
	output, err := exec.Command("pdftotext", "-enc", "UTF-8", path, "-").Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func SameExtractedText(actualPath, expectedPath string) error {
	actual, err := ExtractText(actualPath)
	if err != nil {
		return err
	}
	expected, err := ExtractText(expectedPath)
	if err != nil {
		return err
	}
	actual = NormalizeExtractedText(actual)
	expected = NormalizeExtractedText(expected)
	if actual != expected {
		return fmt.Errorf("extracted PDF text mismatch:\nactual:   %s\nexpected: %s", actual, expected)
	}
	return nil
}

func NormalizeExtractedText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func rasterDiffImages(actualImage, expectedImage image.Image, page int) (RasterDiff, error) {
	actualBounds := actualImage.Bounds()
	expectedBounds := expectedImage.Bounds()
	if actualBounds.Dx() != expectedBounds.Dx() || actualBounds.Dy() != expectedBounds.Dy() {
		return RasterDiff{}, fmt.Errorf("raster size mismatch on page %d: actual=%dx%d expected=%dx%d", page, actualBounds.Dx(), actualBounds.Dy(), expectedBounds.Dx(), expectedBounds.Dy())
	}
	width := actualBounds.Dx()
	height := actualBounds.Dy()
	var total uint64
	var max uint32
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ar, ag, ab, aa := rgba(actualImage, actualBounds.Min.X+x, actualBounds.Min.Y+y)
			er, eg, eb, ea := rgba(expectedImage, expectedBounds.Min.X+x, expectedBounds.Min.Y+y)
			for _, delta := range []uint32{
				channelDelta(ar, er),
				channelDelta(ag, eg),
				channelDelta(ab, eb),
				channelDelta(aa, ea),
			} {
				total += uint64(delta)
				if delta > max {
					max = delta
				}
			}
		}
	}
	channels := float64(width * height * 4)
	return RasterDiff{
		Page:              page,
		Width:             width,
		Height:            height,
		MeanAbsoluteError: float64(total) / channels / 65535,
		MaxChannelError:   max,
	}, nil
}

func QPDFAvailable() bool {
	_, err := exec.LookPath("qpdf")
	return err == nil
}

func NormalizedEqual(actualPath, expectedPath string) error {
	if !QPDFAvailable() {
		return errors.New("qpdf not available")
	}
	actual, err := qpdf(actualPath)
	if err != nil {
		return fmt.Errorf("normalize actual: %w", err)
	}
	expected, err := qpdf(expectedPath)
	if err != nil {
		return fmt.Errorf("normalize expected: %w", err)
	}
	actual = filterVolatileLines(actual)
	expected = filterVolatileLines(expected)
	if !bytes.Equal(actual, expected) {
		return errors.New("normalized PDFs differ")
	}
	return nil
}

func qpdf(path string) ([]byte, error) {
	return exec.Command("qpdf", "--deterministic-id", "--password=fpdf2", "--qdf", path, "-").Output()
}

func filterVolatileLines(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	filtered := lines[:0]
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("  /ID [<")) {
			continue
		}
		if bytes.HasPrefix(line, []byte("%% Original object ID: ")) {
			continue
		}
		if bytes.HasSuffix(line, []byte(" 00000 n ")) {
			continue
		}
		filtered = append(filtered, line)
	}
	return bytes.Join(filtered, []byte("\n"))
}

func renderPagesPNG(path, prefix string, dpi, firstPage, lastPage int) ([]string, error) {
	args := []string{"-r", strconv.Itoa(dpi), "-png"}
	if firstPage > 0 {
		args = append([]string{"-f", strconv.Itoa(firstPage), "-l", strconv.Itoa(lastPage)}, args...)
	}
	args = append(args, path, prefix)
	cmd := exec.Command("pdftoppm", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	candidates, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("pdftoppm produced no PNG for %s", path)
	}
	sortRenderedPages(prefix, candidates)
	return candidates, nil
}

func sortRenderedPages(prefix string, paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		left := renderedPageNumber(prefix, paths[i])
		right := renderedPageNumber(prefix, paths[j])
		if left == right {
			return paths[i] < paths[j]
		}
		return left < right
	})
}

func renderedPageNumber(prefix, path string) int {
	base := filepath.Base(path)
	prefixBase := filepath.Base(prefix)
	number := strings.TrimPrefix(base, prefixBase+"-")
	number = strings.TrimSuffix(number, filepath.Ext(number))
	page, err := strconv.Atoi(number)
	if err != nil {
		return 0
	}
	return page
}

func decodePNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return png.Decode(file)
}

func rgba(img image.Image, x, y int) (uint32, uint32, uint32, uint32) {
	rgba := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
	return uint32(rgba.R) * 257, uint32(rgba.G) * 257, uint32(rgba.B) * 257, uint32(rgba.A) * 257
}

func channelDelta(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
