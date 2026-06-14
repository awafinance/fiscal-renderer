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
		Environment:           environmentName(tpAmb),
		EnvironmentCode:       tpAmb,
		Protocol:              xmlutil.Text(protocolNode, "nProt"),
		ProtocolDate:          protocolDate,
		ProtocolTime:          protocolTime,
		QRCode:                xmlutil.Text(infSupl, "qrCodMDFe"),
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
	h := 297 - config.Margins.Top - config.Margins.Bottom
	pdf.Rect(x, y, w, h, "")
	drawHeader(pdf, x, y, w, data, config)
	y += 92
	drawModal(pdf, x+2, y, w-4, data, config)
	y += 62
	drawLinkedDocuments(pdf, x+2, y, w-4, 44, data.Documents, config)
	y += 47
	drawInsurance(pdf, x+2, y, w-4, data, config)
	y += 52
	drawSupplementaryInfo(pdf, x+2, y, w-4, 297-config.Margins.Bottom-y, data, config)
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
	pdf.SetFont(string(config.FontType), "B", 8)
	pdf.SetXY(x, y+25)
	pdf.MultiCell(w/2, 3, "DAMDFE - Documento Auxiliar do\nManifesto de Documentos Fiscais Eletrônicos", "", "C", false)

	drawBox(pdf, x+1, y+30, w/2-2, 54, "", []field{
		{"MODELO", data.Model},
		{"SÉRIE", data.Series},
		{"NÚMERO", data.Number},
		{"FL", "1/1"},
		{"DATA E HORA", strings.TrimSpace(data.EmissionDate + " " + data.EmissionTime)},
		{"UF CARREG", data.UFStart},
		{"UF DESCARREG", data.UFEnd},
		{"FORMA DE EMISSÃO", data.EmissionType},
		{"PREVISÃO DE INICIO DA VIAGEM", strings.TrimSpace(data.TripStart + " " + data.TripStartTime)},
		{"", ""},
		{"", ""},
		{"", ""},
		{"TIPO DO EMITENTE", data.IssuerType},
		{"TIPO DO AMBIENTE", data.Environment},
		{"INSC. SUFRAMA", ""},
		{"CARGA POSTERIOR", ""},
	}, config)

	drawQR(pdf, x+w-32, y+3, data.QRCode)
	drawBarcode(pdf, x+w/2+8, y+32, data.Key)
	pdf.SetFont(string(config.FontType), "", 6.5)
	pdf.SetXY(x+w/2+6, y+29)
	pdf.CellFormat(w/2-8, 3, "CONTROLE DO FISCO", "", 1, "L", false, 0, "")
	pdf.SetXY(x+w/2+18, y+54)
	pdf.MultiCell(w/2-20, 3, "Consulta em https://dfe-portal.svrs.rs.gov.br/MDFE/Consulta", "", "L", false)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x+w/2+18, y+61)
	pdf.MultiCell(w/2-20, 3, data.Key, "", "L", false)
	pdf.SetFont(string(config.FontType), "B", 6)
	pdf.SetXY(x+w/2+20, y+69)
	pdf.CellFormat(w/2-20, 3, "PROTOCOLO DE AUTORIZAÇÃO DE USO", "", 1, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", 6)
	pdf.SetXY(x+w/2+16, y+73)
	if data.HasContingency {
		pdf.MultiCell(w/2-22, 3, fmt.Sprintf("EMISSÃO EM CONTINGÊNCIA. Obrigatória a autorização em 168 horas após esta emissão (%s)", data.Deadline), "", "C", false)
	} else {
		pdf.MultiCell(w/2-22, 3, strings.TrimSpace(data.Protocol+" "+data.ProtocolDate+" "+data.ProtocolTime), "", "C", false)
	}
}

func drawModal(pdf *pdfdraw.PDF, x, y, w float64, data mdfeData, config Config) {
	title := fmt.Sprintf("MODAL %s DE CARGA - INFORMAÇÕES PARA ANTT", data.ModalName)
	fields := []field{
		{"QTD. CT-e", data.QuantityCTe},
		{"QTD. NF-e", data.QuantityNFe},
		{"PESO TOTAL", strings.TrimSpace(data.CargoQuantity + " " + cargoUnit(data.CargoUnit))},
		{"VALOR TOTAL", data.CargoValue},
		{"PERCURSO", data.Route},
	}
	switch data.ModalCode {
	case "1":
		fields = append(fields,
			field{"PLACA", data.VehiclePlate},
			field{"UF", data.VehicleUF},
			field{"RNTRC", data.VehicleRNTRC},
			field{"RENAVAM", data.VehicleRENAVAM},
		)
		for _, driver := range data.Drivers {
			fields = append(fields, field{"CONDUTOR", strings.TrimSpace(driver.CPF + " " + driver.Name)})
		}
		fields = append(fields, data.RodoviarioPedagio...)
	case "2":
		fields = append(fields, data.Aereo...)
	case "3":
		fields = append(fields, data.Aquaviario...)
		fields = append(fields, data.CargoComposition...)
	case "4":
		fields = append(fields, data.Ferroviario...)
	}
	drawBox(pdf, x, y, w, 59, title, fields, config)
}

func drawLinkedDocuments(pdf *pdfdraw.PDF, x, y, w, h float64, docs []linkedDocument, config Config) {
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 4, "INFORMAÇÕES DA COMPOSIÇÃO DA CARGA", "", 1, "C", false, 0, "")
	pdf.SetFont(string(config.FontType), "B", 5.5)
	pdf.SetXY(x+1, y+5)
	pdf.CellFormat(w/2-2, 3, "MUNICÍPIO     INFORMAÇÕES DOS DOCS. FISCAIS VINCULADOS AO MANIFESTO", "", 1, "L", false, 0, "")
	pdf.SetXY(x+w/2, y+5)
	pdf.CellFormat(w/2-2, 3, "MUNICÍPIO     INFORMAÇÕES DOS DOCS. FISCAIS VINCULADOS AO MANIFESTO", "", 1, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", 6.3)
	rowY := y + 9
	for i, doc := range docs {
		if i >= 10 {
			pdf.SetXY(x+1, rowY)
			pdf.CellFormat(w-2, 3, fmt.Sprintf("... mais %d documento(s)", len(docs)-i), "", 1, "L", false, 0, "")
			break
		}
		colX := x + 1
		if i%2 == 1 {
			colX = x + w/2
		}
		if i > 0 && i%2 == 0 {
			rowY += 7
		}
		pdf.SetXY(colX, rowY)
		pdf.MultiCell(w/2-2, 3, fmt.Sprintf("%s  %s", doc.Municipality, doc.Key), "", "L", false)
	}
}

func drawInsurance(pdf *pdfdraw.PDF, x, y, w float64, data mdfeData, config Config) {
	pdf.Rect(x, y, w, 49, "")
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 4, "INFORMAÇÕES SOBRE OS SEGUROS", "", 1, "C", false, 0, "")
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
	ciotY := y + 34
	pdf.Line(x, ciotY, x+w, ciotY)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x+1, ciotY+1)
	pdf.CellFormat(w-2, 4, "INFORMAÇÕES DO CIOT", "", 1, "C", false, 0, "")
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
	if h > 34 {
		h = 34
	}
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 4, "INFORMAÇÕES COMPLEMENTARES DE INTERESSE DO CONTRIBUINTE", "", 1, "C", false, 0, "")

	cursorY := y + 5.4
	pdf.SetXY(x+1, cursorY)
	pdf.SetFont(string(config.FontType), "", 5.5)
	pdf.CellFormat(w-2, 2.8, fitSingleLine(pdf, optional(data.ComplementaryInfo), w-2), "", 1, "L", false, 0, "")
	cursorY += 3.6

	pdf.Line(x, cursorY, x+w, cursorY)
	cursorY += 0.9
	pdf.SetFont(string(config.FontType), "B", 5.5)
	pdf.SetXY(x+1, cursorY)
	pdf.CellFormat(w-2, 2.5, "INFORMAÇÕES ADICIONAIS DE INTERESSE DO FISCO", "", 1, "L", false, 0, "")
	cursorY += 2.6

	pdf.SetFont(string(config.FontType), "", 3.8)
	lineHeight := 1.6
	maxY := y + h - 1
	for _, line := range splitSupplementaryLines(data.FiscoInfo) {
		if cursorY+lineHeight > maxY {
			return
		}
		pdf.SetXY(x+1, cursorY)
		pdf.CellFormat(w-2, lineHeight, fitSingleLine(pdf, line, w-2), "", 1, "L", false, 0, "")
		cursorY += lineHeight
	}
}

func splitSupplementaryLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !isEscapedBreakMarker(line) {
			lines = append(lines, line)
		}
	}
	return lines
}

func isEscapedBreakMarker(value string) bool {
	return value == "&#10" || value == "&#13" || value == "&#10;" || value == "&#13;"
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
	pngBytes, err := qrcode.PNG(data, 120)
	if err != nil {
		return
	}
	name := "damdfe-qr"
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, 28, 28, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

func drawBarcode(pdf *pdfdraw.PDF, x, y float64, key string) {
	if key == "" {
		return
	}
	pngBytes, err := barcode.Code128PNG(key, 430, 85)
	if err != nil {
		return
	}
	name := "damdfe-code128-" + key
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, 86, 17, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
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
