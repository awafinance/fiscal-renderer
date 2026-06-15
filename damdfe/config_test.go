package damdfe

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
	words, err := golden.ExtractTextWords(path)
	if err != nil {
		t.Fatal(err)
	}
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
	words, err := golden.ExtractTextWords(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range words {
		if word.Text == text && word.XMin >= xMin && word.XMin <= xMax {
			return word
		}
	}
	t.Fatalf("word %q in x range %.2f..%.2f not found in %s", text, xMin, xMax, path)
	return golden.TextWord{}
}
