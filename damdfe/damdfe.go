package damdfe

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/awafinance/fiscal-renderer/internal/barcode"
	"github.com/awafinance/fiscal-renderer/internal/fiscalfmt"
	"github.com/awafinance/fiscal-renderer/internal/images"
	"github.com/awafinance/fiscal-renderer/internal/pdfdraw"
	"github.com/awafinance/fiscal-renderer/internal/qrcode"
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
	pdf := pdfdraw.NewPDF("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(d.Config.Margins.Left, d.Config.Margins.Top, d.Config.Margins.Right)
	pdf.SetAutoPageBreak(false, d.Config.Margins.Bottom)
	pdf.SetTitle("DAMDFE", false)
	pdf.AddPage()
	draw(pdf, data, d.Config)
	return pdf.Output(w)
}

func RenderFile(xml string, path string, config *Config) error {
	doc, err := New(xml, config)
	if err != nil {
		return err
	}
	return doc.Output(path)
}

type mdfeData struct {
	Key                   string
	Model                 string
	Series                string
	Number                string
	EmissionDate          string
	EmissionTime          string
	Deadline              string
	UFStart               string
	UFEnd                 string
	EmissionType          string
	EmissionTypeCode      string
	TripStart             string
	TripStartTime         string
	IssuerType            string
	IssuerTypeCode        string
	Environment           string
	EnvironmentCode       string
	Protocol              string
	ProtocolDate          string
	ProtocolTime          string
	QRCode                string
	Issuer                issuer
	ModalCode             string
	ModalName             string
	RNTRC                 string
	QuantityNFe           string
	QuantityCTe           string
	CargoQuantity         string
	CargoUnit             string
	CargoValue            string
	VehiclePlate          string
	VehicleUF             string
	VehicleRNTRC          string
	VehicleRENAVAM        string
	Drivers               []driver
	Aereo                 []field
	Ferroviario           []field
	Aquaviario            []field
	RodoviarioPedagio     []field
	CargoComposition      []field
	Route                 string
	Documents             []linkedDocument
	Insurance             []insurance
	CIOT                  []field
	ComplementaryInfo     string
	FiscoInfo             string
	HasProtocol           bool
	HasContingency        bool
	DisplayRoutePrestacao bool
}

type issuer struct {
	Name     string
	Street   string
	Number   string
	District string
	CEP      string
	City     string
	UF       string
	CNPJ     string
	IE       string
	Phone    string
	RNTRC    string
}

type driver struct {
	Name string
	CPF  string
}

type linkedDocument struct {
	Municipality string
	Key          string
	Kind         string
}

type insurance struct {
	Name        string
	CNPJ        string
	Policy      string
	Endorsement []string
}

type field struct {
	Label string
	Value string
}

func parseData(root *xmlutil.Node, config Config) mdfeData {
	infMDFe := root.Find("infMDFe")
	ide := root.Find("ide")
	emit := root.Find("emit")
	infModal := root.Find("infModal")
	protocolNode := root.Find("protMDFe")
	infDoc := root.Find("infDoc")
	tot := root.Find("tot")
	infSupl := root.Find("infMDFeSupl")
	infAdic := root.Find("infAdic")

	emissionDate, emissionTime := fiscalfmt.DateUTC(xmlutil.Text(ide, "dhEmi"))
	deadline := contingencyDeadline(emissionDate, emissionTime)
	tripDate, tripTime := fiscalfmt.DateUTC(xmlutil.Text(ide, "dhIniViagem"))
	protocolDate, protocolTime := fiscalfmt.DateUTC(xmlutil.Text(protocolNode, "dhRecbto"))

	modalCode := xmlutil.Text(ide, "modal")
	tpEmis := xmlutil.Text(ide, "tpEmis")
	tpAmb := xmlutil.Text(ide, "tpAmb")

	return mdfeData{
		Key:                   strings.TrimPrefix(infMDFe.Attr("Id"), "MDFe"),
		Model:                 xmlutil.Text(ide, "mod"),
		Series:                xmlutil.Text(ide, "serie"),
		Number:                xmlutil.Text(ide, "nMDF"),
		EmissionDate:          emissionDate,
		EmissionTime:          emissionTime,
		Deadline:              deadline,
		UFStart:               xmlutil.Text(ide, "UFIni"),
		UFEnd:                 xmlutil.Text(ide, "UFFim"),
		EmissionType:          emissionTypeName(tpEmis),
		EmissionTypeCode:      tpEmis,
		TripStart:             tripDate,
		TripStartTime:         tripTime,
		IssuerType:            issuerTypeName(xmlutil.Text(ide, "tpEmit")),
		IssuerTypeCode:        xmlutil.Text(ide, "tpEmit"),
		Environment:           environmentName(tpAmb),
		EnvironmentCode:       tpAmb,
		Protocol:              xmlutil.Text(protocolNode, "nProt"),
		ProtocolDate:          protocolDate,
		ProtocolTime:          protocolTime,
		QRCode:                xmlutil.RawText(infSupl, "qrCodMDFe"),
		Issuer:                parseIssuer(emit, infModal),
		ModalCode:             modalCode,
		ModalName:             modalName(modalCode),
		RNTRC:                 xmlutil.Text(infModal, "RNTRC"),
		QuantityNFe:           xmlutil.Text(tot, "qNFe"),
		QuantityCTe:           xmlutil.Text(tot, "qCTe"),
		CargoQuantity:         xmlutil.Text(tot, "qCarga"),
		CargoUnit:             xmlutil.Text(tot, "cUnid"),
		CargoValue:            money(fiscalfmt.FormatNumber(xmlutil.Text(tot, "vCarga"), 2)),
		VehiclePlate:          xmlutil.Text(infModal, "placa"),
		VehicleUF:             xmlutil.Text(infModal, "UF"),
		VehicleRNTRC:          xmlutil.Text(infModal, "RNTRC"),
		VehicleRENAVAM:        xmlutil.Text(infModal, "RENAVAM"),
		Drivers:               parseDrivers(infModal),
		Aereo:                 parseAereo(infModal),
		Ferroviario:           parseFerroviario(root.Find("ferrov")),
		Aquaviario:            parseAquaviario(root.Find("aquav")),
		RodoviarioPedagio:     parsePedagio(infModal),
		CargoComposition:      parseCargoComposition(root.Find("aquav")),
		Route:                 parseRoute(ide, infDoc, config.DisplayOrigemDestinoPrestacao),
		Documents:             parseDocuments(infDoc),
		Insurance:             parseInsurance(root.FindAll("seg")),
		CIOT:                  parseCIOT(infModal),
		ComplementaryInfo:     strings.ReplaceAll(xmlutil.Text(infAdic, "infCpl"), ";", "\n"),
		FiscoInfo:             strings.ReplaceAll(xmlutil.Text(infAdic, "infAdFisco"), ";", "\n"),
		HasProtocol:           protocolNode != nil && xmlutil.Text(protocolNode, "nProt") != "",
		HasContingency:        tpEmis == "2",
		DisplayRoutePrestacao: config.DisplayOrigemDestinoPrestacao,
	}
}

func parseIssuer(emit, infModal *xmlutil.Node) issuer {
	return issuer{
		Name:     xmlutil.Text(emit, "xNome"),
		Street:   xmlutil.Text(emit, "xLgr"),
		Number:   xmlutil.Text(emit, "nro"),
		District: xmlutil.Text(emit, "xBairro"),
		CEP:      fiscalfmt.FormatCEP(xmlutil.Text(emit, "CEP")),
		City:     xmlutil.Text(emit, "xMun"),
		UF:       xmlutil.Text(emit, "UF"),
		CNPJ:     fiscalfmt.FormatCPFCNPJ(xmlutil.Text(emit, "CNPJ")),
		IE:       xmlutil.Text(emit, "IE"),
		Phone:    fiscalfmt.FormatPhone(xmlutil.Text(emit, "fone")),
		RNTRC:    xmlutil.Text(infModal, "RNTRC"),
	}
}

func parseDrivers(infModal *xmlutil.Node) []driver {
	var drivers []driver
	for _, node := range infModal.FindAll("condutor") {
		name := xmlutil.Text(node, "xNome")
		cpf := xmlutil.Text(node, "CPF")
		if name != "" || cpf != "" {
			drivers = append(drivers, driver{Name: name, CPF: fiscalfmt.FormatCPFCNPJ(cpf)})
		}
	}
	return drivers
}

func parseAereo(infModal *xmlutil.Node) []field {
	aereo := infModal.Find("aereo")
	if aereo == nil {
		aereo = infModal
	}
	date, _ := fiscalfmt.DateUTC(xmlutil.Text(aereo, "dVoo"))
	return []field{
		{"NACIONALIDADE", xmlutil.Text(aereo, "nac")},
		{"MATRÍCULA", xmlutil.Text(aereo, "matr")},
		{"NÚMERO DO VOO", xmlutil.Text(aereo, "nVoo")},
		{"DATA DO VOO", date},
		{"AERÓDROMO EMBARQUE", xmlutil.Text(aereo, "cAerEmb")},
		{"AERÓDROMO DESTINO", xmlutil.Text(aereo, "cAerDes")},
	}
}

func parseFerroviario(ferrov *xmlutil.Node) []field {
	if ferrov == nil {
		return nil
	}
	var fields []field
	for i, vag := range ferrov.FindAll("vag") {
		fields = append(fields, field{
			Label: fmt.Sprintf("VAGÃO %d", i+1),
			Value: strings.Join(nonEmpty(
				"Série "+xmlutil.Text(vag, "serie"),
				"Nº "+xmlutil.Text(vag, "nVag"),
				"Seq "+xmlutil.Text(vag, "nSeq"),
				"TU "+xmlutil.Text(vag, "TU"),
			), " | "),
		})
	}
	return fields
}

func parseAquaviario(aquav *xmlutil.Node) []field {
	if aquav == nil {
		return nil
	}
	return []field{
		{"IRIN", xmlutil.Text(aquav, "irin")},
		{"Tipo embarcação", xmlutil.Text(aquav, "tpEmb")},
		{"Código embarcação", xmlutil.Text(aquav, "cEmbar")},
		{"Nome embarcação", xmlutil.Text(aquav, "xEmbar")},
		{"Porto embarque", strings.Join(nonEmpty(xmlutil.Text(aquav, "cTermCarreg"), xmlutil.Text(aquav, "xTermCarreg")), " - ")},
		{"Porto destino", strings.Join(nonEmpty(xmlutil.Text(aquav, "cTermDescarreg"), xmlutil.Text(aquav, "xTermDescarreg")), " - ")},
	}
}

func parsePedagio(infModal *xmlutil.Node) []field {
	return []field{
		{"CNPJ DA FORNECEDORA", xmlutil.Text(infModal, "CNPJForn")},
		{"CPF/CNPJ DO RESPONSÁVEL", firstNonEmpty(xmlutil.Text(infModal, "CNPJPg"), xmlutil.Text(infModal, "CPFPg"))},
		{"NÚMERO DO COMPROVANTE", xmlutil.Text(infModal, "nCompra")},
		{"VALOR DO VALE-PEDÁGIO", money(fiscalfmt.FormatNumber(xmlutil.Text(infModal, "vValePed"), 2))},
	}
}

func parseCargoComposition(aquav *xmlutil.Node) []field {
	if aquav == nil {
		return nil
	}
	var fields []field
	for _, unit := range aquav.FindAll("infUnidTranspVazia") {
		fields = append(fields, field{Label: "Unidade transporte", Value: strings.Join(nonEmpty(xmlutil.Text(unit, "idUnidTranspVazia"), xmlutil.Text(unit, "tpUnidTranspVazia")), " - ")})
	}
	for _, unit := range aquav.FindAll("infUnidCargaVazia") {
		fields = append(fields, field{Label: "Unidade carga", Value: strings.Join(nonEmpty(xmlutil.Text(unit, "idUnidCargaVazia"), xmlutil.Text(unit, "tpUnidCargaVazia")), " - ")})
	}
	return fields
}

func parseRoute(ide, infDoc *xmlutil.Node, displayPrestacao bool) string {
	var routeParts []string
	for _, per := range ide.FindAll("UFPer") {
		if text := per.TextContent(); text != "" {
			routeParts = append(routeParts, text)
		}
	}
	route := strings.Join(routeParts, " / ")
	if !displayPrestacao {
		return route
	}
	origin := xmlutil.Text(ide.Find("infMunCarrega"), "xMunCarrega")
	var destinations []string
	seen := map[string]bool{}
	for _, node := range infDoc.FindAll("infMunDescarga") {
		destination := xmlutil.Text(node, "xMunDescarga")
		if destination != "" && !seen[destination] {
			seen[destination] = true
			destinations = append(destinations, destination)
		}
	}
	return strings.Join(nonEmpty(route, "ORIGEM DA PRESTAÇÃO: "+origin, "DESTINO DA PRESTAÇÃO: "+strings.Join(destinations, " / ")), " | ")
}

func parseDocuments(infDoc *xmlutil.Node) []linkedDocument {
	var docs []linkedDocument
	for _, descarga := range infDoc.FindAll("infMunDescarga") {
		municipality := xmlutil.Text(descarga, "xMunDescarga")
		for _, nfe := range descarga.FindAll("infNFe") {
			key := xmlutil.Text(nfe, "chNFe")
			if key != "" {
				docs = append(docs, linkedDocument{Municipality: municipality, Key: key, Kind: "NF-e"})
			}
		}
		for _, cte := range descarga.FindAll("infCTe") {
			key := xmlutil.Text(cte, "chCTe")
			if key != "" {
				docs = append(docs, linkedDocument{Municipality: municipality, Key: key, Kind: "CT-e"})
			}
		}
	}
	return docs
}

func parseInsurance(nodes []*xmlutil.Node) []insurance {
	var out []insurance
	for _, seg := range nodes {
		infSeg := seg.Find("infSeg")
		var endorsements []string
		for _, aver := range seg.FindAll("nAver") {
			if value := aver.TextContent(); value != "" {
				endorsements = append(endorsements, value)
			}
		}
		out = append(out, insurance{
			Name:        xmlutil.Text(infSeg, "xSeg"),
			CNPJ:        xmlutil.Text(infSeg, "CNPJ"),
			Policy:      xmlutil.Text(seg, "nApol"),
			Endorsement: endorsements,
		})
	}
	return out
}

func parseCIOT(infModal *xmlutil.Node) []field {
	var out []field
	for _, node := range infModal.FindAll("infCIOT") {
		ciot := xmlutil.Text(node, "CIOT")
		cnpj := xmlutil.Text(node, "CNPJ")
		cpf := xmlutil.Text(node, "CPF")
		doc := firstNonEmpty(cnpj, cpf)
		if ciot == "" && doc == "" {
			continue
		}
		docKind := "CPF"
		if cnpj != "" {
			docKind = "CNPJ"
		}
		value := fiscalfmt.FormatCPFCNPJ(doc)
		if ciot != "" {
			if value != "" {
				value += " e Nº CIOT: " + ciot
			} else {
				value = "Nº CIOT: " + ciot
			}
		}
		out = append(out, field{Label: "RESPONSÁVEL " + docKind, Value: value})
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func draw(pdf *pdfdraw.PDF, data mdfeData, config Config) {
	drawWatermarks(pdf, data, config)
	x := config.Margins.Left
	y := config.Margins.Top
	w := 210 - config.Margins.Left - config.Margins.Right
	drawHeader(pdf, x, y, w, data, config)
	bodyW := w - 0.5
	drawModal(pdf, x, y+45, bodyW, data, config)
	drawVoucher(pdf, x, y+87.5, bodyW, data, config)
	drawLinkedDocuments(pdf, x, y+118, bodyW, data.Documents, config)
	drawInsurance(pdf, x, y+130, bodyW, data, config)
	drawSupplementaryInfo(pdf, x, y+186, bodyW, 297-config.Margins.Bottom-(y+186), data, config)
}

func drawWatermarks(pdf *pdfdraw.PDF, data mdfeData, config Config) {
	if data.EnvironmentCode != "1" || !data.HasProtocol {
		drawRotatedFiscalValueWatermark(pdf, config)
	}
	if data.HasContingency {
		drawContingencyWatermark(pdf, config)
	}
}

func drawRotatedFiscalValueWatermark(pdf *pdfdraw.PDF, config Config) {
	const (
		text   = "SEM VALOR FISCAL"
		size   = 60.0
		height = 15.0
	)
	pdf.SetTextColor(220, 150, 150)
	pdf.SetFont(string(config.FontType), "B", size)
	width := pdf.GetStringWidth(pdf.Encode(text))
	pageW, pageH := pdf.GetPageSize()
	xCenter := (pageW - width) / 2
	yCenter := (pageH + height) / 2
	pdf.TransformBegin()
	pdf.TransformRotate(55, xCenter+(width/2), yCenter-(height/2))
	pdf.Text(xCenter, yCenter, text)
	pdf.TransformEnd()
	pdf.SetTextColor(0, 0, 0)
}

func drawContingencyWatermark(pdf *pdfdraw.PDF, config Config) {
	const height = 15.0
	pdf.SetTextColor(150, 150, 150)
	pdf.SetFont(string(config.FontType), "B", 60)
	width := pdf.GetStringWidth(pdf.Encode("CONTINGÊNCIA "))
	pageW, pageH := pdf.GetPageSize()
	xCenter := (pageW - width) / 2
	yCenter := (pageH + height) / 2
	pdf.TransformBegin()
	pdf.TransformRotate(0, xCenter+(width/2), yCenter-(height/2))
	pdf.Text(xCenter+18, yCenter+5, "EMISSÃO EM")
	pdf.Text(xCenter+3, yCenter+23, "CONTINGÊNCIA")
	pdf.TransformEnd()
	pdf.SetTextColor(0, 0, 0)
}

func drawHeader(pdf *pdfdraw.PDF, x, y, w float64, data mdfeData, config Config) {
	pdf.Rect(x, y, w, 88, "")
	if len(config.LogoBytes) > 0 {
		pdf.ImageBytes("damdfe-logo", config.LogoBytes, x+2, y+2, 20, 0)
	} else if config.Logo != "" {
		imageType, _ := images.TypeFromFile(config.Logo)
		pdf.ImageOptions(config.Logo, x+2, y+2, 20, 0, false, fpdf.ImageOptions{ImageType: imageType}, 0, "")
	}
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.SetXY(x+25, y+5)
	pdf.MultiCell(63, 3, strings.Join(nonEmpty(
		data.Issuer.Name,
		strings.TrimSpace(data.Issuer.Street+" "+data.Issuer.Number),
		strings.TrimSpace(data.Issuer.District+" "+data.Issuer.CEP),
		strings.TrimSpace(data.Issuer.City+" - "+data.Issuer.UF),
		strings.TrimSpace("CNPJ:"+data.Issuer.CNPJ+" IE:"+data.Issuer.IE),
		strings.TrimSpace("RNTRC:"+data.Issuer.RNTRC+" TELEFONE:"+data.Issuer.Phone),
	), "\n"), "", "L", false)
	pdf.Line(x+w/2, y, x+w/2, y+88)
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.Line(x, y+25, x+w/2, y+25)
	pdf.SetXY(x, y+25)
	pdf.MultiCell(w/2, 3, "DAMDFE - Documento Auxiliar do Manifesto de Documentos Fiscais Eletrônicos", "", "C", false)

	drawHeaderRows(pdf, x, y, w, data, config)

	drawQR(pdf, x+136, y+2, data.QRCode)
	drawBarcode(pdf, x+w/2+5.75, y+32, data.Key)
	pdf.SetFont(string(config.FontType), "", 6.5)
	pdf.SetXY(x+100, y+28)
	pdf.CellFormat(w/2-8, 3, "CONTROLE DO FISCO", "", 1, "L", false, 0, "")
	pdf.SetXY(x+w/2+25, y+51)
	pdf.MultiCell(w/2-20, 3, "Consulta em https://dfe-portal.svrs.rs.gov.br/MDFE/Consulta", "", "L", false)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x+w/2+25, y+56)
	pdf.MultiCell(w/2-20, 3, data.Key, "", "L", false)
	pdf.SetFont(string(config.FontType), "B", 6)
	pdf.SetXY(x+w/2+28, y+60)
	pdf.CellFormat(w/2-20, 3, "PROTOCOLO DE AUTORIZAÇÃO DE USO", "", 1, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", 6)
	if data.HasContingency {
		pdf.SetXY(x+w/2+22, y+62)
		pdf.MultiCell(w/2-22, 3, fmt.Sprintf("EMISSÃO EM CONTINGÊNCIA. Obrigatória a autorização em 168 horas após esta emissão (%s)", data.Deadline), "", "C", false)
	} else {
		pdf.SetXY(x+w/2+32, y+63)
		pdf.MultiCell(w/2-22, 3, strings.TrimSpace(data.Protocol+" "+data.ProtocolDate+" "+data.ProtocolTime), "", "L", false)
	}
}

func drawHeaderRows(pdf *pdfdraw.PDF, x, y, w float64, data mdfeData, config Config) {
	midX := x + w/2
	row1Y := y + 28
	pdf.Line(x, row1Y, x+w, row1Y)
	for _, offset := range []float64{13, 21, 32, 38, 58, 73} {
		pdf.Line(x+offset, row1Y, x+offset, row1Y+7)
	}
	drawHeaderText(pdf, config, x+1, row1Y, "MODELO")
	drawHeaderText(pdf, config, x+4, row1Y+3, data.Model)
	drawHeaderText(pdf, config, x+13, row1Y, "SÉRIE")
	drawHeaderText(pdf, config, x+15, row1Y+3, data.Series)
	drawHeaderText(pdf, config, x+21, row1Y, "NÚMERO")
	drawHeaderText(pdf, config, x+24, row1Y+3, data.Number)
	drawHeaderText(pdf, config, x+33, row1Y, "FL")
	drawHeaderText(pdf, config, x+33, row1Y+3, "1/1")
	drawHeaderText(pdf, config, x+39, row1Y, "DATA E HORA")
	drawHeaderText(pdf, config, x+38, row1Y+3, strings.TrimSpace(data.EmissionDate+" "+data.EmissionTime))
	drawHeaderText(pdf, config, x+59, row1Y, "UF CARREG")
	drawHeaderText(pdf, config, x+63, row1Y+3, data.UFStart)
	drawHeaderText(pdf, config, x+77, row1Y, "UF DESCARREG")
	drawHeaderText(pdf, config, x+84, row1Y+3, data.UFEnd)

	row2Y := y + 35
	pdf.Line(x, row2Y, midX, row2Y)
	for _, offset := range []float64{24, 64} {
		pdf.Line(x+offset, row2Y, x+offset, row2Y+7)
	}
	drawHeaderText(pdf, config, x, row2Y, "FORMA DE EMISSÃO")
	drawHeaderText(pdf, config, x+6, row2Y+3, data.EmissionType)
	drawHeaderText(pdf, config, x+25, row2Y, "PREVISÃO DE INICIO DA VIAGEM")
	drawHeaderText(pdf, config, x+32.5, row2Y+3, strings.TrimSpace(data.TripStart+" "+data.TripStartTime))
	drawHeaderText(pdf, config, x+73, row2Y, "INSC. SUFRAMA")

	row3Y := y + 42
	pdf.Line(x, row3Y, midX, row3Y)
	for _, offset := range []float64{44, 70} {
		pdf.Line(x+offset, row3Y, x+offset, row3Y+8)
	}
	drawHeaderText(pdf, config, x+11, row3Y, "TIPO DO EMITENTE")
	if data.IssuerTypeCode == "3" {
		drawHeaderTextWithHeight(pdf, config, x, row3Y+3, "PRESTADOR DE SERVIÇO DE TRANSPORTE", 2)
		drawHeaderTextWithHeight(pdf, config, x+3.5, row3Y+6, "TRANSPORTE (CT-e GLOBALIZADO)", 2)
	} else {
		drawHeaderText(pdf, config, x, row3Y+3, data.IssuerType)
	}
	drawHeaderText(pdf, config, x+46, row3Y, "TIPO DO AMBIENTE")
	environmentX := x + 47.5
	if data.Environment == "PRODUÇÃO" {
		environmentX = x + 50
	}
	drawHeaderText(pdf, config, environmentX, row3Y+3, data.Environment)
	drawHeaderText(pdf, config, x+73, row3Y, "CARGA POSTERIOR")
	pdf.Line(x, y+50, x+w, y+50)
}

func drawHeaderText(pdf *pdfdraw.PDF, config Config, x, y float64, text string) {
	drawHeaderTextWithHeight(pdf, config, x, y, text, 3)
}

func drawHeaderTextWithHeight(pdf *pdfdraw.PDF, config Config, x, y float64, text string, height float64) {
	pdf.SetFont(string(config.FontType), "", 6)
	pdf.SetXY(x, y)
	pdf.MultiCell(100, height, text, "", "L", false)
}

func drawModal(pdf *pdfdraw.PDF, x, y, w float64, data mdfeData, config Config) {
	midX := x + w/2
	pdf.Line(x, y+10, midX, y+10)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x-2, y+8)
	pdf.MultiCell(100, 0, fmt.Sprintf("MODAL %s DE CARGA", data.ModalName), "", "C", false)

	pdf.Line(x, y+15, x+w, y+15)
	pdf.SetXY(x-2, y+13)
	pdf.MultiCell(100, 0, "INFORMAÇÕES PARA ANTT", "", "C", false)
	for _, offset := range []float64{25, 50, 75} {
		pdf.Line(x+offset, y+15, x+offset, y+22)
	}
	drawBodyField(pdf, config, x, y+15, "QTD. CT-e", optional(data.QuantityCTe), 25)
	drawBodyField(pdf, config, x+25, y+15, "QTD. NF-e", optional(data.QuantityNFe), 25)
	drawBodyField(pdf, config, x+50, y+15, "PESO TOTAL", strings.TrimSpace(optional(data.CargoQuantity)+" "+cargoUnit(data.CargoUnit)), 25)
	drawBodyField(pdf, config, x+75, y+15, "VALOR TOTAL", data.CargoValue, 25)
	pdf.Line(x, y+22, x+w, y+22)
	pdf.Line(x, y+26, x+w, y+26)

	switch data.ModalCode {
	case "1":
		pdf.SetFont(string(config.FontType), "B", 7)
		pdf.SetXY(x+42, y+24)
		pdf.MultiCell(100, 0, "VEÍCULOS", "", "L", false)
		for _, offset := range []float64{25, 50, 75} {
			pdf.Line(x+offset, y+26, x+offset, y+43)
		}
		drawBodyField(pdf, config, x, y+26, "PLACA", data.VehiclePlate, 25)
		drawBodyField(pdf, config, x+25, y+26, "UF", data.VehicleUF, 25)
		drawBodyField(pdf, config, x+50, y+26, "RNTRC", data.VehicleRNTRC, 25)
		drawBodyField(pdf, config, x+75, y+26, "RENAVAM", data.VehicleRENAVAM, 25)
		pdf.SetFont(string(config.FontType), "B", 7)
		pdf.SetXY(w/2+6, y+24)
		pdf.MultiCell(100, 0, "CONDUTORES", "", "C", false)
		halfW := w / 2
		rightX := x + halfW
		pdf.Line(x, y+29, x+w, y+29)
		pdf.Line(rightX+halfW/2, y+26, rightX+halfW/2, y+43)
		pdf.Line(x+w, y+26, x+w, y+43)
		cpfX := rightX + 0.5
		driverNameX := rightX + 50.5
		pdf.SetXY(cpfX, y+26.2)
		pdf.MultiCell(45, 3, "CPF", "", "L", false)
		pdf.SetXY(driverNameX, y+26.2)
		pdf.MultiCell(45, 3, "CONDUTORES", "", "L", false)
		driverY := y + 29.5
		for _, driver := range data.Drivers {
			pdf.SetFont(string(config.FontType), "", 7)
			pdf.SetXY(cpfX, driverY)
			pdf.MultiCell(45, 2.5, driver.CPF, "", "L", false)
			pdf.SetXY(driverNameX, driverY)
			pdf.MultiCell(45, 2.5, driver.Name, "", "L", false)
			driverY += 2.5
		}
		pdf.Line(x, y+43, x+w, y+43)
	case "2":
		drawBodyFields(pdf, config, x, y+26, w, data.Aereo)
		pdf.Line(x, y+31.5, x+w, y+31.5)
	case "3":
		drawBodyFields(pdf, config, x, y+26, w, append(data.Aquaviario, data.CargoComposition...))
		pdf.Line(x, y+31.5, x+w, y+31.5)
	case "4":
		drawBodyFields(pdf, config, x, y+26, w, data.Ferroviario)
		pdf.Line(x, y+31.5, x+w, y+31.5)
	}
	pdf.Line(x, y+60, x+w, y+60)
}

func drawVoucher(pdf *pdfdraw.PDF, x, y, w float64, data mdfeData, config Config) {
	if data.ModalCode != "1" {
		return
	}
	pdf.Rect(x, y+0.5, w, 30, "")
	pdf.Line(x, y+4.5, x+w, y+4.5)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY((w-40)/2, y+2.5)
	pdf.MultiCell(100, 0, "INFORMAÇÕES DE VALE PEDÁGIO", "", "L", false)
	for _, offset := range []float64{50, 100, 150} {
		pdf.Line(x+offset, y+4.5, x+offset, y+8.5)
	}
	labels := []field{
		{"CNPJ DA FORNECEDORA", ""},
		{"CPF/CNPJ DO RESPONSÁVEL", ""},
		{"NÚMERO DO COMPROVANTE", ""},
		{"VALOR DO VALE-PEDÁGIO", ""},
	}
	copy(labels, data.RodoviarioPedagio)
	titleLineY := y + 4.5
	labelXs := []float64{x + 12, x + 59, titleLineY + 15, titleLineY + 67}
	valueXs := []float64{x + 17, x + 66, titleLineY + 21, titleLineY + 75}
	for i, field := range labels {
		drawVoucherField(pdf, config, labelXs[i], valueXs[i], titleLineY, field.Label, voucherValue(i, field.Value))
	}
	pdf.Line(x, y+8.5, x+w, y+8.5)
	routeY := y + 21.5
	pdf.Line(x, routeY, x+w, routeY)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY((w-18)/2, routeY-2)
	pdf.MultiCell(100, 0, "PERCURSO", "", "L", false)
	pdf.SetFont(string(config.FontType), "", 6.5)
	pdf.SetXY(x, routeY+1.5)
	pdf.MultiCell(w, 0, data.Route, "", "L", false)
	pdf.Line(x, y+25.5, x+w, y+25.5)
}

func drawVoucherField(pdf *pdfdraw.PDF, config Config, labelX, valueX, y float64, label, value string) {
	pdf.SetFont(string(config.FontType), "B", 6.5)
	pdf.SetXY(labelX, y+1)
	pdf.MultiCell(100, 3, label, "", "L", false)
	pdf.SetFont(string(config.FontType), "", 6.5)
	pdf.SetXY(valueX, y+5)
	pdf.MultiCell(100, 3, value, "", "L", false)
}

func voucherValue(index int, value string) string {
	value = strings.TrimSpace(value)
	if value == "-" {
		return ""
	}
	if index == 3 && (value == "" || value == "R$ 0,00" || value == "R$ 0.00") {
		return ""
	}
	return value
}

func drawBodyFields(pdf *pdfdraw.PDF, config Config, x, y, w float64, fields []field) {
	colW := w / 4
	for i, field := range fields {
		col := float64(i % 4)
		row := float64(i / 4)
		drawBodyField(pdf, config, x+col*colW, y+row*7, field.Label, field.Value, colW)
	}
}

func drawBodyField(pdf *pdfdraw.PDF, config Config, x, y float64, label, value string, w float64) {
	pdf.SetFont(string(config.FontType), "B", 6.5)
	pdf.SetXY(x, y+0.5)
	pdf.MultiCell(w, 3, label, "", "L", false)
	pdf.SetFont(string(config.FontType), "", 6.5)
	pdf.SetXY(x, y+4)
	pdf.MultiCell(w, 3, value, "", "L", false)
}

func drawLinkedDocuments(pdf *pdfdraw.PDF, x, y, w float64, docs []linkedDocument, config Config) {
	pdf.Rect(x, y, w, 4, "")
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY((w-50)/2, y-2)
	pdf.MultiCell(100, 0, "INFORMAÇÕES DA COMPOSIÇÃO DA CARGA", "", "L", false)
	for _, offset := range []float64{30, 92, 125} {
		pdf.Line(x+offset, y, x+offset, y+4)
	}
	pdf.SetFont(string(config.FontType), "", 5.5)
	pdf.SetXY(x, y+1)
	pdf.CellFormat(30, 3, "MUNICÍPIO", "", 0, "L", false, 0, "")
	pdf.SetXY(x+30, y+1)
	pdf.CellFormat(62, 3, "INFORMAÇÕES DOS DOCS. FISCAIS VINCULADOS AO MANIFESTO", "", 0, "L", false, 0, "")
	pdf.SetXY(x+92, y+1)
	pdf.CellFormat(33, 3, "MUNICÍPIO", "", 0, "L", false, 0, "")
	pdf.SetXY(x+125, y+1)
	pdf.CellFormat(w-125, 3, "INFORMAÇÕES DOS DOCS. FISCAIS VINCULADOS AO MANIFESTO", "", 0, "L", false, 0, "")
	rowY := y + 4
	rowCount := (len(docs) + 1) / 2
	if rowCount > 5 {
		rowCount = 5
	}
	if rowCount > 0 {
		pdf.Rect(x, rowY, w, float64(rowCount)*4, "")
	}
	for i, doc := range docs {
		if i >= 10 {
			pdf.SetXY(x+1, rowY)
			pdf.CellFormat(w-2, 3, fmt.Sprintf("... mais %d documento(s)", len(docs)-i), "", 1, "L", false, 0, "")
			break
		}
		colX := x
		keyX := x + 30
		if i%2 == 1 {
			colX = x + 92
			keyX = x + 125
		}
		if i > 0 && i%2 == 0 {
			rowY += 4
		}
		pdf.SetXY(colX, rowY)
		pdf.MultiCell(30, 4, doc.Municipality, "", "L", false)
		pdf.SetXY(keyX, rowY)
		pdf.MultiCell(w-keyX+x, 4, doc.Key, "", "L", false)
	}
}

func drawInsurance(pdf *pdfdraw.PDF, x, y, w float64, data mdfeData, config Config) {
	pdf.Rect(x, y, w, 44, "")
	pdf.Line(x, y+4, x+w, y+4)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY((w-45)/2, y+2)
	pdf.MultiCell(100, 0, "INFORMAÇÕES SOBRE OS SEGUROS", "", "L", false)
	pdf.SetFont(string(config.FontType), "", 6)
	yPos := y + 6
	for _, seg := range data.Insurance {
		if yPos > y+30 {
			break
		}
		pdf.SetXY(x+1, yPos)
		pdf.MultiCell(w-2, 3, strings.Join(nonEmpty(
			"NOME: "+seg.Name,
			"CNPJ: "+seg.CNPJ,
			"APÓLICE: "+seg.Policy,
		), "  "), "", "L", false)
		yPos += 4
		for i := 0; i < len(seg.Endorsement) && yPos <= y+30; i += 2 {
			line := []string{"AVERBAÇÃO: " + seg.Endorsement[i]}
			if i+1 < len(seg.Endorsement) {
				line = append(line, "AVERBAÇÃO: "+seg.Endorsement[i+1])
			}
			pdf.SetXY(x+1, yPos)
			pdf.MultiCell(w-2, 3, strings.Join(line, "  "), "", "L", false)
			yPos += 4
		}
	}
	if len(data.CIOT) == 0 {
		return
	}
	ciotY := y + 40
	pdf.Rect(x, ciotY, w, 16, "")
	pdf.Line(x, ciotY+4, x+w, ciotY+4)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY((w-30)/2, ciotY+2)
	pdf.MultiCell(100, 0, "INFORMAÇÕES DO CIOT", "", "L", false)
	pdf.SetFont(string(config.FontType), "", 6)
	rowY := ciotY + 5
	for _, ciot := range data.CIOT {
		pdf.SetXY(x+1, rowY)
		pdf.MultiCell(w-2, 3, strings.TrimSpace(ciot.Label+": "+ciot.Value), "", "L", false)
		rowY += 3
		if rowY > y+47 {
			return
		}
	}
}

func drawBox(pdf *pdfdraw.PDF, x, y, w, h float64, title string, fields []field, config Config) {
	pdf.Rect(x, y, w, h, "")
	if title != "" {
		pdf.SetFont(string(config.FontType), "B", 7)
		pdf.SetXY(x+1, y+1)
		pdf.CellFormat(w-2, 4, title, "", 1, "C", false, 0, "")
	}
	startY := y + 6
	if title == "" {
		startY = y + 1
	}
	colW := w / 4
	for i, field := range fields {
		if field.Label == "" && field.Value == "" {
			continue
		}
		col := float64(i % 4)
		row := float64(i / 4)
		fx := x + 1 + col*colW
		fy := startY + row*9
		if fy > y+h-7 {
			return
		}
		pdf.SetXY(fx, fy)
		pdf.SetFont(string(config.FontType), "B", 5.5)
		pdf.CellFormat(colW-2, 2.5, field.Label, "", 2, "L", false, 0, "")
		pdf.SetFont(string(config.FontType), "", 6.3)
		pdf.MultiCell(colW-2, 3, optional(field.Value), "", "L", false)
	}
}

func drawSupplementaryInfo(pdf *pdfdraw.PDF, x, y, w, h float64, data mdfeData, config Config) {
	if h <= 0 {
		return
	}
	const complementaryH = 45.0
	if h < complementaryH {
		h = complementaryH
	}
	pdf.Rect(x, y, w, complementaryH, "")
	pdf.Line(x, y+4, x+w, y+4)
	pdf.SetFont(string(config.FontType), "B", 7)
	complementaryTitle := "INFORMAÇÕES COMPLEMENTARES DE INTERESSE DO CONTRIBUINTE"
	pdf.SetXY((w-80)/2, y+2)
	pdf.MultiCell(100, 0, complementaryTitle, "", "L", false)

	pdf.SetXY(x, y+5)
	pdf.SetFont(string(config.FontType), "", 6)
	pdf.MultiCell(w, 3, optional(data.ComplementaryInfo), "", "L", false)

	fiscoY := y + complementaryH
	fiscoH := h - complementaryH
	if fiscoH <= 0 {
		return
	}
	pdf.Rect(x, fiscoY, w, fiscoH, "")
	pdf.Line(x, fiscoY+4, x+w, fiscoY+4)
	fiscoTitle := "INFORMAÇÕES ADICIONAIS DE INTERESSE DO FISCO"
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x+(w-pdf.GetStringWidth(pdf.Encode(fiscoTitle)))/2+1.4, fiscoY+2)
	pdf.MultiCell(100, 0, fiscoTitle, "", "L", false)

	cursorY := fiscoY + 5
	pdf.SetFont(string(config.FontType), "", 5.5)
	lineHeight := 3.0
	maxLines := int((fiscoH - 5) / lineHeight)
	fiscoLines := splitSupplementaryLinesToFit(pdf, data.FiscoInfo, w-4, maxLines)
	pdf.SetXY(x+1, cursorY)
	pdf.MultiCell(w-4, lineHeight, strings.Join(fiscoLines, "\n"), "", "L", false)
}

func splitSupplementaryLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitSupplementaryLinesToFit(pdf *pdfdraw.PDF, value string, width float64, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	var lines []string
	for _, line := range splitSupplementaryLines(value) {
		wrapped := pdf.SplitLines([]byte(line), width)
		if len(wrapped) == 0 {
			wrapped = [][]byte{{}}
		}
		for _, part := range wrapped {
			lines = append(lines, strings.TrimSpace(string(part)))
			if len(lines) >= maxLines {
				return lines
			}
		}
	}
	return lines
}

func fitSingleLine(pdf *pdfdraw.PDF, value string, maxWidth float64) string {
	value = strings.TrimSpace(value)
	if value == "" || pdf.GetStringWidth(pdf.Encode(value)) <= maxWidth {
		return value
	}
	const suffix = "..."
	runes := []rune(value)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := strings.TrimSpace(string(runes)) + suffix
		if pdf.GetStringWidth(pdf.Encode(candidate)) <= maxWidth {
			return candidate
		}
	}
	return ""
}

func drawQR(pdf *pdfdraw.PDF, x, y float64, data string) {
	if data == "" {
		return
	}
	pngBytes, err := qrcode.PNGWithBorder(data, 25, 3)
	if err != nil {
		return
	}
	name := "damdfe-qr"
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, 25, 25, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

func drawBarcode(pdf *pdfdraw.PDF, x, y float64, key string) {
	if key == "" {
		return
	}
	bars, svgWidthMM, err := barcode.Code128Bars(key, 0.3, 1, 15)
	if err != nil {
		return
	}
	const svgHeightMM = 23.764
	const targetW = 86.18
	const targetH = 17.0
	pdf.SetFillColor(0, 0, 0)
	for _, bar := range bars {
		pdf.Rect(
			x+bar.X/svgWidthMM*targetW,
			y+bar.Y/svgHeightMM*targetH,
			bar.Width/svgWidthMM*targetW,
			bar.Height/svgHeightMM*targetH,
			"F",
		)
	}
	pdf.SetFillColor(255, 255, 255)
}

func emissionTypeName(code string) string {
	switch code {
	case "2":
		return "CONTINGÊNCIA"
	default:
		return "NORMAL"
	}
}

func issuerTypeName(code string) string {
	switch code {
	case "2":
		return "TRANSPORTADOR DE CARGA PRÓPRIA"
	case "3":
		return "PRESTADOR DE SERVIÇO DE TRANSPORTE QUE EMITIRÁ CT-e GLOBALIZADO"
	default:
		return "PRESTADOR DE SERVIÇO DE TRANSPORTE"
	}
}

func environmentName(code string) string {
	if code == "1" {
		return "PRODUÇÃO"
	}
	return "HOMOLOGAÇÃO"
}

func modalName(code string) string {
	switch code {
	case "2":
		return string(ModalTypeAereo)
	case "3":
		return string(ModalTypeAquaviario)
	case "4":
		return string(ModalTypeFerroviario)
	default:
		return string(ModalTypeRodoviario)
	}
}

func cargoUnit(code string) string {
	switch code {
	case "00":
		return "M3"
	case "01":
		return "KG"
	default:
		return code
	}
}

func contingencyDeadline(date, hour string) string {
	if date == "" || hour == "" {
		return ""
	}
	parsed, err := time.Parse("02/01/2006 15:04:05", date+" "+hour)
	if err != nil {
		return ""
	}
	return parsed.Add(168 * time.Hour).Format("02/01/2006 15:04")
}

func money(value string) string {
	if value == "" {
		value = "0,00"
	}
	return "R$ " + value
}

func optional(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(value))
	}
	return out
}
