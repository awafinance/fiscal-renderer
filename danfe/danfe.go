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

	data := nfeData{
		Key:                   key,
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
		QRCode:                firstNonEmpty(xmlutil.Text(root.Find("infNFeSupl"), "qrCode"), xmlutil.Text(root.Find("infNFeSupl"), "qrCodNFe")),
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
	return 58
}

func recipientAdvance(data nfeData) float64 {
	if data.Orientation == "L" {
		return 22
	}
	return 27
}

func taxBlockMetrics(data nfeData) (height, advance float64) {
	if data.Orientation == "L" {
		return 12, 13
	}
	return 25, 28
}

func shippingBlockMetrics(data nfeData) (height, advance float64) {
	if data.Orientation == "L" {
		return 18, 19
	}
	return 21, 24
}

func productReserve(data nfeData) float64 {
	if data.Orientation == "L" {
		return 23
	}
	return 52
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
		drawHeader(pdf, x+2, y+2, w-4, data, config, true)
		y += headerAdvance(data)
		if productsNeedContinuation(data.Products, data.Orientation) {
			start := data.ProductSplitIndex
			drawProducts(pdf, x+2, y, w-4, h-103, data.Products, data.ProductTaxCodeHeader, config, start, len(data.Products))
			drawAdditionalContinuation(pdf, x+2, y+h-42, w-4, 38, data, config)
		} else {
			drawAdditionalContinuation(pdf, x+2, y, w-4, h-64, data, config)
		}
		drawFooterStamp(pdf, config)
		return 0
	}
	if data.Orientation != "L" && config.ReceiptPosition == ReceiptPositionTop {
		drawReceipt(pdf, x, y, w, 18, data, config)
		y += 21
	}
	drawHeader(pdf, x+2, y+2, w-4, data, config, false)
	y += headerAdvance(data)
	drawRecipient(pdf, x+2, y, w-4, data, config)
	y += recipientAdvance(data)
	if data.Pickup != nil {
		drawLocation(pdf, x+2, y, w-4, "INFORMAÇÕES DO LOCAL DE RETIRADA", *data.Pickup, config)
		y += locationBlockHeight
	}
	if data.Delivery != nil {
		drawLocation(pdf, x+2, y, w-4, "INFORMAÇÕES DO LOCAL DE ENTREGA", *data.Delivery, config)
		y += locationBlockHeight
	}
	if data.Billing != nil {
		drawInvoices(pdf, x+2, y, w-4, 20, data.Billing, data.Invoices, config)
		y += 23
	}
	taxHeight, taxAdvance := taxBlockMetrics(data)
	drawBox(pdf, x+2, y, w-4, taxHeight, "CÁLCULO DO IMPOSTO", append(data.Totals, data.Taxes...), config, 6)
	y += taxAdvance
	shippingHeight, shippingAdvance := shippingBlockMetrics(data)
	drawBox(pdf, x+2, y, w-4, shippingHeight, "TRANSPORTADOR / VOLUMES TRANSPORTADOS", data.Shipping, config, 6)
	y += shippingAdvance
	remaining := h - (y - config.Margins.Top)
	if data.Orientation != "L" && config.ReceiptPosition == ReceiptPositionBottom {
		remaining -= 22
	}
	productH := normalizedProductHeight(remaining - productReserve(data))
	productLimit := len(data.Products)
	if productsNeedContinuation(data.Products, data.Orientation) {
		productLimit = data.ProductSplitIndex
		if productLimit == 0 {
			productLimit = productLimitForHeight(productH, productRowHeight(config))
		}
	}
	drawProducts(pdf, x+2, y, w-4, productH, data.Products, data.ProductTaxCodeHeader, config, 0, productLimit)
	y += productH + 3
	drawAdditional(pdf, x+2, y, w-4, minFloat(46, h-(y-config.Margins.Top)-4), "DADOS ADICIONAIS", data, config)
	if data.Orientation != "L" && config.ReceiptPosition == ReceiptPositionBottom {
		drawReceipt(pdf, x, pageH-bottomMargin(config)-20, w, 18, data, config)
	}
	drawFooterStamp(pdf, config)
	return productLimit
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
	numberW := 33.0
	pdf.Line(x+w-numberW, y, x+w-numberW, y+h)
	pdf.Line(x, y+h/2, x+w-numberW, y+h/2)
	pdf.Line(x+41, y+h/2, x+41, y+h)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5.4))
	pdf.SetXY(x+1, y+1)
	pdf.MultiCell(w-numberW-2, 2.7, receiptText(data), "", "L", false)
	pdf.SetXY(x+1, y+h/2+1)
	pdf.CellFormat(40, 3, "DATA DE RECEBIMENTO", "", 0, "L", false, 0, "")
	pdf.CellFormat(w-numberW-42, 3, "IDENTIFICAÇÃO E ASSINATURA DO RECEBEDOR", "", 0, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 9))
	pdf.SetXY(x+w-numberW, y+1)
	pdf.MultiCell(numberW, 4.5, "NF-e\nNº "+formatInvoiceNumber(data.Number)+"\nSÉRIE "+data.Series, "", "C", false)
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
	h := 40.0
	operationH := 13.0
	titleY := 3.0
	entryY := 18.0
	invoiceY := 25.0
	barcodeY := 2.0
	keyY := 17.0
	queryY := 29.0
	if data.Orientation == "L" {
		h = 29
		operationH = 10
		titleY = 2
		entryY = 13
		invoiceY = 18
		barcodeY = 1.5
		keyY = 12
		queryY = 21
	}
	emitW := w - 122
	if emitW < 65 {
		emitW = w * 0.38
	}
	idW := 34.0
	codeW := w - emitW - idW
	pdf.Rect(x, y, emitW, h, "")
	if len(config.LogoBytes) > 0 {
		pdf.ImageBytes("danfe-logo", config.LogoBytes, x+2, y+2, 20, 0)
	} else if config.Logo != "" {
		if imageType, _ := images.TypeFromFile(config.Logo); imageType != "" {
			pdf.ImageOptions(config.Logo, x+2, y+2, 20, 0, false, fpdf.ImageOptions{ImageType: imageType}, 0, "")
		}
	}
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 7))
	pdf.SetXY(x+24, y+2)
	pdf.MultiCell(emitW-26, 3.2, data.Issuer.Name, "", "L", false)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 6))
	pdf.SetXY(x+24, y+9)
	pdf.MultiCell(emitW-26, 3, strings.Join(nonEmpty(data.Issuer.Address, data.Issuer.City+" - "+data.Issuer.UF, data.Issuer.CEP, "Fone: "+data.Issuer.Phone), "\n"), "", "L", false)
	pdf.Rect(x+emitW, y, idW, h, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 10))
	title := "DANFE"
	if continuation {
		title = "DANFE\nCONTINUAÇÃO"
	}
	pdf.SetXY(x+emitW, y+titleY)
	pdf.MultiCell(idW, 3.2, title+"\nDOCUMENTO AUXILIAR\nDA NOTA FISCAL\nELETRÔNICA", "", "C", false)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 6))
	pdf.SetXY(x+emitW+1, y+entryY)
	pdf.MultiCell(idW-2, 3, "0-ENTRADA\n1-SAÍDA", "", "L", false)
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 7))
	pdf.SetXY(x+emitW+1, y+invoiceY)
	pdf.MultiCell(idW-2, 3, data.NFTypeCode+"\nNº "+formatInvoiceNumber(data.Number)+"\nSÉRIE "+data.Series+"\nFOLHA "+danfePageLabel(pdf, data), "", "C", false)
	pdf.Rect(x+emitW+idW, y, codeW, h, "")
	drawBarcode(pdf, x+emitW+idW+4, y+barcodeY, codeW-8, data.Key)
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x+emitW+idW+2, y+keyY)
	pdf.MultiCell(codeW-4, 3, "CHAVE DE ACESSO\n"+strings.Join(fiscalfmt.Chunks(data.Key, 4), " "), "", "C", false)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5.8))
	pdf.SetXY(x+emitW+idW+2, y+queryY)
	pdf.MultiCell(codeW-4, 3, "Consulta de autenticidade no portal nacional da NF-e www.nfe.fazenda.gov.br/portal ou no site da Sefaz autorizadora", "", "C", false)
	drawBox(pdf, x, y+h+1, w, operationH, "", []field{
		{"NATUREZA DA OPERAÇÃO", data.Nature},
		{"PROTOCOLO DE AUTORIZAÇÃO DE USO", strings.TrimSpace(data.Protocol + " - " + data.ProtocolDate + " " + data.ProtocolTime)},
		{"INSCRIÇÃO ESTADUAL", data.Issuer.IE},
		{"CNPJ / CPF", data.Issuer.Doc},
	}, config, 4)
}

func drawRecipient(pdf *pdfdraw.PDF, x, y, w float64, data nfeData, config Config) {
	name := data.Recipient.Name
	if data.EnvironmentCode != "1" {
		name = "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
	}
	fields := []field{
		{"NOME / RAZÃO SOCIAL", name},
		{"CNPJ / CPF", data.Recipient.Doc},
		{"DATA DA EMISSÃO", data.EmissionDate},
		{"ENDEREÇO", data.Recipient.Address},
		{"BAIRRO / DISTRITO", data.Recipient.District},
		{"CEP", data.Recipient.CEP},
		{"DATA ENTRADA / SAÍDA", data.ExitDate},
		{"MUNICÍPIO", data.Recipient.City},
		{"FONE / FAX", data.Recipient.Phone},
		{"UF", data.Recipient.UF},
		{"INSCRIÇÃO ESTADUAL", data.Recipient.IE},
		{"HORA ENTRADA / SAÍDA", data.ExitTime},
	}
	height := 24.0
	if data.Orientation == "L" {
		height = 21
	}
	drawBox(pdf, x, y, w, height, "DESTINATÁRIO / REMETENTE", fields, config, 4)
}

const locationBlockHeight = 21.0

func drawLocation(pdf *pdfdraw.PDF, x, y, w float64, title string, p party, config Config) {
	pdf.Rect(x, y, w, locationBlockHeight, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x+1, y+1)
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
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 3, "FATURA / DUPLICATAS", "", 1, "L", false, 0, "")
	nextY := y + 5
	if config.InvoiceDisplay == InvoiceDisplayFullDetails && bill != nil {
		fields := []field{
			{Label: "NÚMERO", Value: bill.Number},
			{Label: "VALOR ORIGINAL", Value: bill.OriginalValue},
			{Label: "VALOR DO DESCONTO", Value: bill.DiscountValue},
			{Label: "VALOR LÍQUIDO", Value: bill.NetValue},
		}
		drawBox(pdf, x, nextY, w, 7, "", fields, config, 4)
		nextY += 7
	}
	if len(invoices) == 0 {
		return
	}
	cols := minInt(len(invoices), 6)
	colW := w / float64(cols)
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5.8))
	for i, inv := range invoices {
		if i >= 6 {
			break
		}
		pdf.Rect(x+float64(i)*colW, nextY, colW, 3, "")
		pdf.SetXY(x+float64(i)*colW+1, nextY+0.2)
		pdf.CellFormat(colW-2, 2.5, strings.Join(nonEmpty(inv.Number, inv.Due, inv.Value), "  "), "", 0, "L", false, 0, "")
	}
}

func drawProducts(pdf *pdfdraw.PDF, x, y, w, h float64, products []product, taxCodeHeader string, config Config, start, limit int) {
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x+1, y+1)
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
	top := y + 5
	xi := x
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 4.8))
	for i, header := range headers {
		pdf.Rect(xi, top, widths[i], 6, "")
		pdf.SetXY(xi+0.3, top+1)
		pdf.MultiCell(widths[i]-0.6, 2, header, "", "C", false)
		xi += widths[i]
	}
	rowY := top + 6
	rowH := productRowHeight(config)
	if start < 0 {
		start = 0
	}
	if start > len(products) {
		start = len(products)
	}
	if limit < start || limit > len(products) {
		limit = len(products)
	}
	for _, product := range products[start:limit] {
		if rowY+rowH > y+h-1 {
			break
		}
		values := []string{product.Code, product.Desc, product.NCM, product.CST, product.CFOP, product.Unit, product.Quantity, product.UnitPrice, product.TotalPrice, product.BCICMS, product.ICMSValue, product.IPIValue, product.ICMSRate, product.IPIRate}
		if config.DisplayPISCOFINS {
			values = append(values, product.PISValue, product.COFINS)
		}
		xi = x
		pdf.SetFont(string(config.FontType), "", fontSize(config, 4.7))
		for i, value := range values {
			pdf.Rect(xi, rowY, widths[i], rowH, "")
			pdf.SetXY(xi+0.3, rowY+0.8)
			align := "C"
			if i == 1 {
				align = "L"
			}
			pdf.MultiCell(widths[i]-0.6, 2.3, fiscalfmt.LimitText(value, maxChars(widths[i])), "", align, false)
			xi += widths[i]
		}
		rowY += rowH
	}
}

func drawAdditional(pdf *pdfdraw.PDF, x, y, w, h float64, title string, data nfeData, config Config) {
	if h < 10 {
		return
	}
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 3, title, "", 1, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", fontSize(config, 5.8))
	text := data.AdditionalInfo
	if text == "" {
		text = data.FiscoInfo
	}
	pdf.SetXY(x+1, y+5)
	pdf.MultiCell(w-2, 2.7, text, "", "L", false)
}

func drawAdditionalContinuation(pdf *pdfdraw.PDF, x, y, w, h float64, data nfeData, config Config) {
	if h < 10 {
		return
	}
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", fontSize(config, 6.2))
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 3, "DADOS ADICIONAIS", "", 1, "L", false, 0, "")
	pdf.SetXY(x+1, y+5)
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
		pdf.SetXY(x+1, y+1)
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

func drawBarcode(pdf *pdfdraw.PDF, x, y, w float64, key string) {
	if key == "" {
		return
	}
	pngBytes, err := barcode.Code128PNG(key, 450, 60)
	if err != nil {
		return
	}
	name := "danfe-code128-" + key
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, w, 10, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
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
	base := []float64{15, 55, 12, 8, 9, 8, 13, 14, 14, 13, 11, 10, 9, 9}
	return scaleWidths(base, total)
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
