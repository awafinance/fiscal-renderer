package danfe

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/awafinance/fiscal-renderer/internal/golden"
	"github.com/awafinance/fiscal-renderer/internal/xmlutil"
)

func TestDefaultConfigMatchesPythonDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Margins != (Margins{Top: 5, Right: 5, Bottom: 5, Left: 5}) {
		t.Fatalf("margins = %#v", cfg.Margins)
	}
	if cfg.DecimalConfig != (DecimalConfig{PricePrecision: 4, QuantityPrecision: 4}) {
		t.Fatalf("decimal config = %#v", cfg.DecimalConfig)
	}
	if cfg.ReceiptPosition != ReceiptPositionTop {
		t.Fatalf("receipt position = %q", cfg.ReceiptPosition)
	}
	if cfg.TaxConfiguration != TaxConfigurationStandardICMSIPI {
		t.Fatalf("tax configuration = %q", cfg.TaxConfiguration)
	}
	if cfg.InvoiceDisplay != InvoiceDisplayFullDetails {
		t.Fatalf("invoice display = %q", cfg.InvoiceDisplay)
	}
	if cfg.FontType != FontTypeTimes {
		t.Fatalf("font type = %q", cfg.FontType)
	}
	if cfg.FontSize != FontSizeSmall {
		t.Fatalf("font size = %f", cfg.FontSize)
	}
	if cfg.Logo != "" ||
		len(cfg.LogoBytes) != 0 ||
		cfg.DisplayPISCOFINS ||
		cfg.WatermarkCancelled ||
		cfg.InfCplSemicolonNewline {
		t.Fatalf("zero-value bool/string defaults changed: %#v", cfg)
	}
	if cfg.ProductDescriptionConfig != (ProductDescriptionConfig{DisplayAdditionalInfo: true}) {
		t.Fatalf("product description config = %#v", cfg.ProductDescriptionConfig)
	}
	if cfg.FooterStamp.Logo != "" ||
		len(cfg.FooterStamp.LogoBytes) != 0 ||
		cfg.FooterStamp.Text != "" ||
		cfg.FooterStamp.Height != 5 ||
		cfg.FooterStamp.LogoMaxWidth != 20 ||
		cfg.FooterStamp.Spacing != 1 {
		t.Fatalf("footer stamp = %#v", cfg.FooterStamp)
	}
}

func TestExportedValuesMatchPythonEnums(t *testing.T) {
	expectedTaxConfigurations := map[TaxConfiguration]string{
		TaxConfigurationStandardICMSIPI: "Standard ICMS and IPI",
		TaxConfigurationICMSST:          "ICMS ST only",
		TaxConfigurationWithoutIPI:      "Without IPI fields",
	}
	for value, expected := range expectedTaxConfigurations {
		if string(value) != expected {
			t.Fatalf("tax configuration value = %q, want %q", value, expected)
		}
	}
	if InvoiceDisplayDuplicatesOnly != "Duplicatas Only" || InvoiceDisplayFullDetails != "Full Details" {
		t.Fatalf("invoice display values changed: %q %q", InvoiceDisplayDuplicatesOnly, InvoiceDisplayFullDetails)
	}
	if FontTypeCourier != "Courier" || FontTypeTimes != "Times" {
		t.Fatalf("font type values changed: %q %q", FontTypeCourier, FontTypeTimes)
	}
	if FontSizeSmall != 1.0 || FontSizeBig != 1.35 {
		t.Fatalf("font size values changed: %f %f", FontSizeSmall, FontSizeBig)
	}
	if ReceiptPositionTop != "top" || ReceiptPositionBottom != "bottom" || ReceiptPositionLeft != "left" {
		t.Fatalf("receipt position values changed: %q %q %q", ReceiptPositionTop, ReceiptPositionBottom, ReceiptPositionLeft)
	}
}

func TestPartialFooterStampUsesPythonDefaults(t *testing.T) {
	cfg := normalizeConfig(&Config{FooterStamp: FooterStamp{Text: "Powered by"}})
	if cfg.FooterStamp.Height != 5 || cfg.FooterStamp.LogoMaxWidth != 20 || cfg.FooterStamp.Spacing != 1 {
		t.Fatalf("footer stamp defaults were not preserved: %#v", cfg.FooterStamp)
	}
}

func TestWriteOutputsPDF(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "danfe", "nfe_test_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	logoBytes, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := New(string(xmlContent), &Config{
		LogoBytes: logoBytes,
		FooterStamp: FooterStamp{
			LogoBytes: logoBytes,
			Text:      "Powered by",
		},
	})
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

func TestCancelledWatermarkIsRotatedLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	compactMargins := Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}
	text := renderFixtureText(t, "nfe_with_production_environment.xml", &Config{
		Margins:            compactMargins,
		WatermarkCancelled: true,
		ProductDescriptionConfig: ProductDescriptionConfig{
			DisplayANVISA:         true,
			DisplayAdditionalInfo: false,
		},
	})
	if strings.Contains(text, "CANCELADA") {
		t.Fatalf("cancelled watermark was extracted as horizontal text, want rotated upstream-like extraction: %q", text)
	}
}

func TestProductTableHeadersMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfe_test_1.xml", nil)
	for _, want := range []string{
		"DADOS DO PRODUTO / SERVIÇO",
		"CÓDIGO",
		"DESCRIÇÃO DOS PRODUTOS / SERVIÇOS",
		"NCM/SH",
		"CST",
		"CFOP",
		"UN.",
		"QTD.",
		"V.UNIT.",
		"V.TOTAL",
		"BC.ICMS",
		"V.ICMS",
		"V.IPI",
		"%ICMS",
		"%IPI",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("product table text missing %q in %q", want, text)
		}
	}
}

func TestPickupAndDeliveryLocationBlocksMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfe_retirada_entrega.xml", nil)
	for _, want := range []string{
		"INFORMAÇÕES DO LOCAL DE RETIRADA",
		"DEPOSITO LOCAL DE RETIRADA LTDA",
		"81.583.054/0001-29",
		"078016350838",
		"Rua das Docas, 100, Galpao 5",
		"Mooca",
		"03101-000",
		"São Paulo",
		"SP",
		"(11) 3333-4444",
		"INFORMAÇÕES DO LOCAL DE ENTREGA",
		"ARMAZEM LOCAL DE ENTREGA LTDA",
		"19.602.452/0001-71",
		"123456789",
		"Avenida Brasil, 2000, Bloco B",
		"Jardim America",
		"01430-000",
		"(11) 5555-6666",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("pickup/delivery location text missing %q in %q", want, text)
		}
	}
}

func TestSimplesNacionalProductHeaderUsesCSOSNLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfe_test_sn.xml", nil)
	if !strings.Contains(text, "CSOSN") {
		t.Fatalf("Simples Nacional product header missing CSOSN in %q", text)
	}
	if strings.Contains(text, "CST") {
		t.Fatalf("Simples Nacional product header should not use CST in %q", text)
	}
}

func TestAdditionalProductInfoPreservesBranchPrefixWhitespaceLikeUpstream(t *testing.T) {
	prod, err := xmlutil.ParseString(`<prod><rastro><nLote>ABC</nLote><qLote>1.5</qLote><dFab>2024-01-02</dFab><dVal>2025-03-04</dVal></rastro></prod>`)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.ProductDescriptionConfig = ProductDescriptionConfig{
		DisplayBranch:    true,
		BranchInfoPrefix: " => ",
	}
	got := buildAdditionalProductInfo(prod, "", cfg)
	want := " =>  Lote: ABC Qtd: 1,5000 Fab: 02/01/2024 Val: 04/03/2025"
	if got != want {
		t.Fatalf("additional product branch info = %q, want %q", got, want)
	}
}

func TestAdditionalProductInfoPreservesRawInfAdProdLikeUpstream(t *testing.T) {
	prod, err := xmlutil.ParseString(`<prod></prod>`)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	got := buildAdditionalProductInfo(prod, " alpha   beta\n gamma ", cfg)
	want := " alpha   beta\n gamma "
	if got != want {
		t.Fatalf("additional product info = %q, want %q", got, want)
	}
}

func TestReceiptTextMatchesUpstreamContract(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "danfe", "nfe_test_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	root, err := xmlutil.ParseString(string(xmlContent))
	if err != nil {
		t.Fatal(err)
	}
	data := parseData(root, DefaultConfig())
	got := receiptText(data)
	want := "RECEBEMOS DE Empresa Lucro Presumido Ltda OS PRODUTOS/SERVIÇOS CONSTANTES DA NOTA FISCAL INDICADA ABAIXO. EMISSÃO: 01/01/2020 VALOR TOTAL: 1.950,00 DESTINATARIO: NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL - Rua do Bosque, 238, Barra Funda - São Paulo - SP, CPF: 765.865.078-12"
	if got != want {
		t.Fatalf("receipt text = %q, want %q", got, want)
	}
}

func TestReceiptPositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_default.pdf")
	for _, anchor := range []struct {
		name      string
		word      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		tolerance float64
	}{
		{name: "receipt text first line", word: "RECEBEMOS", xMin: 0, xMax: 80, yMin: 10, yMax: 30, tolerance: 0.5},
		{name: "receipt text second line", word: "AMBIENTE", xMin: 0, xMax: 80, yMin: 18, yMax: 35, tolerance: 0.5},
		{name: "receipt date label", word: "DATA", xMin: 0, xMax: 80, yMin: 35, yMax: 50, tolerance: 0.5},
		{name: "receipt nf title", word: "NF-e", xMin: 500, xMax: 560, yMin: 10, yMax: 30, tolerance: 1.5},
		{name: "receipt nf number", word: "Nº000.000.002", xMin: 500, xMax: 580, yMin: 25, yMax: 45, tolerance: 1.0},
		{name: "receipt series", word: "SÉRIE", xMin: 500, xMax: 560, yMin: 40, yMax: 60, tolerance: 1.0},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, anchor.word, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		expectedWord := mustFindPDFWordInBox(t, expected, anchor.word, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > anchor.tolerance {
			t.Fatalf("DANFE receipt anchor %q x drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DANFE receipt anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestHeaderTextMatchesUpstreamContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfe_test_1.xml", nil)
	for _, want := range []string{
		"DOCUMENTO AUXILIAR DA NOTA FISCAL ELETRÔNICA",
		"0-ENTRADA",
		"1-SAÍDA",
		"FOLHA 1/1",
		"Consulta de autenticidade no portal nacional da NF-e www.nfe.fazenda.gov.br/portal ou no site da Sefaz autorizadora",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("DANFE header text missing %q in %q", want, text)
		}
	}
}

func TestHeaderPositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_default.pdf")
	for _, anchor := range []struct {
		name      string
		word      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		tolerance float64
	}{
		{name: "issuer name", word: "Empresa", xMin: 0, xMax: 120, yMin: 60, yMax: 90, tolerance: 8},
		{name: "issuer address", word: "Avenida", xMin: 40, xMax: 130, yMin: 80, yMax: 120, tolerance: 8},
		{name: "identity title", word: "DANFE", xMin: 200, xMax: 330, yMin: 60, yMax: 90, tolerance: 4},
		{name: "identity subtitle", word: "DOCUMENTO", xMin: 220, xMax: 300, yMin: 70, yMax: 100, tolerance: 4},
		{name: "entry label", word: "0-ENTRADA", xMin: 220, xMax: 300, yMin: 90, yMax: 120, tolerance: 4},
		{name: "access key label", word: "CHAVE", xMin: 300, xMax: 460, yMin: 80, yMax: 120, tolerance: 1},
		{name: "access key value", word: "3520", xMin: 330, xMax: 390, yMin: 90, yMax: 120, tolerance: 1},
		{name: "query text", word: "Consulta", xMin: 300, xMax: 430, yMin: 100, yMax: 140, tolerance: 8},
		{name: "nature row", word: "NATUREZA", xMin: 0, xMax: 80, yMin: 140, yMax: 180, tolerance: 4},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, anchor.word, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		expectedWord := mustFindPDFWordInBox(t, expected, anchor.word, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > anchor.tolerance {
			t.Fatalf("DANFE header anchor %q x drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DANFE header anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestHeaderInvoiceIdentityBlockTracksGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_default.pdf")
	for _, anchor := range []struct {
		word      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		tolerance float64
	}{
		{word: "Nº", xMin: 220, xMax: 280, yMin: 110, yMax: 145, tolerance: 1},
		{word: "SÉRIE", xMin: 220, xMax: 280, yMin: 125, yMax: 155, tolerance: 1},
		{word: "FOLHA", xMin: 220, xMax: 280, yMin: 130, yMax: 165, tolerance: 1},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, anchor.word, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		expectedWord := mustFindPDFWordInBox(t, expected, anchor.word, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > anchor.tolerance {
			t.Fatalf("DANFE identity anchor %q x drifted by %.2f pt: actual=%f expected=%f", anchor.word, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DANFE identity anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.word, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestHeaderBarcodeImageTracksPythonGeometry(t *testing.T) {
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	transform := mustFindPDFImageTransform(t, data)
	if math.Abs(transform.Width-249.45) > 0.5 ||
		math.Abs(transform.Height-24.09) > 0.5 ||
		math.Abs(transform.X-333.08) > 0.5 ||
		math.Abs(transform.Y-748.35) > 0.5 {
		t.Fatalf("DANFE barcode geometry drifted: width=%f height=%f x=%f y=%f", transform.Width, transform.Height, transform.X, transform.Y)
	}
}

func TestHeaderAccessKeyStackDrawsPythonCellDividers(t *testing.T) {
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	rects := findPDFStrokeRects(t, data)
	for _, want := range []struct {
		name       string
		x, y, w, h float64
	}{
		{name: "barcode cell", x: 331.66, y: 773.86, w: 249.45, h: 28.35},
		{name: "access key cell", x: 331.66, y: 745.51, w: 249.45, h: 17.01},
		{name: "verification cell", x: 331.66, y: 728.50, w: 249.45, h: 42.52},
	} {
		if !hasPDFRect(rects, want.x, want.y, want.w, want.h, 0.3) {
			t.Fatalf("DANFE header missing Python %s rectangle", want.name)
		}
	}
	if hasPDFRect(rects, 331.66, 773.86, 249.45, 87.87, 0.3) {
		t.Fatal("DANFE header should not draw one undivided barcode/access-key column")
	}
}

func TestHeaderIssuerPositionsMatchUpstreamCoordinates(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	for _, anchor := range []struct {
		name      string
		word      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		wantXMin  float64
		wantYMin  float64
		tolerance float64
	}{
		{name: "issuer name", word: "Empresa", xMin: 0, xMax: 120, yMin: 60, yMax: 90, wantXMin: 42.98, wantYMin: 72.27, tolerance: 1},
		{name: "issuer address", word: "Avenida", xMin: 80, xMax: 140, yMin: 100, yMax: 120, wantXMin: 100.59, wantYMin: 106.07, tolerance: 1},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, anchor.word, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		if delta := math.Abs(actualWord.XMin - anchor.wantXMin); delta > anchor.tolerance {
			t.Fatalf("DANFE issuer anchor %q x drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.XMin, anchor.wantXMin)
		}
		if delta := math.Abs(actualWord.YMin - anchor.wantYMin); delta > anchor.tolerance {
			t.Fatalf("DANFE issuer anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, anchor.wantYMin)
		}
	}
}

func TestContinuationPageHeaderMatchesUpstreamContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfe_additional_info_continuation_in_next_page.xml", &Config{
		Margins: Margins{Top: 2, Right: 2, Bottom: 2, Left: 2},
		Logo:    filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg"),
	})
	for _, want := range []string{
		"FOLHA 1/2",
		"FOLHA 2/2",
		"DADOS ADICIONAIS",
		"CONTINUAÇÃO INFORMAÇÕES COMPLEMENTARES",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("DANFE continuation text missing %q in %q", want, text)
		}
	}
	if strings.Contains(text, "CONTINUAÇÃO DOS DADOS ADICIONAIS") {
		t.Fatalf("DANFE continuation used non-upstream title in %q", text)
	}
}

func TestContinuationPageIdentityBoxDoesNotOverlapWithContinuationLabel(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_multi_page_products.xml", nil)
	words := extractPageWords(t, actual, 2)
	for _, word := range words {
		if word.Text == "CONTINUAÇÃO" && word.XMin >= 220 && word.XMin <= 340 && word.YMin <= 60 {
			t.Fatalf("DANFE continuation label overlaps the identity header at x=%f y=%f", word.XMin, word.YMin)
		}
	}
}

func TestLandscapeMultipageProductSplitMatchesUpstreamContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	out := renderFixturePDF(t, "nfe_multi_page_products_landscape.xml", &Config{
		Margins:         Margins{Top: 2, Right: 2, Bottom: 2, Left: 2},
		Logo:            filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg"),
		ReceiptPosition: ReceiptPositionTop,
	})
	page1 := extractPageText(t, out, 1, 1)
	page2 := extractPageText(t, out, 2, 2)
	if count := strings.Count(page1, "FURN_9001"); count != 24 {
		t.Fatalf("landscape first page product rows = %d, want 24 in %q", count, page1)
	}
	if count := strings.Count(page2, "FURN_9001"); count != 5 {
		t.Fatalf("landscape continuation product rows = %d, want 5 in %q", count, page2)
	}
	if strings.Contains(page1, "FATURA / DUPLICATAS") {
		t.Fatalf("landscape fixture without cobr should skip empty billing block like upstream in %q", page1)
	}
}

func TestPortraitMultipageProductSplitMatchesUpstreamContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	out := renderFixturePDF(t, "nfe_multi_page_products.xml", nil)
	page1 := extractPageText(t, out, 1, 1)
	page2 := extractPageText(t, out, 2, 2)
	if count := strings.Count(page1, "FURN_9001"); count != 26 {
		t.Fatalf("portrait first page product rows = %d, want 26 in %q", count, page1)
	}
	if count := strings.Count(page2, "FURN_9001"); count != 3 {
		t.Fatalf("portrait continuation product rows = %d, want 3 in %q", count, page2)
	}
	if count := strings.Count(page1, "Este é um produto"); count != 2 {
		t.Fatalf("portrait first page wrapped rows = %d, want 2 in %q", count, page1)
	}
	if count := strings.Count(page2, "Este é um produto"); count != 1 {
		t.Fatalf("portrait continuation wrapped rows = %d, want 1 in %q", count, page2)
	}
}

func TestContinuationProductTablePositionTracksGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	logo := filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg")
	actual := renderFixturePDF(t, "nfe_multi_page_products_landscape.xml", &Config{
		Margins:         Margins{Top: 2, Right: 2, Bottom: 2, Left: 2},
		Logo:            logo,
		ReceiptPosition: ReceiptPositionTop,
	})
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_multipage_landscape.pdf")
	actualWords := extractPageWords(t, actual, 2)
	expectedWords := extractPageWords(t, expected, 2)
	for _, anchor := range []struct {
		name      string
		text      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		tolerance float64
	}{
		{name: "continuation product title", text: "DADOS", xMin: 0, xMax: 40, yMin: 120, yMax: 140, tolerance: 1},
		{name: "continuation code header", text: "CÓDIGO", xMin: 0, xMax: 40, yMin: 130, yMax: 150, tolerance: 1},
		{name: "continuation description header", text: "DESCRIÇÃO", xMin: 45, xMax: 90, yMin: 130, yMax: 150, tolerance: 1},
		{name: "continuation tax base header", text: "BC.ICMS", xMin: 680, xMax: 725, yMin: 130, yMax: 150, tolerance: 1},
	} {
		actualWord := mustFindPDFWordInPageBox(t, actualWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		expectedWord := mustFindPDFWordInPageBox(t, expectedWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > anchor.tolerance {
			t.Fatalf("DANFE continuation anchor %q x drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DANFE continuation anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestInvoiceDisplayMatchesUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	compactMargins := Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}
	fullDetails := renderFixtureText(t, "nfe_overload.xml", &Config{
		Margins:        compactMargins,
		InvoiceDisplay: InvoiceDisplayFullDetails,
	})
	for _, want := range []string{
		"FATURA / DUPLICATAS",
		"NÚMERO",
		"VALOR ORIGINAL",
		"VALOR DO DESCONTO",
		"VALOR LÍQUIDO",
		"00000488-01/01 02/12/2010 540,00",
	} {
		if !strings.Contains(fullDetails, want) {
			t.Fatalf("full invoice display missing %q in %q", want, fullDetails)
		}
	}

	duplicatesOnly := renderFixtureText(t, "nfe_overload.xml", &Config{
		Margins:        compactMargins,
		InvoiceDisplay: InvoiceDisplayDuplicatesOnly,
	})
	if !strings.Contains(duplicatesOnly, "FATURA / DUPLICATAS") {
		t.Fatalf("duplicates-only display should keep upstream billing title in %q", duplicatesOnly)
	}
	if !strings.Contains(duplicatesOnly, "00000488-01/01 02/12/2010 540,00") {
		t.Fatalf("duplicates-only display missing duplicate row in %q", duplicatesOnly)
	}
	for _, unexpected := range []string{"VALOR ORIGINAL", "VALOR DO DESCONTO", "VALOR LÍQUIDO", "R$ 540,00"} {
		if strings.Contains(duplicatesOnly, unexpected) {
			t.Fatalf("duplicates-only display contains %q in %q", unexpected, duplicatesOnly)
		}
	}
}

func TestBillingBlockHeightTracksGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	compactMargins := Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}
	cfg := &Config{
		Margins:          compactMargins,
		Logo:             filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg"),
		ReceiptPosition:  ReceiptPositionBottom,
		DecimalConfig:    DecimalConfig{PricePrecision: 6, QuantityPrecision: 6},
		TaxConfiguration: TaxConfigurationICMSST,
		InvoiceDisplay:   InvoiceDisplayFullDetails,
		FontType:         FontTypeCourier,
	}
	actual := renderFixturePDF(t, "nfe_overload.xml", cfg)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_overload.pdf")
	for _, tt := range []struct {
		word                   string
		xMin, xMax, yMin, yMax float64
	}{
		{word: "FATURA", xMin: 0, xMax: 80, yMin: 240, yMax: 255},
		{word: "CÁLCULO", xMin: 0, xMax: 80, yMin: 275, yMax: 286},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		expectedWord := mustFindPDFWordInBox(t, expected, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > 4 {
			t.Fatalf("%s y drifted by %.2f pt: actual=%f expected=%f", tt.word, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestRecipientAndTaxGridPositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_default.pdf")
	for _, tt := range []struct {
		name      string
		word      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		tolerance float64
	}{
		{name: "recipient address label", word: "ENDEREÇO", xMin: 0, xMax: 80, yMin: 210, yMax: 225, tolerance: 1},
		{name: "recipient district label", word: "BAIRRO", xMin: 270, xMax: 330, yMin: 210, yMax: 225, tolerance: 1},
		{name: "tax first label", word: "BASE", xMin: 0, xMax: 40, yMin: 255, yMax: 265, tolerance: 1},
		{name: "tax total note label", word: "TOTAL", xMin: 450, xMax: 490, yMin: 270, yMax: 285, tolerance: 1},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		expectedWord := mustFindPDFWordInBox(t, expected, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > tt.tolerance {
			t.Fatalf("DANFE grid anchor %q x drifted by %.2f pt: actual=%f expected=%f", tt.name, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > tt.tolerance {
			t.Fatalf("DANFE grid anchor %q y drifted by %.2f pt: actual=%f expected=%f", tt.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
	words, err := golden.ExtractTextWords(actual)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range words {
		if word.Text == "R$" && word.YMin >= 250 && word.YMin <= 285 {
			t.Fatalf("DANFE tax grid rendered currency prefix at x=%f y=%f", word.XMin, word.YMin)
		}
	}
}

func TestShippingGridPositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_default.pdf")
	for _, tt := range []struct {
		name      string
		word      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		tolerance float64
	}{
		{name: "shipping title", word: "TRANSPORTADOR", xMin: 0, xMax: 90, yMin: 285, yMax: 305, tolerance: 1},
		{name: "freight label", word: "FRETE", xMin: 270, xMax: 305, yMin: 295, yMax: 315, tolerance: 1},
		{name: "antt label", word: "CÓDIGO", xMin: 350, xMax: 390, yMin: 295, yMax: 315, tolerance: 1},
		{name: "vehicle plate label", word: "PLACA", xMin: 400, xMax: 440, yMin: 295, yMax: 315, tolerance: 1},
		{name: "quantity label", word: "QUANTIDADE", xMin: 0, xMax: 60, yMin: 330, yMax: 345, tolerance: 1},
		{name: "species label", word: "ESPÉCIE", xMin: 80, xMax: 120, yMin: 330, yMax: 345, tolerance: 1},
		{name: "gross weight label", word: "PESO", xMin: 380, xMax: 410, yMin: 330, yMax: 345, tolerance: 1},
		{name: "product header", word: "CÓDIGO", xMin: 0, xMax: 45, yMin: 355, yMax: 370, tolerance: 1},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		expectedWord := mustFindPDFWordInBox(t, expected, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > tt.tolerance {
			t.Fatalf("DANFE shipping anchor %q x drifted by %.2f pt: actual=%f expected=%f", tt.name, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > tt.tolerance {
			t.Fatalf("DANFE shipping anchor %q y drifted by %.2f pt: actual=%f expected=%f", tt.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestShippingGridDoesNotDrawExtraOuterWrapper(t *testing.T) {
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	rects := findPDFStrokeRects(t, data)
	if hasPDFRect(rects, 14.17, 555.59, 566.93, 51.02, 0.3) {
		t.Fatal("DANFE shipping grid drew an extra full-width wrapper over field rows")
	}
}

func TestProductTableDoesNotDrawExtraOuterWrapper(t *testing.T) {
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	rects := findPDFStrokeRects(t, data)
	if hasPDFRect(rects, 14.17, 501.73, 566.93, 411.02, 0.3) {
		t.Fatal("DANFE product table drew an extra full-height wrapper that crosses neighboring sections")
	}
}

func TestTaxAndTransportLabelsMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	text := renderFixtureText(t, "nfe_with_production_environment.xml", &Config{
		Margins:            Margins{Top: 2, Right: 2, Bottom: 2, Left: 2},
		WatermarkCancelled: true,
		ProductDescriptionConfig: ProductDescriptionConfig{
			DisplayANVISA:         true,
			DisplayAdditionalInfo: false,
		},
	})
	for _, want := range []string{
		"BASE DE CÁLCULO DO ICMS",
		"VALOR APROX. TRIBUTOS",
		"VALOR TOTAL DOS PRODUTOS",
		"OUTRAS DESPESAS ACESSÓRIAS",
		"VALOR DO IPI",
		"VALOR TOTAL DA NOTA",
		"FRETE POR CONTA",
		"CÓDIGO ANTT",
		"PLACA DO VEÍCULO",
		"QUANTIDADE",
		"ESPÉCIE",
		"MARCA",
		"NUMERAÇÃO",
		"PESO BRUTO",
		"PESO LÍQUIDO",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("tax/transport text missing %q in %q", want, text)
		}
	}
}

func TestProductTableVerticalPositionTracksGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_default.pdf")
	actualY := productCodeHeaderY(t, actual)
	expectedY := productCodeHeaderY(t, expected)
	if delta := math.Abs(actualY - expectedY); delta > 8 {
		t.Fatalf("product table header y drifted by %.2f pt: actual=%f expected=%f", delta, actualY, expectedY)
	}
}

func TestDefaultProductTableLayoutTracksGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_default.pdf")
	for _, tt := range []struct {
		word       string
		xMin, xMax float64
		yMin, yMax float64
	}{
		{word: "DADOS", xMin: 0, xMax: 60, yMin: 330, yMax: 380},
		{word: "CÓDIGO", xMin: 0, xMax: 80, yMin: 340, yMax: 380},
		{word: "FURN_9001", xMin: 0, xMax: 80, yMin: 350, yMax: 390},
		{word: "cBenef:", xMin: 40, xMax: 120, yMin: 360, yMax: 400},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		expectedWord := mustFindPDFWordInBox(t, expected, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > 4 {
			t.Fatalf("%s x drifted by %.2f pt: actual=%f expected=%f", tt.word, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > 4 {
			t.Fatalf("%s y drifted by %.2f pt: actual=%f expected=%f", tt.word, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestDefaultAdditionalDataPositionTracksGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "danfe", "danfe_default.pdf")
	for _, tt := range []struct {
		word       string
		xMin, xMax float64
		yMin, yMax float64
	}{
		{word: "DADOS", xMin: 0, xMax: 60, yMin: 730, yMax: 790},
		{word: "INFORMAÇÕES", xMin: 0, xMax: 90, yMin: 740, yMax: 800},
		{word: "RESERVADO", xMin: 350, xMax: 450, yMin: 740, yMax: 800},
		{word: "Documento", xMin: 0, xMax: 90, yMin: 760, yMax: 810},
	} {
		actualWord := mustFindPDFWordInBox(t, actual, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		expectedWord := mustFindPDFWordInBox(t, expected, tt.word, tt.xMin, tt.xMax, tt.yMin, tt.yMax)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > 4 {
			t.Fatalf("%s x drifted by %.2f pt: actual=%f expected=%f", tt.word, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > 4 {
			t.Fatalf("%s y drifted by %.2f pt: actual=%f expected=%f", tt.word, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestDefaultAdditionalDataBoxesTrackGeneratedReference(t *testing.T) {
	actual := renderFixturePDF(t, "nfe_test_1.xml", nil)
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	rects := findPDFStrokeRects(t, data)
	if !hasPDFRect(rects, 14.17, 82.20, 566.93, 2.83, 0.3) {
		t.Fatal("DANFE additional title strip geometry drifted")
	}
	if !hasPDFRect(rects, 14.17, 70.87, 368.51, 56.69, 0.3) {
		t.Fatal("DANFE additional information box geometry drifted")
	}
	if !hasPDFRect(rects, 382.68, 70.87, 198.43, 56.69, 0.3) {
		t.Fatal("DANFE reserved fisco box geometry drifted")
	}
	if hasPDFRect(rects, 14.17, 82.20, 566.93, 56.69, 0.3) {
		t.Fatal("DANFE additional data should not draw the old full-width outer box")
	}
}

func TestFixtureOutputsMatchGoldenShape(t *testing.T) {
	logo := filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg")
	compactMargins := Margins{Top: 2, Right: 2, Bottom: 2, Left: 2}
	defaultLogoConfig := Config{
		Margins:         compactMargins,
		Logo:            logo,
		ReceiptPosition: ReceiptPositionTop,
	}
	productDescAll := ProductDescriptionConfig{
		DisplayANVISA:         true,
		DisplayAdditionalInfo: false,
		DisplayBranch:         true,
		BranchInfoPrefix:      "=>",
		DisplayANP:            true,
	}
	tests := []struct {
		name     string
		fixture  string
		expected string
		config   Config
	}{
		{name: "default", fixture: "nfe_test_1.xml", expected: "danfe_default.pdf"},
		{name: "simples nacional", fixture: "nfe_test_sn.xml", expected: "danfe_sn.pdf"},
		{name: "minimal", fixture: "nfe_test_1.xml", expected: "danfe_minimal.pdf", config: Config{Margins: Margins{Top: 8, Right: 8, Bottom: 8, Left: 8}, DecimalConfig: DecimalConfig{PricePrecision: 2, QuantityPrecision: 2}}},
		{name: "multipage landscape", fixture: "nfe_multi_page_products_landscape.xml", expected: "danfe_multipage_landscape.pdf", config: defaultLogoConfig},
		{name: "add info below product", fixture: "nfe_additional_info_continuation_in_product_table.xml", expected: "danfe_add_info_below_prod.pdf", config: defaultLogoConfig},
		{name: "add info next page", fixture: "nfe_additional_info_continuation_in_next_page.xml", expected: "danfe_add_info_next_page.pdf", config: defaultLogoConfig},
		{name: "overload", fixture: "nfe_overload.xml", expected: "danfe_overload.pdf", config: Config{Margins: compactMargins, Logo: logo, ReceiptPosition: ReceiptPositionBottom, DecimalConfig: DecimalConfig{PricePrecision: 6, QuantityPrecision: 6}, TaxConfiguration: TaxConfigurationICMSST, InvoiceDisplay: InvoiceDisplayFullDetails, FontType: FontTypeCourier}},
		{name: "duplicatas only", fixture: "nfe_overload.xml", expected: "danfe_duplicatas_only.pdf", config: Config{Margins: compactMargins, InvoiceDisplay: InvoiceDisplayDuplicatesOnly}},
		{name: "pis cofins", fixture: "nfe_test_1.xml", expected: "danfe_pis_confins.pdf", config: Config{Margins: compactMargins, DisplayPISCOFINS: true}},
		{name: "branch", fixture: "nfe_test_branch.xml", expected: "danfe_branch.pdf", config: Config{Margins: compactMargins, ProductDescriptionConfig: ProductDescriptionConfig{DisplayBranch: true, DisplayAdditionalInfo: false}}},
		{name: "branch with prefix", fixture: "nfe_test_branch.xml", expected: "danfe_branch_with_prefix.pdf", config: Config{Margins: compactMargins, ProductDescriptionConfig: ProductDescriptionConfig{DisplayBranch: true, BranchInfoPrefix: "=>", DisplayAdditionalInfo: true}}},
		{name: "anp", fixture: "nfe_test_anp.xml", expected: "danfe_anp.pdf", config: Config{Margins: compactMargins, ProductDescriptionConfig: ProductDescriptionConfig{DisplayANP: true, DisplayAdditionalInfo: false}}},
		{name: "anvisa", fixture: "nfe_test_anvisa.xml", expected: "danfe_anvisa.pdf", config: Config{Margins: compactMargins, ProductDescriptionConfig: ProductDescriptionConfig{DisplayANVISA: true, DisplayAdditionalInfo: false}}},
		{name: "with production environment", fixture: "nfe_with_production_environment.xml", expected: "danfe_with_production_environment.pdf", config: Config{Margins: compactMargins, WatermarkCancelled: true, ProductDescriptionConfig: ProductDescriptionConfig{DisplayANVISA: true, DisplayAdditionalInfo: false}}},
		{name: "without production environment", fixture: "nfe_without_production_environment.xml", expected: "danfe_without_production_environment.pdf", config: Config{Margins: compactMargins, WatermarkCancelled: true, ProductDescriptionConfig: ProductDescriptionConfig{DisplayANVISA: true, DisplayAdditionalInfo: false}}},
		{name: "default production", fixture: "nfe_with_production_environment.xml", expected: "danfe_default_production.pdf", config: Config{Margins: compactMargins, ProductDescriptionConfig: ProductDescriptionConfig{DisplayANVISA: true, DisplayAdditionalInfo: false}}},
		{name: "reforma tributaria", fixture: "nfe_reforma_tributaria.xml", expected: "danfe_reforma_tributaria.pdf"},
		{name: "big font size", fixture: "nfe_big_font_size.xml", expected: "danfe_big_font_size.pdf", config: Config{Margins: compactMargins, ProductDescriptionConfig: productDescAll, DecimalConfig: DecimalConfig{PricePrecision: 3, QuantityPrecision: 2}, FontSize: FontSizeBig}},
		{name: "infcpl semicolon newline", fixture: "nfe_semicolon_line_break.xml", expected: "danfe_infcpl_semicolon_newline.pdf", config: Config{Margins: compactMargins, ProductDescriptionConfig: productDescAll, DecimalConfig: DecimalConfig{PricePrecision: 3, QuantityPrecision: 2}, FontSize: FontSizeBig, InfCplSemicolonNewline: true}},
		{name: "mei", fixture: "nfe_mei.xml", expected: "danfe_mei.pdf", config: Config{Margins: compactMargins, ProductDescriptionConfig: productDescAll, DecimalConfig: DecimalConfig{PricePrecision: 3, QuantityPrecision: 2}, FontSize: FontSizeBig, InfCplSemicolonNewline: true}},
		{name: "footer stamp", fixture: "nfe_test_1.xml", expected: "danfe_footer_stamp.pdf", config: Config{Margins: compactMargins, FooterStamp: FooterStamp{Logo: logo, Text: "Powered by"}}},
		{name: "footer stamp text only", fixture: "nfe_test_1.xml", expected: "danfe_footer_stamp_text_only.pdf", config: Config{Margins: compactMargins, FooterStamp: FooterStamp{Text: "Powered by Engenere"}}},
		{name: "footer stamp logo only", fixture: "nfe_test_1.xml", expected: "danfe_footer_stamp_logo_only.pdf", config: Config{Margins: compactMargins, FooterStamp: FooterStamp{Logo: logo}}},
		{name: "footer stamp multipage", fixture: "nfe_additional_info_continuation_in_next_page.xml", expected: "danfe_footer_stamp_multipage.pdf", config: Config{Margins: compactMargins, FooterStamp: FooterStamp{Logo: logo, Text: "Powered by"}}},
		{name: "retirada", fixture: "nfe_retirada.xml", expected: "danfe_retirada.pdf"},
		{name: "retirada entrega", fixture: "nfe_retirada_entrega.xml", expected: "danfe_retirada_entrega.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "danfe", tt.fixture))
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "danfe.pdf")
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
			expected := filepath.Join("..", "tests", "generated", "danfe", tt.expected)
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
				if max := golden.MaxMeanAbsoluteError(diffs); max > 0.14 {
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
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "danfe", fixture))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "danfe.pdf")
	doc, err := New(string(xmlContent), config)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func extractPageText(t *testing.T, path string, firstPage, lastPage int) string {
	t.Helper()
	output, err := exec.Command("pdftotext", "-enc", "UTF-8", "-f", fmt.Sprint(firstPage), "-l", fmt.Sprint(lastPage), path, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	return golden.NormalizeExtractedText(string(output))
}

func extractPageWords(t *testing.T, path string, page int) []golden.TextWord {
	t.Helper()
	output, err := exec.Command("pdftotext", "-enc", "UTF-8", "-f", fmt.Sprint(page), "-l", fmt.Sprint(page), "-bbox", path, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	decoder := xml.NewDecoder(bytes.NewReader(output))
	var words []golden.TextWord
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "word" {
			continue
		}
		var word struct {
			XMin float64 `xml:"xMin,attr"`
			YMin float64 `xml:"yMin,attr"`
			XMax float64 `xml:"xMax,attr"`
			YMax float64 `xml:"yMax,attr"`
			Text string  `xml:",chardata"`
		}
		if err := decoder.DecodeElement(&word, &start); err != nil {
			t.Fatal(err)
		}
		words = append(words, golden.TextWord{
			Text: strings.TrimSpace(word.Text),
			XMin: word.XMin,
			YMin: word.YMin,
			XMax: word.XMax,
			YMax: word.YMax,
		})
	}
	return words
}

func mustFindPDFWordInPageBox(t *testing.T, words []golden.TextWord, text string, xMin, xMax, yMin, yMax float64) golden.TextWord {
	t.Helper()
	for _, word := range words {
		if word.Text == text && word.XMin >= xMin && word.XMin <= xMax && word.YMin >= yMin && word.YMin <= yMax {
			return word
		}
	}
	t.Fatalf("word %q not found in page box x=[%f,%f] y=[%f,%f]", text, xMin, xMax, yMin, yMax)
	return golden.TextWord{}
}

type pdfRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type pdfImageTransform struct {
	Width  float64
	Height float64
	X      float64
	Y      float64
}

func mustFindPDFImageTransform(t *testing.T, data []byte) pdfImageTransform {
	t.Helper()
	re := regexp.MustCompile(`q ([0-9.]+) 0 0 ([0-9.]+) ([0-9.]+) ([0-9.]+) cm /[^ ]+ Do Q`)
	match := re.FindSubmatch(data)
	if match == nil {
		t.Fatal("PDF image transform not found")
	}
	return pdfImageTransform{
		Width:  mustParseFloat(t, match[1]),
		Height: mustParseFloat(t, match[2]),
		X:      mustParseFloat(t, match[3]),
		Y:      mustParseFloat(t, match[4]),
	}
}

func findPDFStrokeRects(t *testing.T, data []byte) []pdfRect {
	t.Helper()
	re := regexp.MustCompile(`([0-9.]+) ([0-9.]+) ([0-9.]+) -([0-9.]+) re S`)
	matches := re.FindAllSubmatch(data, -1)
	rects := make([]pdfRect, 0, len(matches))
	for _, match := range matches {
		rects = append(rects, pdfRect{
			X:      mustParseFloat(t, match[1]),
			Y:      mustParseFloat(t, match[2]),
			Width:  mustParseFloat(t, match[3]),
			Height: mustParseFloat(t, match[4]),
		})
	}
	return rects
}

func hasPDFRect(rects []pdfRect, x, y, width, height, tolerance float64) bool {
	for _, rect := range rects {
		if math.Abs(rect.X-x) <= tolerance &&
			math.Abs(rect.Y-y) <= tolerance &&
			math.Abs(rect.Width-width) <= tolerance &&
			math.Abs(rect.Height-height) <= tolerance {
			return true
		}
	}
	return false
}

func mustParseFloat(t *testing.T, value []byte) float64 {
	t.Helper()
	parsed, err := strconv.ParseFloat(string(value), 64)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func productCodeHeaderY(t *testing.T, path string) float64 {
	t.Helper()
	words, err := golden.ExtractTextWords(path)
	if err != nil {
		t.Fatal(err)
	}
	y := -1.0
	for _, word := range words {
		if word.Text == "CÓDIGO" && word.XMin < 80 && word.YMin > y {
			y = word.YMin
		}
	}
	if y < 0 {
		t.Fatalf("product CÓDIGO header not found in %s", path)
	}
	return y
}

func mustFindPDFWordInBox(t *testing.T, path string, text string, xMin, xMax, yMin, yMax float64) golden.TextWord {
	t.Helper()
	words, err := golden.ExtractTextWords(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range words {
		if word.Text == text && word.XMin >= xMin && word.XMin <= xMax && word.YMin >= yMin && word.YMin <= yMax {
			return word
		}
	}
	t.Fatalf("word %q not found in %s inside x %.2f..%.2f y %.2f..%.2f", text, path, xMin, xMax, yMin, yMax)
	return golden.TextWord{}
}
