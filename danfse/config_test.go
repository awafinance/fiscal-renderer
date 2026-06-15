package danfse

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awafinance/fiscal-renderer/internal/golden"
)

func TestDefaultConfigMatchesPythonDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Margins != (Margins{Top: 5, Right: 5, Bottom: 5, Left: 5}) {
		t.Fatalf("margins = %#v", cfg.Margins)
	}
	if cfg.DecimalConfig != (DecimalConfig{PricePrecision: 4, QuantityPrecision: 4}) {
		t.Fatalf("decimal config = %#v", cfg.DecimalConfig)
	}
	if cfg.FontType != FontTypeTimes {
		t.Fatalf("font type = %q", cfg.FontType)
	}
	if cfg.WatermarkCancelled {
		t.Fatalf("watermark cancelled default should be false")
	}
}

func TestExportedValuesMatchPythonEnums(t *testing.T) {
	if FontTypeCourier != "Courier" || FontTypeTimes != "Times" {
		t.Fatalf("font type values changed: %q %q", FontTypeCourier, FontTypeTimes)
	}
}

func TestDANFSELogoIsEmbeddedForInstalledBinaries(t *testing.T) {
	if len(nfseLogoPNG) == 0 {
		t.Fatal("embedded DANFSE logo is empty")
	}
	if !bytes.HasPrefix(nfseLogoPNG, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("embedded DANFSE logo is not PNG data")
	}
}

func TestFontTypeCourierIsUsedForDocumentTextLikeUpstream(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "danfse", "nfse_test_prod.xml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := New(string(xmlContent), &Config{FontType: FontTypeCourier})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := doc.Write(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("/BaseFont /Courier")) {
		t.Fatal("DANFSE Courier config did not produce Courier text font")
	}
	if bytes.Contains(out.Bytes(), []byte("/BaseFont /Times")) {
		t.Fatal("DANFSE Courier config still produced Times base font")
	}
}

func TestWriteOutputsPDF(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "danfse", "nfse_test_prod.xml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := New(string(xmlContent), nil)
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

func TestHomologationWatermarkIsRotatedLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfse_test_hom.xml", &Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}})
	if strings.Contains(text, "SEM VALOR FISCAL") {
		t.Fatalf("watermark was extracted as horizontal text, want rotated upstream-like extraction: %q", text)
	}
}

func TestCancelledWatermarkIsRotatedLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfse_test_prod.xml", &Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}, WatermarkCancelled: true})
	if strings.Contains(text, "CANCELADA") {
		t.Fatalf("cancelled watermark was extracted as horizontal text, want rotated upstream-like extraction: %q", text)
	}
}

func TestPISCOFINSDebitValuesMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfse_test_prod.xml", &Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}})
	for _, want := range []string{
		"PIS - Débito Apuração Própria COFINS - Débito Apuração Própria R$ 0,0000 R$ 0,0000",
		"Total das Retenções Federais",
		"PIS/COFINS - Débito Apur. Própria",
		"Valor Líquido da NFS-e",
		"R$ 615,0000",
		"R$ 0,0000",
		"R$ 8.885,0000",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("DANFSE PIS/COFINS text missing %q in %q", want, text)
		}
	}
}

func TestBenchmarkDANFSELongHeaderAndPartyFieldsMatchUpstreamLayout(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	out := renderFixturePDF(t, "nfse_test_prod.xml", &Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}})
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	text = golden.NormalizeExtractedText(text)
	for _, want := range []string{
		"42046082240248250000160000000000099999999999999999",
		"estoque@renataeleandropublicidadeepropagandame.com.br",
		"sistema@bentoesilvanadocessalgadosltda.com.br",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("DANFSE layout truncated upstream-visible field %q in %q", want, text)
		}
	}
	for _, unexpected := range []string{
		"42046082240248250000160000000...",
		"estoque@renataeleandropublicidadeepropaga...",
		"sistema@bentoesilvanadocessalgadosltda.co...",
	} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("DANFSE layout emitted broken ellipsis %q in %q", unexpected, text)
		}
	}
	words, err := golden.ExtractTextWords(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"estoque@renataeleandropublicidadeepropagandame.com.br",
		"sistema@bentoesilvanadocessalgadosltda.com.br",
	} {
		word, ok := findPDFWord(words, want)
		if !ok {
			t.Fatalf("DANFSE positioned text missing word %q", want)
		}
		if word.XMin < 290 {
			t.Fatalf("DANFSE email %q rendered in left grid at xMin=%f, want right-half upstream layout", want, word.XMin)
		}
	}
	expectedWords, err := golden.ExtractTextWords(filepath.Join("..", "tests", "generated", "danfse", "danfse_default_prod.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, anchor := range []struct {
		name       string
		text       string
		xMax       float64
		occurrence int
		tolerance  float64
	}{
		{name: "issuer title", text: "EMITENTE", xMax: 60, tolerance: 4},
		{name: "taker title", text: "TOMADOR", xMax: 60, tolerance: 4},
		{name: "service title", text: "SERVIÇO", xMax: 60, tolerance: 4},
		{name: "municipal tax title", text: "TRIBUTAÇÃO", xMax: 60, tolerance: 4},
		{name: "federal tax title", text: "TRIBUTAÇÃO", xMax: 60, occurrence: 1, tolerance: 4},
		{name: "total value title", text: "VALOR", xMax: 60, tolerance: 4},
	} {
		actualWord, ok := findPDFWordByPosition(words, anchor.text, 0, anchor.xMax, anchor.occurrence)
		if !ok {
			t.Fatalf("actual DANFSE anchor %q not found", anchor.name)
		}
		expectedWord, ok := findPDFWordByPosition(expectedWords, anchor.text, 0, anchor.xMax, anchor.occurrence)
		if !ok {
			t.Fatalf("expected DANFSE anchor %q not found", anchor.name)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DANFSE anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestServiceCodeStripsDuplicatedDescriptionPrefixLikeUpstream(t *testing.T) {
	goldenMargins := Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}
	if got := serviceColumnWidth(goldenMargins); got != 51.5 {
		t.Fatalf("service column width = %f", got)
	}
	if got := serviceColumnWidth(DefaultMargins()); got != 50 {
		t.Fatalf("default service column width = %f", got)
	}
	if got := serviceNationalTaxCode("010501", "0105 | 1.05 - Serviço de Publicidade e Propaganda", goldenMargins, FontTypeTimes); got != "01.05.01 - Serviço de Publicidade e..." {
		t.Fatalf("service national tax code = %q", got)
	}
	if got := serviceNationalTaxCode("1401", "Serviço Prestado conforme CNAE 4929-9/02 – Preparação de documentos e serviços especializados de apoio administrativo não especificados anteriormente, referente ao mês de 02/2026.", goldenMargins, FontTypeTimes); got != "1401 - Serviço Prestado conforme CNAE..." {
		t.Fatalf("homologation service national tax code = %q", got)
	}
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfse_test_prod.xml", &Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}})
	if strings.Contains(text, "01.05.01 - 0105 | 1.05") {
		t.Fatalf("service code duplicated municipal prefix in %q", text)
	}
	if !strings.Contains(text, "01.05.01 - Serviço de Publicidade e...") {
		t.Fatalf("service code missing upstream stripped description in %q", text)
	}
}

func TestFixtureOutputsMatchGoldenShape(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		expected string
		config   Config
	}{
		{
			name:     "default prod",
			fixture:  filepath.Join("..", "tests", "fixtures", "danfse", "nfse_test_prod.xml"),
			expected: filepath.Join("..", "tests", "generated", "danfse", "danfse_default_prod.pdf"),
			config:   Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}},
		},
		{
			name:     "default hom",
			fixture:  filepath.Join("..", "tests", "fixtures", "danfse", "nfse_test_hom.xml"),
			expected: filepath.Join("..", "tests", "generated", "danfse", "danfse_default_hom.pdf"),
			config:   Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}},
		},
		{
			name:     "cancelled prod",
			fixture:  filepath.Join("..", "tests", "fixtures", "danfse", "nfse_test_prod.xml"),
			expected: filepath.Join("..", "tests", "generated", "danfse", "danfse_cancelled_prod.pdf"),
			config:   Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}, WatermarkCancelled: true},
		},
		{
			name:     "cancelled hom",
			fixture:  filepath.Join("..", "tests", "fixtures", "danfse", "nfse_test_hom.xml"),
			expected: filepath.Join("..", "tests", "generated", "danfse", "danfse_cancelled_hom.pdf"),
			config:   Config{Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}, WatermarkCancelled: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xmlContent, err := os.ReadFile(tt.fixture)
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "danfse.pdf")
			doc, err := New(string(xmlContent), &tt.config)
			if err != nil {
				t.Fatal(err)
			}
			if err := doc.Output(out); err != nil {
				t.Fatal(err)
			}
			if err := golden.IsPDF(out); err != nil {
				t.Fatal(err)
			}
			if err := golden.SamePageCount(out, tt.expected); err != nil {
				t.Fatal(err)
			}
			if golden.PDFInfoAvailable() {
				if err := golden.SamePageGeometry(out, tt.expected, 0.01); err != nil {
					t.Fatal(err)
				}
			}
			if golden.PDFTextAvailable() {
				text, err := golden.ExtractText(out)
				if err != nil {
					t.Fatal(err)
				}
				text = golden.NormalizeExtractedText(text)
				for _, want := range []string{
					"A autenticidade desta NFS-e",
					"Prestador do Serviço",
					"Simples Nacional na Data de Competência",
					"Regime de Apuração Tributária pelo SN",
					"INTERMEDIÁRIO DO SERVIÇO",
					"NÃO IDENTIFICADO NA NFS-e",
					"Código de Tributação Municipal",
					"País da Prestação",
					"Descrição do Serviço",
					"Tributação do ISSQN",
					"Município de Incidência do ISSQN",
					"Regime Especial de Tributação",
					"BC ISSQN",
					"TRIBUTAÇÃO FEDERAL",
					"VALOR TOTAL DA NFS-E",
					"TOTAIS APROXIMADOS DOS TRIBUTOS",
					"INFORMAÇÕES COMPLEMENTARES",
				} {
					if !strings.Contains(text, want) {
						t.Fatalf("extracted text missing %q in %q", want, text)
					}
				}
			}
			if golden.PDFToPPMAvailable() {
				diffs, err := golden.RasterDiffPages(out, tt.expected, 72)
				if err != nil {
					t.Fatal(err)
				}
				if max := golden.MaxMeanAbsoluteError(diffs); max > 0.07 {
					t.Fatalf("raster diff too high: max=%f pages=%#v", max, diffs)
				}
			}
		})
	}
}

func renderFixtureText(t *testing.T, fixture string, config *Config) string {
	t.Helper()
	out := renderFixturePDF(t, fixture, config)
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	return golden.NormalizeExtractedText(text)
}

func renderFixturePDF(t *testing.T, fixture string, config *Config) string {
	t.Helper()
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "danfse", fixture))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "danfse.pdf")
	doc, err := New(string(xmlContent), config)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func findPDFWord(words []golden.TextWord, text string) (golden.TextWord, bool) {
	for _, word := range words {
		if word.Text == text {
			return word, true
		}
	}
	return golden.TextWord{}, false
}

func findPDFWordByPosition(words []golden.TextWord, text string, xMin, xMax float64, occurrence int) (golden.TextWord, bool) {
	seen := 0
	for _, word := range words {
		if word.Text != text || word.XMin < xMin || word.XMin > xMax {
			continue
		}
		if seen == occurrence {
			return word, true
		}
		seen++
	}
	return golden.TextWord{}, false
}
