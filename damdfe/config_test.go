package damdfe

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	if cfg.Logo != "" || cfg.DisplayOrigemDestinoPrestacao {
		t.Fatalf("zero-value bool/string defaults changed: %#v", cfg)
	}
}

func TestExportedValuesMatchPythonEnums(t *testing.T) {
	if FontTypeCourier != "Courier" || FontTypeTimes != "Times" {
		t.Fatalf("font type values changed: %q %q", FontTypeCourier, FontTypeTimes)
	}
	expectedModals := map[ModalType]string{
		ModalTypeRodoviario:  "RODOVIÁRIO",
		ModalTypeAereo:       "AÉREO",
		ModalTypeAquaviario:  "AQUAVIÁRIO",
		ModalTypeFerroviario: "FERROVIÁRIO",
	}
	for modal, expected := range expectedModals {
		if string(modal) != expected {
			t.Fatalf("modal value = %q, want %q", modal, expected)
		}
	}
	if EmissionTypeNormal != "NORMAL" || EmissionTypeContingencia != "CONTINGÊNCIA" {
		t.Fatalf("emission type values changed: %q %q", EmissionTypeNormal, EmissionTypeContingencia)
	}
}

func TestFontTypeCourierIsUsedForDocumentTextLikeUpstream(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "damdfe", "mdf-e_test_1.xml"))
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
		t.Fatal("DAMDFE Courier config did not produce Courier text font")
	}
	if bytes.Contains(out.Bytes(), []byte("/BaseFont /Times")) {
		t.Fatal("DAMDFE Courier config still produced Times base font")
	}
}

func TestWriteOutputsPDF(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "damdfe", "mdf-e_test_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	logoBytes, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := New(string(xmlContent), &Config{LogoBytes: logoBytes})
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

func TestNoAuthorizationWatermarkIsRotatedLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "mdf-e_test_2.xml", nil)
	if strings.Contains(text, "SEM VALOR FISCAL") {
		t.Fatalf("watermark was extracted as horizontal text, want rotated upstream-like extraction: %q", text)
	}
}

func TestContingencyWatermarkTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "mdf-e_test_aereo_contingencia.xml", nil)
	for _, want := range []string{"EMISSÃO EM", "CONTINGÊNCIA"} {
		if !strings.Contains(text, want) {
			t.Fatalf("contingency watermark text missing %q in %q", want, text)
		}
	}
}

func TestHeaderFieldsMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "mdf-e_test_1.xml", nil)
	for _, want := range []string{
		"DAMDFE - Documento Auxiliar do Manifesto de Documentos Fiscais Eletrônicos",
		"MODELO",
		"SÉRIE",
		"NÚMERO",
		"FL",
		"DATA E HORA",
		"UF CARREG",
		"UF DESCARREG",
		"FORMA DE EMISSÃO",
		"PREVISÃO DE INICIO DA VIAGEM",
		"TIPO DO EMITENTE",
		"TIPO DO AMBIENTE",
		"INSC. SUFRAMA",
		"CARGA POSTERIOR",
		"CONTROLE DO FISCO",
		"Consulta em https://dfe-portal.svrs.rs.gov.br/MDFE/Consulta",
		"PROTOCOLO DE AUTORIZAÇÃO DE USO",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("header text missing %q in %q", want, text)
		}
	}
}

func TestHeaderTitlePositionTracksGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "mdf-e_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "damdfe", "damdfe_default.pdf")
	actualTitle := mustFindPDFWord(t, actual, "DAMDFE")
	expectedTitle := mustFindPDFWord(t, expected, "DAMDFE")
	if delta := math.Abs(actualTitle.XMin - expectedTitle.XMin); delta > 2 {
		t.Fatalf("DAMDFE title x drifted by %.2f pt: actual=%f expected=%f", delta, actualTitle.XMin, expectedTitle.XMin)
	}
	if delta := math.Abs(actualTitle.YMin - expectedTitle.YMin); delta > 2 {
		t.Fatalf("DAMDFE title y drifted by %.2f pt: actual=%f expected=%f", delta, actualTitle.YMin, expectedTitle.YMin)
	}
}

func TestHeaderBarcodePlacementTracksPythonSVGReference(t *testing.T) {
	actual := renderFixturePDF(t, "mdf-e_test_1.xml", nil)
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`([0-9.]+) ([0-9.]+) ([0-9.]+) -([0-9.]+) re f`)
	matches := re.FindAllSubmatch(data, -1)
	var found int
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := 0.0, 0.0
	for _, match := range matches {
		x := mustParsePDFPoint(t, match[1])
		y := mustParsePDFPoint(t, match[2])
		width := mustParsePDFPoint(t, match[3])
		height := mustParsePDFPoint(t, match[4])
		if x < 300 || x > 570 || y < 700 || y > 750 || width > 10 || height > 40 {
			continue
		}
		found++
		if x < minX {
			minX = x
		}
		if x+width > maxX {
			maxX = x + width
		}
		if y-height < minY {
			minY = y - height
		}
		if y > maxY {
			maxY = y
		}
	}
	if found != 76 {
		t.Fatalf("DAMDFE barcode vector bars = %d, want 76", found)
	}
	if math.Abs(minX-320.97) > 0.5 || math.Abs(maxX-551.19) > 0.5 || math.Abs(minY-704.56) > 0.5 || math.Abs(maxY-734.98) > 0.5 {
		t.Fatalf("DAMDFE barcode vector bounds drifted: minX=%f maxX=%f minY=%f maxY=%f", minX, maxX, minY, maxY)
	}
}

func TestBodySectionPositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "mdf-e_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "damdfe", "damdfe_default.pdf")
	for _, tt := range []struct {
		word string
		xMin float64
		xMax float64
	}{
		{word: "EMITENTE"},
		{word: "TRANSPORTADOR"},
		{word: "MODAL"},
		{word: "INFORMAÇÕES"},
		{word: "MUNICÍPIO", xMin: 0, xMax: 60},
		{word: "SEGUROS"},
		{word: "CIOT"},
		{word: "AUTORIZACAO"},
		{word: "DADOS"},
	} {
		actualWord := mustFindPDFWord(t, actual, tt.word)
		expectedWord := mustFindPDFWord(t, expected, tt.word)
		if tt.xMax > 0 {
			actualWord = mustFindPDFWordInXRange(t, actual, tt.word, tt.xMin, tt.xMax)
			expectedWord = mustFindPDFWordInXRange(t, expected, tt.word, tt.xMin, tt.xMax)
		}
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > 4 {
			t.Fatalf("%s x drifted by %.2f pt: actual=%f expected=%f", tt.word, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > 4 {
			t.Fatalf("%s y drifted by %.2f pt: actual=%f expected=%f", tt.word, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestFiscoTitlePositionTracksGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "mdf-e_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "damdfe", "damdfe_default.pdf")
	actualWords := mustExtractPDFWords(t, actual)
	expectedWords := mustExtractPDFWords(t, expected)
	actualWord := mustFindPDFWordInBox(t, actualWords, "ADICIONAIS", 250, 320, 650, 690)
	expectedWord := mustFindPDFWordInBox(t, expectedWords, "ADICIONAIS", 250, 320, 650, 690)
	if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > 1 {
		t.Fatalf("DAMDFE fisco title x drifted by %.2f pt: actual=%f expected=%f", delta, actualWord.XMin, expectedWord.XMin)
	}
	if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > 1 {
		t.Fatalf("DAMDFE fisco title y drifted by %.2f pt: actual=%f expected=%f", delta, actualWord.YMin, expectedWord.YMin)
	}
}

func TestFiscoInfoStopsBeforeBottomBorderLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "mdf-e_test_1.xml", nil)
	actualWords := mustExtractPDFWords(t, actual)
	var lineCount int
	for _, word := range actualWords {
		if word.Text == "LINHA" && word.XMin < 60 && word.YMin > 790 {
			lineCount++
		}
		if word.Text == "4" && word.XMin < 60 && word.YMin > 820 {
			t.Fatalf("DAMDFE fisco info overflowed past the Python three-line cutoff at x=%f y=%f", word.XMin, word.YMin)
		}
	}
	if lineCount != 3 {
		t.Fatalf("DAMDFE fisco info rendered %d bottom LINHA rows, want 3 like upstream", lineCount)
	}
}

func TestRodoviarioDriverAndRoutePositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "mdf-e_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "damdfe", "damdfe_default.pdf")
	actualWords := mustExtractPDFWords(t, actual)
	expectedWords := mustExtractPDFWords(t, expected)
	for _, anchor := range []struct {
		name      string
		text      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		tolerance float64
	}{
		{name: "driver cpf header", text: "CPF", xMin: 280, xMax: 330, yMin: 205, yMax: 225, tolerance: 1.5},
		{name: "driver name header", text: "CONDUTORES", xMin: 430, xMax: 500, yMin: 215, yMax: 230, tolerance: 1.5},
		{name: "first driver name", text: "CONDUTOR", xMin: 430, xMax: 500, yMin: 225, yMax: 235, tolerance: 1.5},
		{name: "voucher supplier label", text: "CNPJ", xMin: 45, xMax: 75, yMin: 270, yMax: 290, tolerance: 1},
		{name: "voucher responsible label", text: "CPF/CNPJ", xMin: 175, xMax: 220, yMin: 270, yMax: 290, tolerance: 1},
		{name: "voucher number label", text: "NÚMERO", xMin: 310, xMax: 360, yMin: 270, yMax: 290, tolerance: 1},
		{name: "voucher amount label", text: "VALE-PEDÁGIO", xMin: 490, xMax: 560, yMin: 270, yMax: 290, tolerance: 1},
		{name: "route title", text: "PERCURSO", xMin: 240, xMax: 320, yMin: 305, yMax: 325, tolerance: 2},
		{name: "composition title", text: "INFORMAÇÕES", xMin: 210, xMax: 280, yMin: 335, yMax: 350, tolerance: 1},
		{name: "left composition municipality header", text: "MUNICÍPIO", xMin: 0, xMax: 60, yMin: 350, yMax: 365, tolerance: 1},
		{name: "left composition docs header", text: "INFORMAÇÕES", xMin: 95, xMax: 150, yMin: 350, yMax: 365, tolerance: 1},
		{name: "right composition municipality header", text: "MUNICÍPIO", xMin: 270, xMax: 315, yMin: 350, yMax: 365, tolerance: 1},
		{name: "right composition docs header", text: "INFORMAÇÕES", xMin: 360, xMax: 420, yMin: 350, yMax: 365, tolerance: 1},
		{name: "left first document municipality", text: "BRUSQUE", xMin: 0, xMax: 55, yMin: 360, yMax: 370, tolerance: 1},
		{name: "right first document municipality", text: "BRUSQUE", xMin: 270, xMax: 315, yMin: 360, yMax: 370, tolerance: 1},
	} {
		actualWord := mustFindPDFWordInBox(t, actualWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		expectedWord := mustFindPDFWordInBox(t, expectedWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > anchor.tolerance {
			t.Fatalf("DAMDFE rodoviario anchor %q x drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DAMDFE rodoviario anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestBodySectionsMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "mdf-e_test_1.xml", nil)
	for _, want := range []string{
		"MODAL RODOVIÁRIO DE CARGA",
		"INFORMAÇÕES PARA ANTT",
		"CNPJ DA FORNECEDORA",
		"CPF/CNPJ DO RESPONSÁVEL",
		"NÚMERO DO COMPROVANTE",
		"VALOR DO VALE-PEDÁGIO",
		"PERCURSO",
		"INFORMAÇÕES DA COMPOSIÇÃO DA CARGA",
		"MUNICÍPIO",
		"INFORMAÇÕES SOBRE OS SEGUROS",
		"AVERBAÇÃO:",
		"INFORMAÇÕES DO CIOT",
		"RESPONSÁVEL CNPJ",
		"Nº CIOT",
		"INFORMAÇÕES COMPLEMENTARES DE INTERESSE DO CONTRIBUINTE",
		"Informações Complementares",
		"INFORMAÇÕES ADICIONAIS DE INTERESSE DO FISCO",
		"AUTORIZACAO SISTEMA TESTE LTDA",
		"TOTAL MERCADORIA P/ AVERBACAO",
		"FRETE CIF",
		"DESTINO DA PRESTACAO",
		"DADOS FICTICIOS PARA HOMOLOGACAO",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("body text missing %q in %q", want, text)
		}
	}
}

func TestInsuranceCNPJUsesRawXMLDigitsLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "mdf-e_test_1.xml", nil)
	if !strings.Contains(text, "CNPJ: 12345678000199") {
		t.Fatalf("insurance CNPJ missing raw upstream value in %q", text)
	}
	if strings.Contains(text, "CNPJ: 12.345.678/0001-99") {
		t.Fatalf("insurance CNPJ should not be formatted in %q", text)
	}
}

func TestEmptyVoucherValuesStayBlankLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "mdf-e_test_1.xml", nil)
	if strings.Contains(text, "R$ 0,00") {
		t.Fatalf("empty vale-pedagio amount should stay blank like upstream in %q", text)
	}
	if strings.Contains(text, "CNPJ DA FORNECEDORA -") ||
		strings.Contains(text, "CPF/CNPJ DO RESPONSÁVEL -") ||
		strings.Contains(text, "NÚMERO DO COMPROVANTE -") {
		t.Fatalf("empty vale-pedagio cells should not render placeholder hyphens in %q", text)
	}
}

func TestFixtureOutputsMatchGoldenShape(t *testing.T) {
	logo := filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg")
	tests := []struct {
		name     string
		fixture  string
		expected string
		config   Config
	}{
		{
			name:     "default",
			fixture:  "mdf-e_test_1.xml",
			expected: "damdfe_default.pdf",
		},
		{
			name:     "default logo",
			fixture:  "mdf-e_test_1.xml",
			expected: "damdfe_default_logo.pdf",
			config:   Config{Logo: logo},
		},
		{
			name:     "default logo margins",
			fixture:  "mdf-e_test_1.xml",
			expected: "damdfe_default_logo_margins.pdf",
			config:   Config{Logo: logo, Margins: Margins{Top: 10, Right: 10, Bottom: 10, Left: 10}},
		},
		{
			name:     "no authorization",
			fixture:  "mdf-e_test_2.xml",
			expected: "damdfe_no_authorization.pdf",
		},
		{
			name:     "aereo",
			fixture:  "mdf-e_test_aereo.xml",
			expected: "damdfe_aereo.pdf",
			config:   Config{Logo: logo},
		},
		{
			name:     "cte",
			fixture:  "mdf-e_test_3_cte.xml",
			expected: "damdfe_default_cte.pdf",
		},
		{
			name:     "aereo contingencia",
			fixture:  "mdf-e_test_aereo_contingencia.xml",
			expected: "damdfe_aereo_contingencia.pdf",
		},
		{
			name:     "ferroviario",
			fixture:  "mdf-e_test_ferroviario.xml",
			expected: "damdfe_ferroviario.pdf",
		},
		{
			name:     "aquaviario",
			fixture:  "mdf-e_test_aquaviario.xml",
			expected: "damdfe_aquaviario.pdf",
		},
		{
			name:     "multi municipio",
			fixture:  "mdf-e_test_multi_municipio.xml",
			expected: "damdfe_multi_municipio.pdf",
		},
		{
			name:     "origem destino prestacao",
			fixture:  "mdf-e_test_1.xml",
			expected: "damdfe_origem_destino_prestacao.pdf",
			config:   Config{DisplayOrigemDestinoPrestacao: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "damdfe", tt.fixture))
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "damdfe.pdf")
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
			expected := filepath.Join("..", "tests", "generated", "damdfe", tt.expected)
			if err := golden.SamePageCount(out, expected); err != nil {
				t.Fatal(err)
			}
			if golden.PDFInfoAvailable() {
				if err := golden.SamePageGeometry(out, expected, 0.01); err != nil {
					t.Fatal(err)
				}
			}
			if golden.PDFToPPMAvailable() {
				diffs, err := golden.RasterDiffPages(out, expected, 72)
				if err != nil {
					t.Fatal(err)
				}
				if max := golden.MaxMeanAbsoluteError(diffs); max > 0.09 {
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
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "damdfe", fixture))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "damdfe.pdf")
	doc, err := New(string(xmlContent), config)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func mustFindPDFWord(t *testing.T, path string, text string) golden.TextWord {
	t.Helper()
	words := mustExtractPDFWords(t, path)
	for _, word := range words {
		if word.Text == text {
			return word
		}
	}
	t.Fatalf("word %q not found in %s", text, path)
	return golden.TextWord{}
}

func mustFindPDFWordInXRange(t *testing.T, path string, text string, xMin, xMax float64) golden.TextWord {
	t.Helper()
	words := mustExtractPDFWords(t, path)
	for _, word := range words {
		if word.Text == text && word.XMin >= xMin && word.XMin <= xMax {
			return word
		}
	}
	t.Fatalf("word %q in x range %.2f..%.2f not found in %s", text, xMin, xMax, path)
	return golden.TextWord{}
}

func mustExtractPDFWords(t *testing.T, path string) []golden.TextWord {
	t.Helper()
	words, err := golden.ExtractTextWords(path)
	if err != nil {
		t.Fatal(err)
	}
	return words
}

func mustFindPDFWordInBox(t *testing.T, words []golden.TextWord, text string, xMin, xMax, yMin, yMax float64) golden.TextWord {
	t.Helper()
	for _, word := range words {
		if word.Text == text && word.XMin >= xMin && word.XMin <= xMax && word.YMin >= yMin && word.YMin <= yMax {
			return word
		}
	}
	t.Fatalf("word %q in box x %.2f..%.2f y %.2f..%.2f not found", text, xMin, xMax, yMin, yMax)
	return golden.TextWord{}
}

func mustParsePDFPoint(t *testing.T, value []byte) float64 {
	t.Helper()
	parsed, err := strconv.ParseFloat(string(value), 64)
	if err != nil {
		t.Fatalf("parse PDF point %q: %v", value, err)
	}
	return parsed
}
