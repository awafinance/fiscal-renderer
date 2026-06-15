package danfe

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/awafinance/fiscal-renderer/internal/barcode"
	"github.com/awafinance/fiscal-renderer/internal/fiscalfmt"
	"github.com/awafinance/fiscal-renderer/internal/images"
	"github.com/awafinance/fiscal-renderer/internal/pdfdraw"
	"github.com/awafinance/fiscal-renderer/internal/xmlutil"
	"github.com/go-pdf/fpdf"
)

type Document struct {
	XML    string
	Config Config
	root   *xmlutil.Node
}

func New(xml string, config *Config) (*Document, error) {
	root, err := xmlutil.ParseString(xml)
	if err != nil {
		return nil, err
	}
	return &Document{XML: xml, Config: normalizeConfig(config), root: root}, nil
}

func (d *Document) Output(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return d.Write(file)
}

func (d *Document) Write(w io.Writer) error {
	root := d.root
	if root == nil {
		parsed, err := xmlutil.ParseString(d.XML)
		if err != nil {
			return err
		}
		root = parsed
	}
	data := parseData(root, d.Config)
	pdf := pdfdraw.NewPDF(data.Orientation, "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(d.Config.Margins.Left, d.Config.Margins.Top, d.Config.Margins.Right)
	pdf.SetAutoPageBreak(false, bottomMargin(d.Config))
	pdf.SetTitle("DANFE", false)
	pdf.AddPage()
	data.ProductSplitIndex = draw(pdf, data, d.Config, false)
	if data.NeedsContinuationPage {
		pdf.AddPage()
		draw(pdf, data, d.Config, true)
	}
	return pdf.Output(w)
}

func RenderFile(xml string, path string, config *Config) error {
	doc, err := New(xml, config)
	if err != nil {
		return err
	}
	return doc.Output(path)
}

type nfeData struct {
	Key                   string
	BarcodePNG            []byte
	Model                 string
	Series                string
	Number                string
	NFType                string
	NFTypeCode            string
	Nature                string
	EnvironmentCode       string
	Protocol              string
	ProtocolDate          string
	ProtocolTime          string
	EmissionDate          string
	EmissionTime          string
	ExitDate              string
	ExitTime              string
	Orientation           string
	QRCode                string
	Issuer                party
	Recipient             party
	Pickup                *party
	Delivery              *party
	Products              []product
	ProductTaxCodeHeader  string
	ProductSplitIndex     int
	Billing               *billing
	Invoices              []invoice
	Totals                []field
	Taxes                 []field
	Shipping              []field
	AdditionalInfo        string
	FiscoInfo             string
	HasProtocol           bool
	WatermarkCancelled    bool
	NeedsContinuationPage bool
}

type party struct {
	Name         string
	Doc          string
	IE           string
	Address      string
	Complement   string
	District     string
	City         string
	UF           string
	CEP          string
	Phone        string
	Municipality string
	Street       string
	Number       string
}

type product struct {
	Code       string
	Desc       string
	NCM        string
	CST        string
	CFOP       string
	Unit       string
	Quantity   string
	UnitPrice  string
	TotalPrice string
	BCICMS     string
	ICMSValue  string
	IPIValue   string
	ICMSRate   string
	IPIRate    string
	PISValue   string
	COFINS     string
}

type invoice struct {
	Number string
	Due    string
	Value  string
}

type billing struct {
	Number        string
	OriginalValue string
	DiscountValue string
	NetValue      string
}

type field struct {
	Label string
	Value string
}

func parseData(root *xmlutil.Node, config Config) nfeData {
	infNFe := root.Find("infNFe")
	ide := root.Find("ide")
	emit := root.Find("emit")
	dest := root.Find("dest")
	icmsTot := root.Find("ICMSTot")
	transp := root.Find("transp")
	cobr := root.Find("cobr")
	infAdic := root.Find("infAdic")
	protocol := root.Find("protNFe")
	if protocol == nil {
		protocol = root.Find("infProt")
	}
	key := strings.TrimPrefix(infNFe.Attr("Id"), "NFe")
	emissionDate, emissionTime := fiscalfmt.DateUTC(xmlutil.Text(ide, "dhEmi"))
	exitDate, exitTime := fiscalfmt.DateUTC(xmlutil.Text(ide, "dhSaiEnt"))
	protocolDate, protocolTime := fiscalfmt.DateUTC(xmlutil.Text(protocol, "dhRecbto"))
	orientation := "P"
	if xmlutil.Text(ide, "tpImp") != "1" {
		orientation = "L"
	}
	additional := additionalInfo(infAdic, config)
	fisco := normalizeSpaces(xmlutil.Text(infAdic, "infAdFisco"))
	crt := xmlutil.Text(emit, "CRT")
	products := parseProducts(root.FindAll("det"), crt, config)
	barcodePNG, _ := barcode.Code128PNG(key, 450, 60)

	data := nfeData{
		Key:                   key,
		BarcodePNG:            barcodePNG,
		Model:                 xmlutil.Text(ide, "mod"),
		Series:                xmlutil.Text(ide, "serie"),
		Number:                xmlutil.Text(ide, "nNF"),
		NFType:                nfType(xmlutil.Text(ide, "tpNF")),
		NFTypeCode:            xmlutil.Text(ide, "tpNF"),
		Nature:                xmlutil.Text(ide, "natOp"),
		EnvironmentCode:       xmlutil.Text(ide, "tpAmb"),
		Protocol:              xmlutil.Text(protocol, "nProt"),
		ProtocolDate:          protocolDate,
		ProtocolTime:          protocolTime,
		EmissionDate:          emissionDate,
		EmissionTime:          emissionTime,
		ExitDate:              exitDate,
		ExitTime:              exitTime,
		Orientation:           orientation,
		QRCode:                firstNonEmpty(xmlutil.RawText(root.Find("infNFeSupl"), "qrCode"), xmlutil.RawText(root.Find("infNFeSupl"), "qrCodNFe")),
		Issuer:                parseParty(emit),
		Recipient:             parseParty(dest),
		Pickup:                parseOptionalParty(root.Find("retirada")),
		Delivery:              parseOptionalParty(root.Find("entrega")),
		Products:              products,
		ProductTaxCodeHeader:  productTaxCodeHeader(crt),
		Billing:               parseBilling(cobr),
		Invoices:              parseInvoices(cobr),
		Totals:                parseTotals(icmsTot),
		Taxes:                 parseTaxSummary(root, config),
		Shipping:              parseShipping(transp),
		AdditionalInfo:        additional,
		FiscoInfo:             fisco,
		HasProtocol:           protocol != nil && xmlutil.Text(protocol, "nProt") != "",
		WatermarkCancelled:    config.WatermarkCancelled,
		NeedsContinuationPage: productsNeedContinuation(products, orientation) || additionalNeedsContinuation(products, additional, fisco),
	}
	return data
}

func parseParty(node *xmlutil.Node) party {
	if node == nil {
		return party{Name: "-", Doc: "-", IE: "-", Address: "-", District: "-", City: "-", UF: "-", CEP: "-", Phone: "-"}
	}
	addressNode := firstNode(node.Find("enderEmit"), node.Find("enderDest"), node.Find("end"))
	if addressNode == nil {
		addressNode = node
	}
	return party{
		Name:         optional(firstNonEmpty(xmlutil.Text(node, "xNome"), xmlutil.Text(node, "xFant"))),
		Doc:          optional(firstNonEmpty(fiscalfmt.FormatCPFCNPJ(xmlutil.Text(node, "CNPJ")), fiscalfmt.FormatCPFCNPJ(xmlutil.Text(node, "CPF")))),
		IE:           optional(xmlutil.Text(node, "IE")),
		Address:      formatAddress(addressNode),
		Complement:   optional(xmlutil.Text(addressNode, "xCpl")),
		District:     optional(xmlutil.Text(addressNode, "xBairro")),
		City:         optional(xmlutil.Text(addressNode, "xMun")),
		UF:           optional(xmlutil.Text(addressNode, "UF")),
		CEP:          optional(fiscalfmt.FormatCEP(xmlutil.Text(addressNode, "CEP"))),
		Phone:        optional(fiscalfmt.FormatPhone(xmlutil.Text(addressNode, "fone"))),
		Municipality: optional(xmlutil.Text(addressNode, "cMun")),
		Street:       optional(xmlutil.Text(addressNode, "xLgr")),
		Number:       optional(xmlutil.Text(addressNode, "nro")),
	}
}

func parseOptionalParty(node *xmlutil.Node) *party {
	if node == nil {
		return nil
	}
	p := parseParty(node)
	if p.Name == "-" && p.Doc == "-" && p.Address == "-" {
		return nil
	}
	return &p
}

func formatAddress(node *xmlutil.Node) string {
	parts := nonEmpty(xmlutil.Text(node, "xLgr"), xmlutil.Text(node, "nro"), xmlutil.Text(node, "xCpl"), xmlutil.Text(node, "xBairro"))
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func issuerHeaderAddress(p party) string {
	streetNumber := strings.Join(nonEmpty(p.Street, p.Number), ", ")
	lines := nonEmpty(
		streetNumber,
		p.Complement,
		p.District,
		strings.TrimSpace(strings.Join(nonEmpty(p.City, p.UF), " - ")),
		p.CEP,
		"Fone: "+p.Phone,
	)
	if len(lines) == 0 {
		return p.Address
	}
	return strings.Join(lines, "\n")
}

func parseProducts(detNodes []*xmlutil.Node, crt string, config Config) []product {
	var products []product
	for _, det := range detNodes {
		prod := det.Find("prod")
		icms := det.Find("ICMS")
		ipi := det.Find("IPI")
		pis := det.Find("PIS")
		cofins := det.Find("COFINS")
		infAdProd := buildAdditionalProductInfo(prod, xmlutil.Text(det, "infAdProd"), config)
		desc := xmlutil.Text(prod, "xProd")
		if infAdProd != "" {
			desc = strings.TrimSpace(desc + "\n" + infAdProd)
		}
		cst := xmlutil.Text(icms, "orig")
		if crt == "1" || crt == "4" {
			cst += xmlutil.Text(icms, "CSOSN")
		} else {
			cst += xmlutil.Text(icms, "CST")
		}
		products = append(products, product{
			Code:       xmlutil.Text(prod, "cProd"),
			Desc:       desc,
			NCM:        xmlutil.Text(prod, "NCM"),
			CST:        cst,
			CFOP:       xmlutil.Text(prod, "CFOP"),
			Unit:       fiscalfmt.MergeIfDifferent(xmlutil.Text(prod, "uCom"), xmlutil.Text(prod, "uTrib")),
			Quantity:   fiscalfmt.MergeIfDifferent(fiscalfmt.FormatNumber(xmlutil.Text(prod, "qCom"), config.DecimalConfig.QuantityPrecision), fiscalfmt.FormatNumber(xmlutil.Text(prod, "qTrib"), config.DecimalConfig.QuantityPrecision)),
			UnitPrice:  fiscalfmt.MergeIfDifferent(fiscalfmt.FormatNumber(xmlutil.Text(prod, "vUnCom"), config.DecimalConfig.PricePrecision), fiscalfmt.FormatNumber(xmlutil.Text(prod, "vUnTrib"), config.DecimalConfig.PricePrecision)),
			TotalPrice: fiscalfmt.FormatNumber(xmlutil.Text(prod, "vProd"), 2),
			BCICMS:     fiscalfmt.FormatNumber(xmlutil.Text(icms, "vBC"), 2),
			ICMSValue:  fiscalfmt.FormatNumber(xmlutil.Text(icms, "vICMS"), 2),
			IPIValue:   fiscalfmt.FormatNumber(xmlutil.Text(ipi, "vIPI"), 2),
			ICMSRate:   rate(xmlutil.Text(icms, "pICMS")),
			IPIRate:    rate(xmlutil.Text(ipi, "pIPI")),
			PISValue:   fiscalfmt.FormatNumber(xmlutil.Text(pis, "vPIS"), 2),
			COFINS:     fiscalfmt.FormatNumber(xmlutil.Text(cofins, "vCOFINS"), 2),
		})
	}
	return products
}

func productTaxCodeHeader(crt string) string {
	if crt == "1" || crt == "4" {
		return "CSOSN"
	}
	return "CST"
}

func buildAdditionalProductInfo(prod *xmlutil.Node, infAdProd string, config Config) string {
	var infos []string
	prefix := config.ProductDescriptionConfig.BranchInfoPrefix
	if prefix != "" {
		prefix += " "
	}
	if config.ProductDescriptionConfig.DisplayBranch {
		for _, rastro := range prod.FindAll("rastro") {
			fab, _ := fiscalfmt.DateUTC(xmlutil.Text(rastro, "dFab"))
			val, _ := fiscalfmt.DateUTC(xmlutil.Text(rastro, "dVal"))
			infos = append(infos, fmt.Sprintf("%sLote: %s Qtd: %s Fab: %s Val: %s", prefix, xmlutil.Text(rastro, "nLote"), fiscalfmt.FormatNumber(xmlutil.Text(rastro, "qLote"), config.DecimalConfig.QuantityPrecision), fab, val))
		}
	}
	if config.ProductDescriptionConfig.DisplayANP {
		for _, comb := range prod.FindAll("comb") {
			infos = append(infos, fmt.Sprintf("cProdANP: %s descANP: %s UFCons: %s", xmlutil.Text(comb, "cProdANP"), xmlutil.Text(comb, "descANP"), xmlutil.Text(comb, "UFCons")))
		}
	}
	if config.ProductDescriptionConfig.DisplayANVISA {
		for _, med := range prod.FindAll("med") {
			infos = append(infos, fmt.Sprintf("cProdANVISA: %s PMC: %s", xmlutil.Text(med, "cProdANVISA"), fiscalfmt.FormatNumber(xmlutil.Text(med, "vPMC"), config.DecimalConfig.QuantityPrecision)))
		}
	}
	if cBenef := xmlutil.Text(prod, "cBenef"); cBenef != "" {
		infos = append(infos, "cBenef: "+cBenef)
	}
	if cCred := xmlutil.Text(prod, "cCredPresumido"); cCred != "" {
		infos = append(infos, "cCredPresumido: "+cCred)
	}
	if config.ProductDescriptionConfig.DisplayAdditionalInfo && infAdProd != "" {
		infos = append(infos, infAdProd)
	}
	return strings.Join(infos, "\n")
}

func parseInvoices(cobr *xmlutil.Node) []invoice {
	var invoices []invoice
	for _, dup := range cobr.FindAll("dup") {
		date, _ := fiscalfmt.DateUTC(xmlutil.Text(dup, "dVenc"))
		invoices = append(invoices, invoice{
			Number: xmlutil.Text(dup, "nDup"),
			Due:    date,
			Value:  fiscalfmt.FormatNumber(xmlutil.Text(dup, "vDup"), 2),
		})
	}
	return invoices
}

func parseBilling(cobr *xmlutil.Node) *billing {
	if cobr == nil {
		return nil
	}
	fat := cobr.Find("fat")
	return &billing{
		Number:        xmlutil.Text(fat, "nFat"),
		OriginalValue: fiscalfmt.FormatNumber(xmlutil.Text(fat, "vOrig"), 2),
		DiscountValue: fiscalfmt.FormatNumber(xmlutil.Text(fat, "vDesc"), 2),
		NetValue:      fiscalfmt.FormatNumber(xmlutil.Text(fat, "vLiq"), 2),
	}
}

func parseTotals(icmsTot *xmlutil.Node) []field {
	return []field{
		{"BASE DE CÁLCULO DO ICMS", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vBC"), 2))},
		{"VALOR DO ICMS", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vICMS"), 2))},
		{"BASE DE CÁLCULO DO ICMS ST", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vBCST"), 2))},
		{"VALOR DO ICMS ST", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vST"), 2))},
		{"VALOR APROX. TRIBUTOS", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vTotTrib"), 2))},
		{"VALOR TOTAL DOS PRODUTOS", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vProd"), 2))},
		{"VALOR DO FRETE", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vFrete"), 2))},
		{"VALOR DO SEGURO", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vSeg"), 2))},
		{"DESCONTO", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vDesc"), 2))},
		{"OUTRAS DESPESAS ACESSÓRIAS", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vOutro"), 2))},
		{"VALOR DO IPI", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vIPI"), 2))},
		{"VALOR TOTAL DA NOTA", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vNF"), 2))},
	}
}

func parseTaxSummary(root *xmlutil.Node, config Config) []field {
	ibscbs := root.Find("IBSCBSTot")
	var fields []field
	if config.DisplayPISCOFINS {
		icmsTot := root.Find("ICMSTot")
		fields = append(fields,
			field{"VALOR DO PIS", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vPIS"), 2))},
			field{"VALOR DO COFINS", money(fiscalfmt.FormatNumber(xmlutil.Text(icmsTot, "vCOFINS"), 2))},
		)
	}
	if ibscbs != nil {
		fields = append(fields,
			field{"IBS UF", money(fiscalfmt.FormatNumber(xmlutil.Text(ibscbs, "vIBSUF"), 2))},
			field{"IBS MUN.", money(fiscalfmt.FormatNumber(xmlutil.Text(ibscbs, "vIBSMun"), 2))},
			field{"CBS", money(fiscalfmt.FormatNumber(xmlutil.Text(ibscbs, "vCBS"), 2))},
		)
	}
	return fields
}

func parseShipping(transp *xmlutil.Node) []field {
	transporta := transp.Find("transporta")
	vehicle := transp.Find("veicTransp")
	vol := transp.Find("vol")
	return []field{
		{"NOME / RAZÃO SOCIAL", xmlutil.Text(transporta, "xNome")},
		{"FRETE POR CONTA", freightMode(xmlutil.Text(transp, "modFrete"))},
		{"CÓDIGO ANTT", xmlutil.Text(vehicle, "RNTC")},
		{"PLACA DO VEÍCULO", xmlutil.Text(vehicle, "placa")},
		{"UF", xmlutil.Text(vehicle, "UF")},
		{"CNPJ / CPF", firstNonEmpty(fiscalfmt.FormatCPFCNPJ(xmlutil.Text(transporta, "CNPJ")), fiscalfmt.FormatCPFCNPJ(xmlutil.Text(transporta, "CPF")))},
		{"ENDEREÇO", xmlutil.Text(transporta, "xEnder")},
		{"MUNICÍPIO", xmlutil.Text(transporta, "xMun")},
		{"UF", xmlutil.Text(transporta, "UF")},
		{"INSCRIÇÃO ESTADUAL", xmlutil.Text(transporta, "IE")},
		{"QUANTIDADE", fiscalfmt.FormatNumber(xmlutil.Text(vol, "qVol"), 0)},
		{"ESPÉCIE", xmlutil.Text(vol, "esp")},
		{"MARCA", xmlutil.Text(vol, "marca")},
		{"NUMERAÇÃO", xmlutil.Text(vol, "nVol")},
		{"PESO BRUTO", fiscalfmt.FormatNumber(xmlutil.Text(vol, "pesoB"), 3)},
		{"PESO LÍQUIDO", fiscalfmt.FormatNumber(xmlutil.Text(vol, "pesoL"), 3)},
	}
}

func additionalInfo(infAdic *xmlutil.Node, config Config) string {
	parts := nonEmpty(xmlutil.Text(infAdic, "infCpl"), xmlutil.Text(infAdic, "infAdFisco"))
	text := strings.Join(parts, " ")
	if config.InfCplSemicolonNewline {
		text = strings.ReplaceAll(text, ";", "\n")
		return strings.TrimSpace(text)
	}
	return normalizeSpaces(text)
}

func productsNeedContinuation(products []product, orientation string) bool {
	if len(products) >= 24 {
		return true
	}
	if orientation == "L" && len(products) >= 18 {
		return true
	}
	return false
}

func additionalNeedsContinuation(products []product, additional, fisco string) bool {
	infoLen := len(additional) + len(fisco)
	if infoLen > 1200 && len(products) >= 6 {
		return true
	}
	return false
}

func headerAdvance(data nfeData) float64 {
	if data.Orientation == "L" {
		return 44
	}
	return 40
}

func continuationProductYOffset(data nfeData) float64 {
	if data.Orientation == "L" {
		return 5
	}
	return 1
}

func recipientAdvance(data nfeData) float64 {
	if data.Orientation == "L" {
		return 22
	}
	return 22
}

func taxBlockMetrics(data nfeData) (height, advance float64) {
	if data.Orientation == "L" {
		return 12, 13
	}
	return 13, 14
}

func shippingBlockMetrics(data nfeData) (height, advance float64) {
	if data.Orientation == "L" {
		return 18, 19
	}
	return 18, 19
}

func productReserve(data nfeData) float64 {
	if data.Orientation == "L" {
		return 23
	}
	return 27
}

func normalizedProductHeight(height float64) float64 {
	if height < 44 {
		return 44
	}
	return height
}

func productRowHeight(config Config) float64 {
	if config.FontSize == FontSizeBig {
		return 4.2
	}
	return 3.0
}

func productLimitForHeight(height, rowHeight float64) int {
	if rowHeight <= 0 || height <= 12 {
		return 0
	}
	return int((height - 12) / rowHeight)
}

func draw(pdf *pdfdraw.PDF, data nfeData, config Config, continuation bool) int {
	drawWatermark(pdf, data, config)
	pageW, pageH := pdf.GetPageSize()
	x := config.Margins.Left
	y := config.Margins.Top
	w := pageW - config.Margins.Left - config.Margins.Right
	h := pageH - config.Margins.Top - bottomMargin(config)
	if data.Orientation == "L" && !continuation {
		drawLandscapeReceipt(pdf, x, y, 17, h, data, config)
		x += 19
		w -= 19
	}
	pdf.Rect(x, y, w, h, "")
	if continuation {
		drawHeader(pdf, x, y, w, data, config, true)
		y += headerAdvance(data)
		if productsNeedContinuation(data.Products, data.Orientation) {
			start := data.ProductSplitIndex
			productYOffset := continuationProductYOffset(data)
			drawProducts(pdf, x, y-productYOffset, w, h-103+productYOffset, data.Products, data.ProductTaxCodeHeader, config, start, len(data.Products))
			drawAdditionalContinuation(pdf, x, y+h-42, w, 38, data, config)
		} else {
			drawAdditionalContinuation(pdf, x, y, w, h-64, data, config)
		}
		drawFooterStamp(pdf, config)
		return 0
	}
	if data.Orientation != "L" && config.ReceiptPosition == ReceiptPositionTop {
		drawReceipt(pdf, x, y, w, 17, data, config)
		y += 20
	}
	headerY := y + 2
	if data.Orientation != "L" && config.ReceiptPosition == ReceiptPositionTop {
		headerY = y - 1
	}
	drawHeader(pdf, x, headerY, w, data, config, false)
	y += headerAdvance(data)
	drawRecipient(pdf, x, y, w, data, config)
	y += recipientAdvance(data)
	if data.Pickup != nil {
		drawLocation(pdf, x, y, w, "INFORMAÇÕES DO LOCAL DE RETIRADA", *data.Pickup, config)
		y += locationBlockHeight
	}
	if data.Delivery != nil {
		drawLocation(pdf, x, y, w, "INFORMAÇÕES DO LOCAL DE ENTREGA", *data.Delivery, config)
		y += locationBlockHeight
	}
	if data.Billing != nil {
		drawInvoices(pdf, x, y, w, 12, data.Billing, data.Invoices, config)
		y += 12
	}
	taxHeight, taxAdvance := taxBlockMetrics(data)
	drawTaxSummary(pdf, x, y, w, taxHeight, data, config)
	y += taxAdvance
	shippingHeight, shippingAdvance := shippingBlockMetrics(data)
	drawShipping(pdf, x, y, w, shippingHeight, data.Shipping, config)
	y += shippingAdvance
	remaining := h - (y - config.Margins.Top)
	if data.Orientation != "L" && config.ReceiptPosition == ReceiptPositionBottom {
		remaining -= 22
	}
	productH := normalizedProductHeight(remaining - productReserve(data))
	productLimit := len(data.Products)
	if productsNeedContinuation(data.Products, data.Orientation) && data.ProductSplitIndex > 0 {
		productLimit = data.ProductSplitIndex
	}
	drawnLimit := drawProducts(pdf, x, y, w, productH, data.Products, data.ProductTaxCodeHeader, config, 0, productLimit)
	y += productH + 3
	drawAdditional(pdf, x, y, w, minFloat(46, h-(y-config.Margins.Top)-4), "DADOS ADICIONAIS", data, config)
	if data.Orientation != "L" && config.ReceiptPosition == ReceiptPositionBottom {
		drawReceipt(pdf, x, pageH-bottomMargin(config)-20, w, 17, data, config)
	}
	drawFooterStamp(pdf, config)
	return drawnLimit
}

func drawWatermark(pdf *pdfdraw.PDF, data nfeData, config Config) {
	text := ""
	size := 60.0
	if data.WatermarkCancelled {
		if data.EnvironmentCode == "1" {
			text = "CANCELADA"
		} else {
			text = "CANCELADA - SEM VALOR FISCAL"
			size = 45
		}
	} else if data.EnvironmentCode != "1" || !data.HasProtocol {
		text = "SEM VALOR FISCAL"
	}
	if text == "" {
		return
	}
	pdf.SetTextColor(220, 150, 150)
	pdf.SetFont(string(config.FontType), "B", size)
	width := pdf.GetStringWidth(pdf.Encode(text))
	height := size * 0.25
	pageW, pageH := pdf.GetPageSize()
	xCenter := (pageW - width) / 2
	yCenter := (pageH + height) / 2
	pdf.TransformBegin()
	pdf.TransformRotate(55, xCenter+(width/2), yCenter-(height/2))
	pdf.Text(xCenter, yCenter, text)
	pdf.TransformEnd()
	pdf.SetTextColor(0, 0, 0)
}

func drawReceipt(pdf *pdfdraw.PDF, x, y, w, h float64, data nfeData, config Config) {
	pdf.Rect(x, y, w, h, "")
	numberW := 30.0
	pdf.Line(x+w-numberW, y, x+w-numberW, y+h)
	pdf.Line(x, y+h/2, x+w-numberW, y+h/2)
	pdf.Line(x+41, y+h/2, x+41, y+h)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5))
	pdf.SetXY(x, y+1)
	pdf.MultiCell(w-numberW, 1.75, receiptText(data), "", "L", false)
	pdf.SetXY(x, y+h/2-0.1)
	pdf.CellFormat(40, 3, "DATA DE RECEBIMENTO", "", 0, "L", false, 0, "")
	pdf.CellFormat(w-numberW-42, 3, "IDENTIFICAÇÃO E ASSINATURA DO RECEBEDOR", "", 0, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 10))
	pdf.SetXY(x+w-numberW, y+0.4)
	pdf.MultiCell(numberW, 4.5, "NF-e\nNº"+formatInvoiceNumber(data.Number)+"\nSÉRIE "+data.Series, "", "C", false)
}

func drawLandscapeReceipt(pdf *pdfdraw.PDF, x, y, w, h float64, data nfeData, config Config) {
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 7))
	pdf.SetXY(x+1, y+4)
	pdf.MultiCell(w-2, 4, "NF-e\nNº "+formatInvoiceNumber(data.Number)+"\nSÉRIE "+data.Series, "", "C", false)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 4.8))
	pdf.SetXY(x+1, y+35)
	pdf.MultiCell(w-2, 2.7, receiptText(data), "", "L", false)
}

func drawHeader(pdf *pdfdraw.PDF, x, y, w float64, data nfeData, config Config, continuation bool) {
	h := 31.0
	entryY := 12.4
	invoiceY := 19.65
	keyY := 11.5
	queryY := 18.5
	if data.Orientation == "L" {
		h = 29
		entryY = 13
		invoiceY = 18
		keyY = 12
		queryY = 21
	}
	idW := 33.0
	codeW := 88.0
	emitW := w - idW - codeW
	if emitW < 65 {
		emitW = w * 0.38
		codeW = w - emitW - idW
	}
	pdf.Rect(x, y, emitW, h, "")
	hasLogo := len(config.LogoBytes) > 0 || config.Logo != ""
	if len(config.LogoBytes) > 0 {
		pdf.ImageBytes("danfe-logo", config.LogoBytes, x+2, y+10, 30, 0)
	} else if config.Logo != "" {
		if imageType, _ := images.TypeFromFile(config.Logo); imageType != "" {
			pdf.ImageOptions(config.Logo, x+2, y+10, 30, 0, false, fpdf.ImageOptions{ImageType: imageType}, 0, "")
		}
	}
	issuerNameY := y + 1.1
	issuerAddressY := y + 13
	if hasLogo {
		issuerNameY = y + 4
		issuerAddressY = y + 15
	}
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 12))
	pdf.SetXY(x, issuerNameY)
	pdf.MultiCell(emitW, 4, data.Issuer.Name, "", "C", false)
	addressX := x + 2
	addressW := emitW
	if hasLogo {
		addressX = x + 32
		addressW = emitW - 33
	}
	pdf.SetFont(string(config.FontType), "", fontSize(config, 8))
	pdf.SetXY(addressX, issuerAddressY)
	pdf.MultiCell(addressW, 3, issuerHeaderAddress(data.Issuer), "", "C", false)
	pdf.Rect(x+emitW, y, idW, h, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 12))
	pdf.SetXY(x+emitW, y)
	pdf.CellFormat(idW, 4, "DANFE", "", 2, "C", false, 0, "")
	pdf.SetFont(string(config.FontType), "", fontSize(config, 7))
	pdf.SetXY(x+emitW, y+3.4)
	pdf.CellFormat(idW, 3, "DOCUMENTO AUXILIAR", "", 2, "C", false, 0, "")
	pdf.CellFormat(idW, 3, "DA NOTA FISCAL", "", 2, "C", false, 0, "")
	pdf.CellFormat(idW, 3, "ELETRÔNICA", "", 2, "C", false, 0, "")
	pdf.SetFont(string(config.FontType), "", fontSize(config, 6))
	pdf.SetXY(x+emitW+1, y+entryY)
	pdf.MultiCell(idW-2, 3, "0-ENTRADA\n1-SAÍDA", "", "L", false)
	pdf.Rect(x+emitW+26, y+entryY, 5, 5, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 10))
	pdf.SetXY(x+emitW+26, y+entryY)
	pdf.CellFormat(5, 5, data.NFTypeCode, "", 0, "C", false, 0, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 10))
	pdf.SetXY(x+emitW+1, y+invoiceY)
	pdf.CellFormat(idW-2, 4.4, "Nº "+formatInvoiceNumber(data.Number), "", 2, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 8))
	pdf.CellFormat(idW-2, 3, "SÉRIE "+data.Series, "", 2, "L", false, 0, "")
	pdf.CellFormat(idW-2, 3, "FOLHA "+danfePageLabel(pdf, data), "", 0, "L", false, 0, "")
	codeX := x + emitW + idW
	pdf.Rect(codeX, y, codeW, 10, "")
	drawBarcode(pdf, codeX+0.5, y+0.5, codeW, 8.5, data.Key, data.BarcodePNG)
	pdf.Rect(codeX, y+10, codeW, 6, "")
	pdf.Rect(codeX, y+16, codeW, h-16, "")
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5))
	pdf.SetXY(codeX, y+keyY-1.5)
	pdf.CellFormat(codeW-2, 2.8, "CHAVE DE ACESSO", "", 2, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 7))
	pdf.SetXY(codeX, y+keyY+1.3)
	pdf.MultiCell(codeW, 3, strings.Join(fiscalfmt.Chunks(data.Key, 4), " "), "", "C", false)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 10))
	pdf.SetXY(codeX+2, y+queryY-0.24)
	pdf.MultiCell(codeW-4, 3.53, "Consulta de autenticidade no portal nacional da NF-e\nwww.nfe.fazenda.gov.br/portal ou no site da Sefaz\nautorizadora", "", "C", false)
	rowsY := y + h
	drawDANFEHeaderField(pdf, x, rowsY, emitW+idW, 6, "NATUREZA DA OPERAÇÃO", data.Nature, "", config)
	drawDANFEHeaderField(pdf, x+emitW+idW, rowsY, codeW, 6, "PROTOCOLO DE AUTORIZAÇÃO DE USO", strings.TrimSpace(data.Protocol+" - "+data.ProtocolDate+" "+data.ProtocolTime), "protocolo", config)
	thirdW := w / 3
	drawDANFEHeaderField(pdf, x, rowsY+6, thirdW, 6, "INSCRIÇÃO ESTADUAL", data.Issuer.IE, "", config)
	drawDANFEHeaderField(pdf, x+thirdW, rowsY+6, thirdW, 6, "INSCRIÇÃO ESTADUAL DO SUBST. TRIB", "", "", config)
	drawDANFEHeaderField(pdf, x+(thirdW*2), rowsY+6, w-(thirdW*2), 6, "CNPJ / CPF", data.Issuer.Doc, "", config)
}

func drawDANFEHeaderField(pdf *pdfdraw.PDF, x, y, w, h float64, label, value, fieldType string, config Config) {
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5))
	pdf.SetXY(x+1, y+0.5)
	pdf.CellFormat(w-2, 2.4, label, "", 2, "L", false, 0, "")
	style := ""
	align := "L"
	size := fontSize(config, 6.2)
	if fieldType == "protocolo" {
		style = "B"
		align = "C"
		size = fontSize(config, 6.6)
	}
	pdf.SetFont(string(config.FontType), style, size)
	pdf.MultiCell(w-2, 2.6, optional(value), "", align, false)
}

func drawRecipient(pdf *pdfdraw.PDF, x, y, w float64, data nfeData, config Config) {
	name := data.Recipient.Name
	if data.EnvironmentCode != "1" {
		name = "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
	}
	drawDANFEBlockTitleAt(pdf, x, y+1, w, "DESTINATÁRIO / REMETENTE", config)

	fieldY := y + 5
	nameW := w - 35 - 30
	drawDANFEBasicField(pdf, x, fieldY, nameW, 6, "NOME / RAZÃO SOCIAL", name, "", config)
	drawDANFEBasicField(pdf, x+nameW, fieldY, 35, 6, "CNPJ / CPF", data.Recipient.Doc, "", config)
	drawDANFEBasicField(pdf, x+nameW+35, fieldY, 30, 6, "DATA DA EMISSÃO", data.EmissionDate, "", config)

	row2Y := fieldY + 6
	bairroW := 50.0
	cepW := 25.0
	exitDateW := 30.0
	addressW := w - bairroW - cepW - exitDateW
	drawDANFEBasicField(pdf, x, row2Y, addressW, 6, "ENDEREÇO", fiscalfmt.LimitText(data.Recipient.Address, maxChars(addressW)), "", config)
	drawDANFEBasicField(pdf, x+addressW, row2Y, bairroW, 6, "BAIRRO / DISTRITO", data.Recipient.District, "", config)
	drawDANFEBasicField(pdf, x+addressW+bairroW, row2Y, cepW, 6, "CEP", data.Recipient.CEP, "", config)
	drawDANFEBasicField(pdf, x+addressW+bairroW+cepW, row2Y, exitDateW, 6, "DATA DA ENTRADA / SAÍDA", data.ExitDate, "", config)

	row3Y := fieldY + 12
	phoneW := 40.0
	ufW := 10.0
	ieW := 50.0
	exitTimeW := 30.0
	cityW := w - phoneW - ufW - ieW - exitTimeW
	drawDANFEBasicField(pdf, x, row3Y, cityW, 6, "MUNICÍPIO", data.Recipient.City, "", config)
	drawDANFEBasicField(pdf, x+cityW, row3Y, phoneW, 6, "FONE / FAX", data.Recipient.Phone, "", config)
	drawDANFEBasicField(pdf, x+cityW+phoneW, row3Y, ufW, 6, "UF", data.Recipient.UF, "", config)
	drawDANFEBasicField(pdf, x+cityW+phoneW+ufW, row3Y, ieW, 6, "INSCRIÇÃO ESTADUAL", data.Recipient.IE, "", config)
	drawDANFEBasicField(pdf, x+cityW+phoneW+ufW+ieW, row3Y, exitTimeW, 6, "HORA DE ENTRADA / SAÍDA", data.ExitTime, "", config)
}

const locationBlockHeight = 21.0

func drawLocation(pdf *pdfdraw.PDF, x, y, w float64, title string, p party, config Config) {
	pdf.Rect(x, y, w, locationBlockHeight, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x, y+1)
	pdf.CellFormat(w-2, 3, title, "", 1, "L", false, 0, "")

	line1 := []locationField{
		{Label: "NOME / RAZÃO SOCIAL", Value: p.Name, Width: w - 65},
		{Label: "CNPJ / CPF", Value: p.Doc, Width: 35},
		{Label: "IE", Value: p.IE, Width: 30},
	}
	line2 := []locationField{
		{Label: "ENDEREÇO", Value: p.Address, Width: w - 105},
		{Label: "BAIRRO / DISTRITO", Value: p.District, Width: 75},
		{Label: "CEP", Value: p.CEP, Width: 30},
	}
	line3 := []locationField{
		{Label: "MUNICÍPIO", Value: p.City, Width: w - 40},
		{Label: "UF", Value: p.UF, Width: 10},
		{Label: "FONE", Value: p.Phone, Width: 30},
	}
	rowY := y + 4
	drawLocationRow(pdf, x, rowY, line1, config)
	drawLocationRow(pdf, x, rowY+5.5, line2, config)
	drawLocationRow(pdf, x, rowY+11, line3, config)
}

type locationField struct {
	Label string
	Value string
	Width float64
}

func drawLocationRow(pdf *pdfdraw.PDF, x, y float64, fields []locationField, config Config) {
	cursorX := x
	for _, field := range fields {
		pdf.SetXY(cursorX+1, y)
		pdf.SetFont(string(config.FontType), "B", fontSize(config, 4.9))
		pdf.CellFormat(field.Width-2, 2.5, field.Label, "", 2, "L", false, 0, "")
		pdf.SetFont(string(config.FontType), "", fontSize(config, 5.8))
		pdf.CellFormat(field.Width-2, 2.8, fiscalfmt.LimitText(optional(field.Value), maxChars(field.Width-2)), "", 0, "L", false, 0, "")
		cursorX += field.Width
	}
}

func drawInvoices(pdf *pdfdraw.PDF, x, y, w, h float64, bill *billing, invoices []invoice, config Config) {
	drawDANFEBlockTitle(pdf, x, y, w, "FATURA / DUPLICATAS", config)
	nextY := y + 4
	if config.InvoiceDisplay == InvoiceDisplayFullDetails && bill != nil {
		colW := w / 4
		drawDANFEBasicField(pdf, x, nextY, colW, 6, "NÚMERO", bill.Number, "", config)
		drawDANFEBasicField(pdf, x+colW, nextY, colW, 6, "VALOR ORIGINAL", bill.OriginalValue, "number", config)
		drawDANFEBasicField(pdf, x+(colW*2), nextY, colW, 6, "VALOR DO DESCONTO", bill.DiscountValue, "number", config)
		drawDANFEBasicField(pdf, x+(colW*3), nextY, w-(colW*3), 6, "VALOR LÍQUIDO", bill.NetValue, "number", config)
		nextY += 6
	}
	if len(invoices) == 0 {
		return
	}
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5.8))
	dupTexts := make([]string, 0, len(invoices))
	maxWidth := 0.0
	for _, inv := range invoices {
		text := strings.Join(nonEmpty(inv.Number, inv.Due, inv.Value), "  ")
		dupTexts = append(dupTexts, text)
		maxWidth = maxFloat(maxWidth, pdf.GetStringWidth(pdf.Encode(text))+2)
	}
	if maxWidth <= 0 {
		return
	}
	cols := int(w / maxWidth)
	if cols < 1 {
		cols = 1
	}
	colW := w / float64(cols)
	for i, dupText := range dupTexts {
		row := i / cols
		col := i % cols
		cellX := x + float64(col)*colW
		cellY := nextY + float64(row)*3
		pdf.Rect(cellX, cellY, colW, 3, "")
		pdf.SetXY(cellX, cellY+danfeFieldTextYOffset(config))
		pdf.CellFormat(colW, 3, dupText, "", 0, "L", false, 0, "")
	}
}

func drawTaxSummary(pdf *pdfdraw.PDF, x, y, w, h float64, data nfeData, config Config) {
	drawDANFEBlockTitle(pdf, x, y, w, "CÁLCULO DO IMPOSTO", config)
	row1, row2 := taxSummaryRows(data)
	drawDANFEFieldRow(pdf, x, y+4, w, 6, row1, config)
	drawDANFEFieldRow(pdf, x, y+10, w, 6, row2, config)
}

func taxSummaryRows(data nfeData) ([]field, []field) {
	row1 := append([]field(nil), data.Totals[:minInt(6, len(data.Totals))]...)
	row2 := []field{}
	if len(data.Totals) > 6 {
		row2 = append(row2, data.Totals[6:minInt(12, len(data.Totals))]...)
	}
	if len(data.Taxes) > 0 {
		for _, tax := range data.Taxes {
			switch tax.Label {
			case "VALOR DO PIS":
				row1 = insertBeforeLast(row1, tax)
			case "VALOR DO COFINS":
				row2 = insertBeforeLast(row2, tax)
			default:
				row2 = insertBeforeLast(row2, tax)
			}
		}
	}
	return row1, row2
}

func drawShipping(pdf *pdfdraw.PDF, x, y, w, h float64, fields []field, config Config) {
	drawDANFEBlockTitle(pdf, x, y+1, w, "TRANSPORTADOR / VOLUMES TRANSPORTADOS", config)

	row1 := []field{
		{Label: "NOME / RAZÃO SOCIAL", Value: shippingValue(fields, 0)},
		{Label: "FRETE POR CONTA", Value: shippingValue(fields, 1)},
		{Label: "CÓDIGO ANTT", Value: shippingValue(fields, 2)},
		{Label: "PLACA DO VEÍCULO", Value: shippingValue(fields, 3)},
		{Label: "UF", Value: shippingValue(fields, 4)},
		{Label: "CNPJ / CPF", Value: shippingValue(fields, 5)},
	}
	drawDANFEFieldRowWithWidths(pdf, x, y+5, []float64{w - 28 - 18 - 23 - 8 - 30, 28, 18, 23, 8, 30}, 6, row1, config)

	municipalityW := 69.0
	ufW := 8.0
	ieW := 30.0
	row2 := []field{
		{Label: "ENDEREÇO", Value: shippingValue(fields, 6)},
		{Label: "MUNICÍPIO", Value: shippingValue(fields, 7)},
		{Label: "UF", Value: shippingValue(fields, 8)},
		{Label: "INSCRIÇÃO ESTADUAL", Value: shippingValue(fields, 9)},
	}
	drawDANFEFieldRowWithWidths(pdf, x, y+11, []float64{w - municipalityW - ufW - ieW, municipalityW, ufW, ieW}, 6, row2, config)

	remainingW := w - 25 - 30 - 30 - 45
	weightW := remainingW / 2
	row3 := []field{
		{Label: "QUANTIDADE", Value: shippingValue(fields, 10)},
		{Label: "ESPÉCIE", Value: shippingValue(fields, 11)},
		{Label: "MARCA", Value: shippingValue(fields, 12)},
		{Label: "NUMERAÇÃO", Value: shippingValue(fields, 13)},
		{Label: "PESO BRUTO", Value: shippingValue(fields, 14)},
		{Label: "PESO LÍQUIDO", Value: shippingValue(fields, 15)},
	}
	drawDANFEFieldRowWithWidths(pdf, x, y+17, []float64{25, 30, 30, 45, weightW, w - 25 - 30 - 30 - 45 - weightW}, 6, row3, config)
}

func shippingValue(fields []field, index int) string {
	if index < 0 || index >= len(fields) {
		return ""
	}
	return fields[index].Value
}

func drawDANFEFieldRowWithWidths(pdf *pdfdraw.PDF, x, y float64, widths []float64, h float64, fields []field, config Config) {
	cursorX := x
	for i, field := range fields {
		if i >= len(widths) {
			return
		}
		value := fiscalfmt.LimitText(optional(field.Value), maxChars(widths[i]-2))
		drawDANFEBasicField(pdf, cursorX, y, widths[i], h, field.Label, value, "", config)
		cursorX += widths[i]
	}
}

func drawDANFEFieldRow(pdf *pdfdraw.PDF, x, y, w, h float64, fields []field, config Config) {
	widths := danfeFieldRowWidths(w, len(fields))
	cursorX := x
	for i, field := range fields {
		value := strings.TrimSpace(strings.TrimPrefix(field.Value, "R$"))
		drawDANFEBasicField(pdf, cursorX, y, widths[i], h, field.Label, value, "number", config)
		cursorX += widths[i]
	}
}

func danfeFieldRowWidths(w float64, count int) []float64 {
	if count <= 0 {
		return nil
	}
	widths := make([]float64, count)
	fixedCount := count - 1
	if fixedCount < 0 {
		fixedCount = 0
	}
	fixed := float64(fixedCount) * 30
	remainder := w - fixed
	if remainder <= 0 {
		return scaleWidths(makeFilledWidths(count, 30), w)
	}
	for i := 0; i < fixedCount; i++ {
		widths[i] = 30
	}
	widths[count-1] = remainder
	return widths
}

func makeFilledWidths(count int, width float64) []float64 {
	widths := make([]float64, count)
	for i := range widths {
		widths[i] = width
	}
	return widths
}

func insertBeforeLast(fields []field, extra field) []field {
	if len(fields) == 0 {
		return []field{extra}
	}
	out := make([]field, 0, len(fields)+1)
	out = append(out, fields[:len(fields)-1]...)
	out = append(out, extra)
	out = append(out, fields[len(fields)-1])
	return out
}

func drawDANFEBlockTitle(pdf *pdfdraw.PDF, x, y, w float64, title string, config Config) {
	drawDANFEBlockTitleAt(pdf, x, y, w, title, config)
}

func drawDANFEBlockTitleAt(pdf *pdfdraw.PDF, x, y, w float64, title string, config Config) {
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6))
	pdf.SetXY(x, y+1+danfeFieldTextYOffset(config))
	pdf.CellFormat(w, 3, title, "", 1, "L", false, 0, "")
}

func drawDANFEBasicField(pdf *pdfdraw.PDF, x, y, w, h float64, label, value, fieldType string, config Config) {
	pdf.Rect(x, y, w, h, "")
	textY := y + danfeFieldTextYOffset(config)
	pdf.SetXY(x, textY)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5))
	pdf.CellFormat(w, 2.8, label, "", 2, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", fontSize(config, 7))
	align := "L"
	if fieldType == "number" {
		align = "R"
	}
	pdf.MultiCell(w, 3, optional(value), "", align, false)
}

func danfeFieldTextYOffset(config Config) float64 {
	if config.FontType == FontTypeCourier {
		return 1
	}
	return 0
}

func drawProducts(pdf *pdfdraw.PDF, x, y, w, h float64, products []product, taxCodeHeader string, config Config, start, limit int) int {
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x, y+4)
	pdf.CellFormat(w-2, 3, "DADOS DO PRODUTO / SERVIÇO", "", 1, "L", false, 0, "")
	if taxCodeHeader == "" {
		taxCodeHeader = "CST"
	}
	headers := []string{"CÓDIGO", "DESCRIÇÃO DOS PRODUTOS / SERVIÇOS", "NCM/SH", taxCodeHeader, "CFOP", "UN.", "QTD.", "V.UNIT.", "V.TOTAL", "BC.ICMS", "V.ICMS", "V.IPI", "%ICMS", "%IPI"}
	if config.DisplayPISCOFINS {
		headers = append(headers, "PIS", "COFINS")
	}
	widths := productWidths(w, len(headers))
	if taxCodeHeader == "CSOSN" && len(widths) > 3 && widths[1] > 4 {
		widths[1] -= 2
		widths[3] += 2
	}
	top := y + 7
	headerH := 3.0
	xi := x
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 5))
	for i, header := range headers {
		pdf.Rect(xi, top, widths[i], headerH, "")
		pdf.SetXY(xi, top)
		pdf.CellFormat(widths[i]-1, 3, header, "", 0, "L", false, 0, "")
		xi += widths[i]
	}
	rowY := top + headerH
	if start < 0 {
		start = 0
	}
	if start > len(products) {
		start = len(products)
	}
	if limit < start || limit > len(products) {
		limit = len(products)
	}
	drawnLimit := start
	pdf.SetFont(string(config.FontType), "", fontSize(config, 6))
	for i, product := range products[start:limit] {
		rowH := productCellRowHeight(pdf, product, widths[1], config)
		if rowY+rowH > y+h+productRowFitTolerance {
			break
		}
		values := []string{product.Code, product.Desc, product.NCM, product.CST, product.CFOP, product.Unit, product.Quantity, product.UnitPrice, product.TotalPrice, product.BCICMS, product.ICMSValue, product.IPIValue, product.ICMSRate, product.IPIRate}
		if config.DisplayPISCOFINS {
			values = append(values, product.PISValue, product.COFINS)
		}
		xi = x
		pdf.SetFont(string(config.FontType), "", fontSize(config, 6))
		for i, value := range values {
			pdf.Rect(xi, rowY, widths[i], rowH, "")
			align := "L"
			cellX := xi
			cellW := widths[i] - 1
			if i >= 6 && i <= 13 {
				align = "R"
				cellX = xi + 0.5
				cellW = widths[i] - 0.5
			}
			pdf.SetXY(cellX, rowY-0.1)
			if i == 1 {
				pdf.MultiCell(cellW, 3, productCellText(pdf, value, widths[i], true), "", align, false)
			} else {
				pdf.CellFormat(cellW, 3, value, "", 0, align, false, 0, "")
			}
			xi += widths[i]
		}
		rowY += rowH
		drawnLimit = start + i + 1
	}
	return drawnLimit
}

const productRowFitTolerance = 3.1

func drawAdditional(pdf *pdfdraw.PDF, x, y, w, h float64, title string, data nfeData, config Config) {
	if h < 10 {
		return
	}
	pdf.Rect(x, y, w, 1, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x, y+1)
	pdf.CellFormat(w, 3, title, "", 1, "L", false, 0, "")
	reservedW := 70.0
	infoW := w - reservedW
	if infoW < w*0.5 {
		infoW = w
		reservedW = 0
	}
	fieldY := y + 4
	pdf.Rect(x, fieldY, infoW, h, "")
	if reservedW > 0 {
		pdf.Rect(x+infoW, fieldY, reservedW, h, "")
	}
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5))
	pdf.SetXY(x, fieldY)
	pdf.CellFormat(infoW-2, 3, "INFORMAÇÕES COMPLEMENTARES", "", 0, "L", false, 0, "")
	if reservedW > 0 {
		pdf.SetXY(x+infoW, fieldY)
		pdf.CellFormat(reservedW-2, 3, "RESERVADO AO FISCO", "", 0, "L", false, 0, "")
	}
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5.8))
	text := data.AdditionalInfo
	if text == "" {
		text = data.FiscoInfo
	}
	pdf.SetXY(x+1, fieldY+3)
	pdf.MultiCell(infoW-2, 2.7, text, "", "L", false)
}

func drawAdditionalContinuation(pdf *pdfdraw.PDF, x, y, w, h float64, data nfeData, config Config) {
	if h < 10 {
		return
	}
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x, y+1)
	pdf.CellFormat(w-2, 3, "DADOS ADICIONAIS", "", 1, "L", false, 0, "")
	pdf.SetXY(x, y+5)
	pdf.CellFormat(w-2, 3, "CONTINUAÇÃO INFORMAÇÕES COMPLEMENTARES", "", 1, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5.8))
	text := data.AdditionalInfo
	if text == "" {
		text = data.FiscoInfo
	}
	pdf.SetXY(x+1, y+9)
	pdf.MultiCell(w-2, 2.7, text, "", "L", false)
}

func drawBox(pdf *pdfdraw.PDF, x, y, w, h float64, title string, fields []field, config Config, columns int) {
	pdf.Rect(x, y, w, h, "")
	startY := y + 1
	if title != "" {
		pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
		pdf.SetXY(x, y+1)
		pdf.CellFormat(w-2, 3, title, "", 1, "L", false, 0, "")
		startY = y + 5
	}
	if columns <= 0 {
		columns = 3
	}
	colW := w / float64(columns)
	rowH := 8.0
	labelSize := fontSize(config, 4.9)
	valueSize := fontSize(config, 5.8)
	valueLineH := 2.8
	if columns >= 6 {
		rowH = 5.0
		labelSize = fontSize(config, 4.25)
		valueSize = fontSize(config, 5)
		valueLineH = 2.2
	}
	rows := 0
	if len(fields) > 0 {
		rows = (len(fields) + columns - 1) / columns
	}
	if rows > 0 {
		available := h - (startY - y)
		if compactRowH := available / float64(rows); compactRowH > 0 && compactRowH < rowH {
			rowH = compactRowH
			labelSize = minFloat(labelSize, fontSize(config, 4.1))
			valueSize = minFloat(valueSize, fontSize(config, 4.7))
			valueLineH = minFloat(valueLineH, maxFloat(1.5, rowH-2.1))
		}
	}
	for i, field := range fields {
		col := float64(i % columns)
		row := float64(i / columns)
		fx := x + col*colW
		fy := startY + row*rowH
		if fy+rowH > y+h {
			return
		}
		pdf.SetXY(fx+1, fy)
		pdf.SetFont(string(config.FontType), "B", labelSize)
		pdf.CellFormat(colW-2, 2.5, field.Label, "", 2, "L", false, 0, "")
		pdf.SetFont(string(config.FontType), "", valueSize)
		pdf.MultiCell(colW-2, valueLineH, optional(field.Value), "", "L", false)
	}
}

func drawBarcode(pdf *pdfdraw.PDF, x, y, w, h float64, key string, pngBytes []byte) {
	if key == "" || len(pngBytes) == 0 {
		return
	}
	name := "danfe-code128-" + key
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, w, h, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

func drawFooterStamp(pdf *pdfdraw.PDF, config Config) {
	if config.FooterStamp.Logo == "" && len(config.FooterStamp.LogoBytes) == 0 && strings.TrimSpace(config.FooterStamp.Text) == "" {
		return
	}
	pageW, pageH := pdf.GetPageSize()
	y := pageH - config.Margins.Bottom - config.FooterStamp.Height
	x := config.Margins.Left
	w := pageW - config.Margins.Left - config.Margins.Right
	pdf.SetDrawColor(180, 180, 180)
	pdf.Line(x, y-config.FooterStamp.Spacing, x+w, y-config.FooterStamp.Spacing)
	pdf.SetDrawColor(0, 0, 0)
	cursor := x + w/2
	if len(config.FooterStamp.LogoBytes) > 0 {
		logoW := config.FooterStamp.LogoMaxWidth
		pdf.ImageBytes("danfe-footer-logo", config.FooterStamp.LogoBytes, cursor-logoW/2, y, logoW, 0)
		cursor += logoW/2 + 2
	} else if config.FooterStamp.Logo != "" {
		if imageType, _ := images.TypeFromFile(config.FooterStamp.Logo); imageType != "" {
			logoW := config.FooterStamp.LogoMaxWidth
			pdf.ImageOptions(config.FooterStamp.Logo, cursor-logoW/2, y, logoW, 0, false, fpdf.ImageOptions{ImageType: imageType}, 0, "")
			cursor += logoW/2 + 2
		}
	}
	if strings.TrimSpace(config.FooterStamp.Text) != "" {
		pdf.SetFont(string(config.FontType), "", fontSize(config, 6))
		pdf.SetXY(cursor, y+1)
		pdf.CellFormat(w/2, config.FooterStamp.Height, config.FooterStamp.Text, "", 0, "L", false, 0, "")
	}
}

func productWidths(total float64, columns int) []float64 {
	if columns > 14 {
		base := []float64{13, 40, 11, 7, 8, 7, 12, 13, 13, 12, 10, 9, 8, 8, 8, 9}
		return scaleWidths(base, total)
	}
	widths := []float64{15, 0, 11, 6, 7, 6, 12, 13, 13, 13, 10, 10, 9, 8}
	fixed := 0.0
	for _, width := range widths {
		fixed += width
	}
	widths[1] = total - fixed
	if widths[1] < 30 {
		return scaleWidths([]float64{15, 55, 12, 8, 9, 8, 13, 14, 14, 13, 11, 10, 9, 9}, total)
	}
	return widths
}

func productCellRowHeight(pdf *pdfdraw.PDF, product product, descWidth float64, config Config) float64 {
	lineHeight := 3.0
	if config.FontSize == FontSizeBig {
		lineHeight = 4.2
	}
	lines := 1
	if desc := strings.TrimSpace(product.Desc); desc != "" {
		lines = productCellLineCount(pdf, desc, descWidth-1)
	}
	return maxFloat(productRowHeight(config), float64(lines)*lineHeight)
}

func productCellText(pdf *pdfdraw.PDF, value string, width float64, preserveLines bool) string {
	if !preserveLines {
		return fiscalfmt.LimitText(value, maxChars(width))
	}
	return value
}

func productCellLineCount(pdf *pdfdraw.PDF, value string, width float64) int {
	if width <= 0 {
		return len(strings.Split(value, "\n"))
	}
	count := 0
	for _, line := range strings.Split(value, "\n") {
		if line == "" {
			count++
			continue
		}
		split := pdf.SplitLines([]byte(pdf.Encode(line)), width)
		if len(split) == 0 {
			count++
		} else {
			count += len(split)
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

func scaleWidths(base []float64, total float64) []float64 {
	sum := 0.0
	for _, value := range base {
		sum += value
	}
	out := make([]float64, len(base))
	for i, value := range base {
		out[i] = value * total / sum
	}
	return out
}

func receiptText(data nfeData) string {
	return fmt.Sprintf("RECEBEMOS DE %s OS PRODUTOS/SERVIÇOS CONSTANTES DA NOTA FISCAL INDICADA ABAIXO. EMISSÃO: %s VALOR TOTAL: %s DESTINATARIO: %s, %s: %s",
		data.Issuer.Name,
		data.EmissionDate,
		receiptTotalValue(data.Totals),
		receiptRecipientAddress(data.Recipient),
		receiptDocLabel(data.Recipient.Doc),
		data.Recipient.Doc,
	)
}

func receiptTotalValue(fields []field) string {
	for _, field := range fields {
		if field.Label == "VALOR TOTAL DA NOTA" {
			return strings.TrimSpace(strings.TrimPrefix(field.Value, "R$"))
		}
	}
	return "0,00"
}

func receiptRecipientAddress(p party) string {
	return strings.Join(nonEmpty(
		p.Name,
		strings.Join(nonEmpty(p.Street, p.Number, p.District), ", "),
		strings.TrimSpace(strings.Join(nonEmpty(p.City, p.UF), " - ")),
	), " - ")
}

func receiptDocLabel(doc string) string {
	if strings.Contains(doc, "/") {
		return "CNPJ"
	}
	return "CPF"
}

func bottomMargin(config Config) float64 {
	margin := config.Margins.Bottom
	if config.FooterStamp.Logo != "" || len(config.FooterStamp.LogoBytes) > 0 || strings.TrimSpace(config.FooterStamp.Text) != "" {
		margin += config.FooterStamp.Height + config.FooterStamp.Spacing
	}
	return margin
}

func danfePageLabel(pdf *pdfdraw.PDF, data nfeData) string {
	if data.NeedsContinuationPage {
		return fmt.Sprintf("%d/2", pdf.PageNo())
	}
	return "1/1"
}

func fontSize(config Config, size float64) float64 {
	if config.FontType == FontTypeTimes {
		return size * float64(config.FontSize)
	}
	return size
}

func maxChars(width float64) int {
	chars := int(width * 1.15)
	if chars < 4 {
		return 4
	}
	return chars
}

func formatInvoiceNumber(number string) string {
	digits := fiscalfmt.NumberFilter(number)
	if digits == "" {
		return number
	}
	for len(digits) < 9 {
		digits = "0" + digits
	}
	if len(digits) <= 3 {
		return digits
	}
	parts := fiscalfmt.Chunks(digits, 3)
	return strings.Join(parts, ".")
}

func nfType(code string) string {
	switch code {
	case "0":
		return "ENTRADA"
	case "1":
		return "SAÍDA"
	default:
		return code
	}
}

func freightMode(code string) string {
	switch code {
	case "0":
		return "0 - Remetente"
	case "1":
		return "1 - Destinatário"
	case "2":
		return "2 - Terceiros"
	case "3":
		return "3 - Próprio/Rem"
	case "4":
		return "4 - Próprio/Dest"
	case "9":
		return "9 - Sem Frete"
	default:
		return code
	}
}

func money(value string) string {
	if value == "" {
		value = "0,00"
	}
	return "R$ " + value
}

func rate(value string) string {
	if value == "" {
		return "-"
	}
	return fiscalfmt.FormatNumber(value, 2)
}

func optional(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func normalizeSpaces(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNode(nodes ...*xmlutil.Node) *xmlutil.Node {
	for _, node := range nodes {
		if node != nil {
			return node
		}
	}
	return nil
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "-" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
