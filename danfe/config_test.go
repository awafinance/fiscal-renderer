package danfe

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
