package dacte

import (
	"bytes"
	"fmt"
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
	if cfg.FontType != FontTypeTimes {
		t.Fatalf("font type = %q", cfg.FontType)
	}
	if cfg.Logo != "" || cfg.WatermarkCancelled || cfg.DisplayIBSCBS {
		t.Fatalf("zero-value bool/string defaults changed: %#v", cfg)
	}
}

func TestExportedValuesMatchPythonEnums(t *testing.T) {
	if FontTypeCourier != "Courier" || FontTypeTimes != "Times" {
		t.Fatalf("font type values changed: %q %q", FontTypeCourier, FontTypeTimes)
	}
	if ReceiptPositionTop != "top" || ReceiptPositionBottom != "bottom" || ReceiptPositionLeft != "left" {
		t.Fatalf("receipt position values changed: %q %q %q", ReceiptPositionTop, ReceiptPositionBottom, ReceiptPositionLeft)
	}
	expectedModals := map[ModalType]string{
		ModalTypeRodoviario:  "RODOVIÁRIO",
		ModalTypeAereo:       "AÉREO",
		ModalTypeAquaviario:  "AQUAVIÁRIO",
		ModalTypeFerroviario: "FERROVIÁRIO",
		ModalTypeDutoviario:  "DUTOVIÁRIO",
		ModalTypeMultimodal:  "MULTIMODAL",
	}
	for modal, expected := range expectedModals {
		if string(modal) != expected {
			t.Fatalf("modal value = %q, want %q", modal, expected)
		}
	}
}

func TestFontTypeCourierIsUsedForDocumentTextLikeUpstream(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", "dacte_test_1.xml"))
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
		t.Fatal("DACTE Courier config did not produce Courier text font")
	}
	if bytes.Contains(out.Bytes(), []byte("/BaseFont /Times")) {
		t.Fatal("DACTE Courier config still produced Times base font")
	}
}

func TestReceiptFieldsAreRendered(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", "dacte_test_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dacte.pdf")
	doc, err := New(string(xmlContent), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	text = golden.NormalizeExtractedText(text)
	for _, want := range []string{
		"NOME",
		"RG",
		"ASSINATURA / CARIMBO",
		"CHEGADA DATA/HORA",
		"SAÍDA DATA/HORA",
		"CT-E",
		"NRO. DOCUMENTO",
		"SÉRIE",
		"99203223",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("receipt text missing %q in %q", want, text)
		}
	}
}

func TestHeaderFieldsMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", "dacte_test_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dacte.pdf")
	doc, err := New(string(xmlContent), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	text = golden.NormalizeExtractedText(text)
	for _, want := range []string{
		"DOCUMENTO AUXILIAR DO CONHECIMENTO DE TRANSPORTE ELETRÔNICO",
		"TIPO DO CT-E",
		"DATA E HORA DE EMISSÃO",
		"27/03/2024 00:00:00",
		"FL",
		"1/1",
		"CHAVE DE ACESSO",
		"CONSULTA EM http://www.cte.fazenda.gov.br",
		"PROTOCOLO DE AUTORIZAÇÃO DE USO",
		"CFOP - NATUREZA DA PRESTAÇÃO",
		"6353 - PRESTACAO DE SERVICO",
		"INÍCIO DA PRESTAÇÃO",
		"SAO JOAO BATISTA - SC",
		"TÉRMINO DA PRESTAÇÃO",
		"VARGEM GRANDE PAULISTA - SP",
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
	actual := renderFixturePDF(t, "dacte_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "dacte", "dacte_default.pdf")
	actualWords := mustExtractPDFWords(t, actual)
	expectedWords := mustExtractPDFWords(t, expected)
	actualTitle := mustFindPDFWordInBox(t, actualWords, "DACTE", 0, 600, 0, 200, 0)
	expectedTitle := mustFindPDFWordInBox(t, expectedWords, "DACTE", 0, 600, 0, 200, 0)
	if delta := math.Abs(actualTitle.XMin - expectedTitle.XMin); delta > 8 {
		t.Fatalf("DACTE title x drifted by %.2f pt: actual=%f expected=%f", delta, actualTitle.XMin, expectedTitle.XMin)
	}
	if delta := math.Abs(actualTitle.YMin - expectedTitle.YMin); delta > 3 {
		t.Fatalf("DACTE title y drifted by %.2f pt: actual=%f expected=%f", delta, actualTitle.YMin, expectedTitle.YMin)
	}
	for _, anchor := range []struct {
		name      string
		text      string
		xMin      float64
		xMax      float64
		yMin      float64
		yMax      float64
		tolerance float64
	}{
		{name: "model value", text: "57", xMin: 180, xMax: 260, yMin: 100, yMax: 140, tolerance: 3},
		{name: "type value", text: "NORMAL", xMin: 0, xMax: 80, yMin: 150, yMax: 180, tolerance: 3},
		{name: "tomador label", text: "TOMADOR", xMin: 0, xMax: 120, yMin: 180, yMax: 220, tolerance: 3},
		{name: "tomador value", text: "REMETENTE", xMin: 0, xMax: 100, yMin: 190, yMax: 230, tolerance: 3},
		{name: "cfop value", text: "6353", xMin: 0, xMax: 80, yMin: 220, yMax: 250, tolerance: 3},
	} {
		actualWord := mustFindPDFWordInBox(t, actualWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax, 0)
		expectedWord := mustFindPDFWordInBox(t, expectedWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax, 0)
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DACTE header anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestReceiptPositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "dacte_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "dacte", "dacte_default.pdf")
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
		{name: "receipt text", text: "DECLARO", xMin: 0, xMax: 80, yMin: 0, yMax: 40, tolerance: 1},
		{name: "name label", text: "NOME", xMin: 0, xMax: 80, yMin: 20, yMax: 50, tolerance: 1},
		{name: "arrival label", text: "CHEGADA", xMin: 300, xMax: 380, yMin: 20, yMax: 50, tolerance: 1},
		{name: "series label", text: "SÉRIE", xMin: 430, xMax: 500, yMin: 45, yMax: 70, tolerance: 1},
		{name: "signature label", text: "ASSINATURA", xMin: 150, xMax: 230, yMin: 55, yMax: 80, tolerance: 1},
	} {
		actualWord := mustFindPDFWordInBox(t, actualWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax, 0)
		expectedWord := mustFindPDFWordInBox(t, expectedWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax, 0)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > anchor.tolerance {
			t.Fatalf("DACTE receipt anchor %q x drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DACTE receipt anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestHeaderQRCodePlacementTracksGeneratedReference(t *testing.T) {
	actual := renderFixturePDF(t, "dacte_test_1.xml", nil)
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`q ([0-9.]+) 0 0 ([0-9.]+) ([0-9.]+) ([0-9.]+) cm /I[0-9a-f]+ Do Q`)
	matches := re.FindAllSubmatch(data, -1)
	for _, match := range matches {
		width := mustParsePDFPoint(t, match[1])
		height := mustParsePDFPoint(t, match[2])
		if math.Abs(width-height) > 0.1 || width < 100 {
			continue
		}
		x := mustParsePDFPoint(t, match[3])
		y := mustParsePDFPoint(t, match[4])
		if math.Abs(width-107.72) > 0.2 || math.Abs(x-456.38) > 0.5 || math.Abs(y-626.46) > 0.5 {
			t.Fatalf("DACTE QR transform drifted: width=%f x=%f y=%f", width, x, y)
		}
		return
	}
	t.Fatalf("DACTE QR image transform not found in %s", actual)
}

func TestHeaderBarcodePlacementTracksGeneratedReference(t *testing.T) {
	actual := renderFixturePDF(t, "dacte_test_1.xml", nil)
	data, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`q ([0-9.]+) 0 0 ([0-9.]+) ([0-9.]+) ([0-9.]+) cm /I[0-9a-f]+ Do Q`)
	matches := re.FindAllSubmatch(data, -1)
	for _, match := range matches {
		width := mustParsePDFPoint(t, match[1])
		height := mustParsePDFPoint(t, match[2])
		if width < 200 || height > 30 {
			continue
		}
		x := mustParsePDFPoint(t, match[3])
		y := mustParsePDFPoint(t, match[4])
		if math.Abs(width-232.44) > 0.5 || math.Abs(height-24.09) > 0.5 || math.Abs(x-206.93) > 0.5 || math.Abs(y-666.06) > 0.5 {
			t.Fatalf("DACTE barcode transform drifted: width=%f height=%f x=%f y=%f", width, height, x, y)
		}
		return
	}
	t.Fatalf("DACTE barcode image transform not found in %s", actual)
}

func TestBodyVerticalPositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "dacte_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "dacte", "dacte_default.pdf")
	for _, text := range []string{"COMPONENTES", "TRIBUTAÇÃO", "DOCUMENTOS", "OBSERVAÇÕES", "DADOS"} {
		actualWord := mustFindPDFWord(t, actual, text)
		expectedWord := mustFindPDFWord(t, expected, text)
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > 8 {
			t.Fatalf("DACTE body anchor %q y drifted by %.2f pt: actual=%f expected=%f", text, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestValueAndDocumentGridPositionsTrackGeneratedReference(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	actual := renderFixturePDF(t, "dacte_test_1.xml", nil)
	expected := filepath.Join("..", "tests", "generated", "dacte", "dacte_default.pdf")
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
		{name: "components title", text: "COMPONENTES", xMin: 150, xMax: 230, yMin: 430, yMax: 470, tolerance: 3},
		{name: "component name header", text: "NOME", xMin: 0, xMax: 80, yMin: 440, yMax: 480, tolerance: 3},
		{name: "component value header", text: "VALOR", xMin: 70, xMax: 130, yMin: 440, yMax: 480, tolerance: 3},
		{name: "component value", text: "117.78", xMin: 70, xMax: 130, yMin: 450, yMax: 490, tolerance: 3},
		{name: "fourth component", text: "PEDAGIO", xMin: 0, xMax: 80, yMin: 480, yMax: 510, tolerance: 3},
		{name: "total value title", text: "VALOR", xMin: 400, xMax: 460, yMin: 440, yMax: 470, tolerance: 3},
		{name: "documents title", text: "DOCUMENTOS", xMin: 200, xMax: 280, yMin: 540, yMax: 580, tolerance: 4},
		{name: "document type", text: "NFE", xMin: 0, xMax: 60, yMin: 560, yMax: 590, tolerance: 3},
		{name: "document series", text: "413/849104089", xMin: 180, xMax: 260, yMin: 560, yMax: 590, tolerance: 3},
		{name: "observations title", text: "OBSERVAÇÕES", xMin: 220, xMax: 320, yMin: 660, yMax: 700, tolerance: 3},
	} {
		actualWord := mustFindPDFWordInBox(t, actualWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax, 0)
		expectedWord := mustFindPDFWordInBox(t, expectedWords, anchor.text, anchor.xMin, anchor.xMax, anchor.yMin, anchor.yMax, 0)
		if delta := math.Abs(actualWord.XMin - expectedWord.XMin); delta > anchor.tolerance {
			t.Fatalf("DACTE body anchor %q x drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.XMin, expectedWord.XMin)
		}
		if delta := math.Abs(actualWord.YMin - expectedWord.YMin); delta > anchor.tolerance {
			t.Fatalf("DACTE body anchor %q y drifted by %.2f pt: actual=%f expected=%f", anchor.name, delta, actualWord.YMin, expectedWord.YMin)
		}
	}
}

func TestBodySectionsMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", "dacte_test_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dacte.pdf")
	doc, err := New(string(xmlContent), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	text = golden.NormalizeExtractedText(text)
	for _, want := range []string{
		"REMETENTE",
		"DESTINATÁRIO",
		"ENDEREÇO",
		"MUNICÍPIO",
		"CNPJ/CPF",
		"PAÍS",
		"CEP",
		"IE",
		"FONE",
		"TOMADOR DO SERVIÇO",
		"VALOR TOTAL DA CARGA",
		"PRODUTO PREDOMINATE",
		"TIPO MEDIDA",
		"QTD/UN.",
		"CUBAGEM (M³)",
		"QTD DE VOLUMES",
		"COMPONENTES DO VALOR DA PRESTAÇÃO DO SERVIÇO",
		"NOME",
		"VALOR",
		"117.78",
		"4.24",
		"13.11",
		"79.64",
		"3.15",
		"3.27",
		"VALOR TOTAL DO SERVIÇO",
		"VALOR TOTAL A RECEBER",
		"INFORMAÇÕES RELATIVAS AO IMPOSTO",
		"SITUAÇÃO TRIBUTÁRIA",
		"BASE DE CALCULO",
		"ALÍQ ICMS",
		"VALOR ICMS",
		"% RED. BC ICMS",
		"ICMS ST",
		"00 - TRIBUTAÇÃO NORMAL",
		"DOCUMENTOS ORIGINÁRIOS",
		"TIPO DOC",
		"CNPJ/CHAVE",
		"SÉRIE/NRO. DOCUMENTO",
		"413/849104089",
		"OBSERVAÇÕES",
		"DADOS ESPECÍFICOS DO MODAL RODOVIÁRIO - CARGA FRACIONADA",
		"RNTRC DA EMPRESA",
		"CIOT",
		"DATA PREVISTA DE ENTREGA",
		"USO EXCLUSIVO DO EMISSOR DO CT-E",
		"ESTE CONHECIMENTO DE TRANSPORTE ATENDE",
		"ATENDEÀ",
		"VIGOR",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("body text missing %q in %q", want, text)
		}
	}
}

func TestMissingExpeditorReceiverDoNotRenderDashPlaceholdersLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", "dacte_test_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dacte.pdf")
	doc, err := New(string(xmlContent), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	text = golden.NormalizeExtractedText(text)
	for _, unexpected := range []string{"EXPEDIDOR -", "RECEBEDOR -"} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("missing party placeholder %q should be blank like upstream in %q", unexpected, text)
		}
	}
}

func TestObservationTextIgnoresXObsLikeUpstream(t *testing.T) {
	root, err := xmlutil.ParseString(`<compl>
		<xObs>ignored free-form observation</xObs>
		<ObsCont xCampo="1"><xTexto>kept continuation observation</xTexto></ObsCont>
		<ObsFisco xCampo="2"><xTexto>kept fiscal observation</xTexto></ObsFisco>
	</compl>`)
	if err != nil {
		t.Fatal(err)
	}
	text := observationText(parseObservations(root))
	if strings.Contains(text, "ignored free-form observation") {
		t.Fatalf("xObs text should be ignored like upstream, got %q", text)
	}
	for _, want := range []string{"kept continuation observation", "kept fiscal observation"} {
		if !strings.Contains(text, want) {
			t.Fatalf("observation text missing %q in %q", want, text)
		}
	}
}

func TestMultiPageContinuationRepeatsUpstreamDACTEHeader(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", "dacte_test_multi_pages.xml"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dacte.pdf")
	doc, err := New(string(xmlContent), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	text = golden.NormalizeExtractedText(text)
	for _, want := range []string{
		"DOCUMENTO AUXILIAR DO CONHECIMENTO DE TRANSPORTE ELETRÔNICO",
		"2/2",
		"DOCUMENTOS ORIGINÁRIOS",
		"OBSERVAÇÕES",
		"Seguradora: 12345678901234",
		"eguradora: 12345678901234 Texto fictício para teste: Informação exemplo 3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("continuation text missing %q in %q", want, text)
		}
	}
	for _, unexpected := range []string{
		"DACTE - Continuação das Observações",
		"Texto fictício para teste: Informação exemplo 1",
		"... mais",
	} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("continuation text should not contain %q in %q", unexpected, text)
		}
	}
	page2Text := extractPageText(t, out, 2, 2)
	if strings.Contains(page2Text, "62367248427294724924247484294724947224924724") {
		t.Fatalf("continuation page should start after the first 24 upstream origin docs, got early document in %q", page2Text)
	}
}

func TestHomologationWatermarkIsRotatedLikeUpstream(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", "dacte_test_1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dacte.pdf")
	doc, err := New(string(xmlContent), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Output(out); err != nil {
		t.Fatal(err)
	}
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	text = golden.NormalizeExtractedText(text)
	if strings.Contains(text, "SEM VALOR FISCAL") {
		t.Fatalf("watermark was extracted as horizontal text, want rotated upstream-like extraction: %q", text)
	}
}

func TestModalSpecificSectionsMatchUpstreamTextContract(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	tests := []struct {
		name    string
		fixture string
		want    []string
	}{
		{
			name:    "aereo",
			fixture: "dacte_aereo_test.xml",
			want: []string{
				"DADOS ESPECÍFICOS DO MODAL AÉREO",
				"NÚMERO OPERACIONAL AÉREO",
				"CLASSE",
				"CÓDIGO DA TARIFA",
				"VALOR DA TARIFA",
				"DA MINUTA",
				"RETIRA",
				"RELATIVOS A RETIRADA DA CARGA",
				"CARACTERÍSTICAS ADICIONAL DO SERVIÇO",
				"DATA PREVISTA DA ENTREGA",
				"INFORMAÇÕES DE MANUSEIO",
				"DIMENSÃO",
				"OCA56789",
				"TEST_CL",
				"TAR123",
				"R$ 0,00",
				"TEST123",
				"2024-11-22",
				"Dimensão Padrão",
			},
		},
		{
			name:    "aquaviario",
			fixture: "dacte_aquaviario_test.xml",
			want: []string{
				"INFORMAÇÕES ESPECÍFICAS DO MODAL AQUAVIÁRIO",
				"LACRE",
				"IDENTIFICAÇÃO DO CONTAINER",
				"IDENTIFICAÇÃO DO NAVIO / REBOCADOR",
				"IDENTIFICAÇÃO DA BALSA",
				"VLR DO AFRMM",
				"Navio Mercante 123",
				"Balsa A",
				"Balsa B",
				"R$ 1.200,00",
			},
		},
		{
			name:    "ferroviario",
			fixture: "dacte_ferroviario_test.xml",
			want: []string{
				"INFORMAÇÕES ESPECÍFICAS DO MODAL FERROVIÁRIO",
				"TIPO DE TRÁFICO",
				"FLUXO FERROVIÁRIO",
				"VALOR DO FRETE",
				"FERROVIA EMITENTE DO CT-E",
				"FERROVIA DO FATURAMENTO",
				"DAS FERROVIARIAS ENVOLVIDAS",
				"COD. INTERNO",
				"RAZÃO SOCIAL",
				"MÚTUO",
				"Fluxo Norte-Sul",
				"0001",
				"8923902389",
				"TESTE",
			},
		},
		{
			name:    "dutoviario",
			fixture: "dacte_dutoviario_test.xml",
			want: []string{
				"DADOS ESPECÍFICOS DO MODAL DUTOVIÁRIO",
				"VALOR UNITÁRIO",
				"VALOR DO FRETE",
				"OUTROS",
				"BASE DE CÁLCULO",
				"ALÍQUOTA",
				"VALOR DO IMPOSTO",
				"VALOR TOTAL DO FRETE",
				"OBSERVAÇÕES",
				"SÉRIE",
				"NÚMERO",
				"EMITENTE",
				"R$ 1.500,00",
			},
		},
		{
			name:    "multimodal",
			fixture: "dacte_multimodal_test.xml",
			want: []string{
				"INFORMAÇÕES E ESPECIFICAÇÕES DO TRANSPORTE MULTIMODAL DE CAMADAS",
				"Nº DO CERTIFICADO DO OPERADOR DE TRANSPORTE MULTIMODAL",
				"INDICADOR NEGOCIÁVEL",
				"NEGOCIÁVEL",
				"NÃO NEGOCIÁVEL",
				"CNPJ DA SEGURADO",
				"NOME DA SEGURADO",
				"NÚMERO DA APÓLICE",
				"NÚMERO DE AVERBAÇÃO",
				"0001111",
				"TESTE",
				"23423423455409",
				"001",
				"002",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", tt.fixture))
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "dacte.pdf")
			doc, err := New(string(xmlContent), nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := doc.Output(out); err != nil {
				t.Fatal(err)
			}
			text, err := golden.ExtractText(out)
			if err != nil {
				t.Fatal(err)
			}
			text = golden.NormalizeExtractedText(text)
			for _, want := range tt.want {
				if !strings.Contains(text, want) {
					t.Fatalf("modal text missing %q in %q", want, text)
				}
			}
		})
	}
}

func TestWriteOutputsPDF(t *testing.T) {
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", "dacte_test_1.xml"))
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

func TestFixtureOutputsMatchGoldenShape(t *testing.T) {
	logo := filepath.Join("..", "tests", "fixtures", "logo-engenere.jpg")
	defaultLogoConfig := Config{Logo: logo}
	tests := []struct {
		name     string
		fixture  string
		expected string
		config   Config
	}{
		{name: "default", fixture: "dacte_test_1.xml", expected: "dacte_default.pdf"},
		{name: "without compl", fixture: "dacte_test_without_compl.xml", expected: "dacte_without_compl.pdf"},
		{name: "overload", fixture: "dacte_test_overload.xml", expected: "dacte_overload.pdf", config: Config{Margins: Margins{Top: 10, Right: 10, Bottom: 10, Left: 10}}},
		{name: "multi pages", fixture: "dacte_test_multi_pages.xml", expected: "dacte_multi_pages.pdf"},
		{name: "logo", fixture: "dacte_test_1.xml", expected: "dacte_default_logo.pdf", config: defaultLogoConfig},
		{name: "aquaviario", fixture: "dacte_aquaviario_test.xml", expected: "dacte_default_aquaviario.pdf", config: defaultLogoConfig},
		{name: "aereo", fixture: "dacte_aereo_test.xml", expected: "dacte_default_aereo.pdf", config: defaultLogoConfig},
		{name: "ferroviario", fixture: "dacte_ferroviario_test.xml", expected: "dacte_default_ferroviario.pdf", config: defaultLogoConfig},
		{name: "dutoviario", fixture: "dacte_dutoviario_test.xml", expected: "dacte_default_dutoviario.pdf", config: defaultLogoConfig},
		{name: "multimodal", fixture: "dacte_multimodal_test.xml", expected: "dacte_default_multimodal.pdf", config: defaultLogoConfig},
		{name: "tomador outros", fixture: "dacte_tomador_outros.xml", expected: "dacte_tomador_outros.pdf", config: defaultLogoConfig},
		{name: "cancelled production", fixture: "dacte_test_1.xml", expected: "dacte_watermark_cancelled_production.pdf", config: Config{WatermarkCancelled: true}},
		{name: "cancelled homologation", fixture: "dacte_test_homolog.xml", expected: "dacte_watermark_cancelled_homologation.pdf", config: Config{WatermarkCancelled: true}},
		{name: "homologation watermark", fixture: "dacte_test_homolog.xml", expected: "dacte_watermark_homologation_only.pdf"},
		{name: "reforma tributaria", fixture: "dacte_reforma_tributaria.xml", expected: "dacte_reforma_tributaria.pdf", config: Config{DisplayIBSCBS: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", tt.fixture))
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "dacte.pdf")
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
			expected := filepath.Join("..", "tests", "generated", "dacte", tt.expected)
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
				if max := golden.MaxMeanAbsoluteError(diffs); max > 0.11 {
					t.Fatalf("raster diff too high: max=%f pages=%#v", max, diffs)
				}
			}
		})
	}
}

func renderFixturePDF(t *testing.T, fixture string, config *Config) string {
	t.Helper()
	xmlContent, err := os.ReadFile(filepath.Join("..", "tests", "fixtures", "dacte", fixture))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dacte.pdf")
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

func mustExtractPDFWords(t *testing.T, path string) []golden.TextWord {
	t.Helper()
	words, err := golden.ExtractTextWords(path)
	if err != nil {
		t.Fatal(err)
	}
	return words
}

func mustFindPDFWordInBox(t *testing.T, words []golden.TextWord, text string, xMin, xMax, yMin, yMax float64, occurrence int) golden.TextWord {
	t.Helper()
	seen := 0
	for _, word := range words {
		if word.Text != text || word.XMin < xMin || word.XMin > xMax || word.YMin < yMin || word.YMin > yMax {
			continue
		}
		if seen == occurrence {
			return word
		}
		seen++
	}
	t.Fatalf("word %q not found in box x=[%f,%f] y=[%f,%f]", text, xMin, xMax, yMin, yMax)
	return golden.TextWord{}
}

func mustParsePDFPoint(t *testing.T, value []byte) float64 {
	t.Helper()
	point, err := strconv.ParseFloat(string(value), 64)
	if err != nil {
		t.Fatal(err)
	}
	return point
}

func extractPageText(t *testing.T, path string, firstPage, lastPage int) string {
	t.Helper()
	output, err := exec.Command("pdftotext", "-enc", "UTF-8", "-f", fmt.Sprint(firstPage), "-l", fmt.Sprint(lastPage), path, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	return golden.NormalizeExtractedText(string(output))
}
