package dacte

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
	pdf.SetTitle("DACTE", false)
	pdf.AddPage()
	draw(pdf, data, d.Config)
	if data.NeedsContinuationPage {
		pdf.AddPage()
		drawContinuation(pdf, data, d.Config)
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

type cteData struct {
	Key                   string
	Model                 string
	Series                string
	Number                string
	CFOP                  string
	Nature                string
	EmissionDate          string
	EmissionTime          string
	StartLocation         string
	EndLocation           string
	ModalCode             string
	ModalName             string
	ServiceType           string
	CTeType               string
	EnvironmentCode       string
	Protocol              string
	ProtocolDate          string
	ProtocolTime          string
	QRCode                string
	TomadorType           string
	Emitter               party
	Sender                party
	Recipient             party
	Expeditor             party
	Receiver              party
	Tomador               party
	FreightTotal          string
	Receivable            string
	Components            []field
	ICMS                  []field
	IBSCBS                []field
	Cargo                 []field
	LinkedDocuments       []linkedDocument
	Observations          []string
	ModalSpecific         []field
	WatermarkCancelled    bool
	HasProtocol           bool
	NeedsContinuationPage bool
}

type party struct {
	Name     string
	Doc      string
	IE       string
	Street   string
	Number   string
	District string
	Address  string
	City     string
	UF       string
	CEP      string
	Phone    string
	Country  string
}

type field struct {
	Label string
	Value string
}

type linkedDocument struct {
	Type         string
	Key          string
	SeriesNumber string
}

const (
	firstPageDocumentLimit     = 24
	firstPageDocumentRows      = 12
	observationContinuationLen = 350
)

func parseData(root *xmlutil.Node, config Config) cteData {
	infCTe := root.Find("infCte")
	ide := root.Find("ide")
	emit := root.Find("emit")
	rem := root.Find("rem")
	dest := root.Find("dest")
	exped := root.Find("exped")
	receb := root.Find("receb")
	toma3 := root.Find("toma3")
	toma4 := root.Find("toma4")
	tomadorNode := toma3
	if tomadorNode == nil {
		tomadorNode = toma4
	}
	protocol := root.Find("protCTe")
	infProt := root.Find("infProt")
	if protocol == nil {
		protocol = infProt
	}
	infSupl := root.Find("infCTeSupl")
	vPrest := root.Find("vPrest")
	imp := root.Find("imp")
	infCarga := root.Find("infCarga")
	infDoc := root.Find("infDoc")
	infModal := root.Find("infModal")
	compl := root.Find("compl")

	date, hour := fiscalfmt.DateUTC(xmlutil.Text(ide, "dhEmi"))
	protocolDate, protocolHour := fiscalfmt.DateUTC(xmlutil.Text(protocol, "dhRecbto"))
	tomadorType := tomadorName(xmlutil.Text(tomadorNode, "toma"))

	linkedDocs := parseLinkedDocuments(infDoc)
	obs := parseObservations(compl)
	obsText := observationText(obs)
	data := cteData{
		Key:                   strings.TrimPrefix(infCTe.Attr("Id"), "CTe"),
		Model:                 xmlutil.Text(ide, "mod"),
		Series:                xmlutil.Text(ide, "serie"),
		Number:                xmlutil.Text(ide, "nCT"),
		CFOP:                  xmlutil.Text(ide, "CFOP"),
		Nature:                xmlutil.Text(ide, "natOp"),
		EmissionDate:          date,
		EmissionTime:          hour,
		StartLocation:         location(xmlutil.Text(ide, "xMunIni"), xmlutil.Text(ide, "UFIni")),
		EndLocation:           location(xmlutil.Text(ide, "xMunFim"), xmlutil.Text(ide, "UFFim")),
		ModalCode:             xmlutil.Text(ide, "modal"),
		ModalName:             modalName(xmlutil.Text(ide, "modal")),
		ServiceType:           serviceTypeName(xmlutil.Text(ide, "tpServ")),
		CTeType:               cteTypeName(xmlutil.Text(ide, "tpCTe")),
		EnvironmentCode:       xmlutil.Text(ide, "tpAmb"),
		Protocol:              xmlutil.Text(protocol, "nProt"),
		ProtocolDate:          protocolDate,
		ProtocolTime:          protocolHour,
		QRCode:                xmlutil.Text(infSupl, "qrCodCTe"),
		TomadorType:           tomadorType,
		Emitter:               parseParty(emit),
		Sender:                parseParty(rem),
		Recipient:             parseParty(dest),
		Expeditor:             parseParty(exped),
		Receiver:              parseParty(receb),
		FreightTotal:          money(fiscalfmt.FormatNumber(xmlutil.Text(vPrest, "vTPrest"), 2)),
		Receivable:            money(fiscalfmt.FormatNumber(xmlutil.Text(vPrest, "vRec"), 2)),
		Components:            parseComponents(vPrest),
		ICMS:                  parseICMS(imp),
		IBSCBS:                parseIBSCBS(root.Find("IBSCBS"), config.DisplayIBSCBS),
		Cargo:                 parseCargo(infCarga),
		LinkedDocuments:       linkedDocs,
		Observations:          obs,
		ModalSpecific:         parseModalSpecific(root, infModal, xmlutil.Text(ide, "modal")),
		WatermarkCancelled:    config.WatermarkCancelled,
		HasProtocol:           protocol != nil && xmlutil.Text(protocol, "nProt") != "",
		NeedsContinuationPage: len(linkedDocs) > firstPageDocumentLimit || len([]rune(obsText)) > observationContinuationLen,
	}
	data.Tomador = tomadorParty(tomadorType, toma4, data)
	return data
}

func parseParty(node *xmlutil.Node) party {
	if node == nil {
		return party{}
	}
	street := xmlutil.Text(node, "xLgr")
	number := xmlutil.Text(node, "nro")
	district := xmlutil.Text(node, "xBairro")
	return party{
		Name:     xmlutil.Text(node, "xNome"),
		Doc:      firstNonEmpty(fiscalfmt.FormatCPFCNPJ(xmlutil.Text(node, "CNPJ")), fiscalfmt.FormatCPFCNPJ(xmlutil.Text(node, "CPF"))),
		IE:       xmlutil.Text(node, "IE"),
		Street:   street,
		Number:   number,
		District: district,
		Address:  strings.Join(nonEmpty(street, number, district), ", "),
		City:     xmlutil.Text(node, "xMun"),
		UF:       xmlutil.Text(node, "UF"),
		CEP:      fiscalfmt.FormatCEP(xmlutil.Text(node, "CEP")),
		Phone:    fiscalfmt.FormatPhone(xmlutil.Text(node, "fone")),
		Country:  xmlutil.Text(node, "xPais"),
	}
}

func tomadorParty(kind string, toma4 *xmlutil.Node, data cteData) party {
	switch kind {
	case "REMETENTE":
		return data.Sender
	case "EXPEDIDOR":
		return data.Expeditor
	case "RECEBEDOR":
		return data.Receiver
	case "DESTINATÁRIO":
		return data.Recipient
	case "OUTRO":
		return parseParty(toma4)
	default:
		return data.Sender
	}
}

func parseComponents(vPrest *xmlutil.Node) []field {
	var fields []field
	for _, comp := range vPrest.FindAll("Comp") {
		fields = append(fields, field{
			Label: xmlutil.Text(comp, "xNome"),
			Value: xmlutil.Text(comp, "vComp"),
		})
	}
	return fields
}

func parseICMS(imp *xmlutil.Node) []field {
	cst := firstNonEmpty(xmlutil.Text(imp, "CST"), xmlutil.Text(imp, "CSOSN"))
	return []field{
		{"SITUAÇÃO TRIBUTÁRIA", strings.TrimSpace(cst + " - " + icmsDescription(cst))},
		{"BASE DE CALCULO", fiscalfmt.FormatNumber(xmlutil.Text(imp, "vBC"), 2)},
		{"ALÍQ ICMS", fiscalfmt.FormatNumber(xmlutil.Text(imp, "pICMS"), 2)},
		{"VALOR ICMS", fiscalfmt.FormatNumber(xmlutil.Text(imp, "vICMS"), 2)},
		{"% RED. BC ICMS", fiscalfmt.FormatNumber(xmlutil.Text(imp, "pRedBC"), 2)},
		{"ICMS ST", fiscalfmt.FormatNumber(firstNonEmpty(xmlutil.Text(imp, "vICMSST"), xmlutil.Text(imp, "vICMS")), 2)},
	}
}

func parseIBSCBS(node *xmlutil.Node, enabled bool) []field {
	if !enabled || node == nil {
		return nil
	}
	return []field{
		{"IBS ESTADUAL (%/R$)", fiscalfmt.FormatNumber(xmlutil.Text(node, "pIBSUF"), 2) + " / " + fiscalfmt.FormatNumber(xmlutil.Text(node, "vIBSUF"), 2)},
		{"IBS MUNICIPAL (%/R$)", fiscalfmt.FormatNumber(xmlutil.Text(node, "pIBSMun"), 2) + " / " + fiscalfmt.FormatNumber(xmlutil.Text(node, "vIBSMun"), 2)},
		{"CBS (%/R$)", fiscalfmt.FormatNumber(xmlutil.Text(node, "pCBS"), 2) + " / " + fiscalfmt.FormatNumber(xmlutil.Text(node, "vCBS"), 2)},
	}
}

func parseCargo(infCarga *xmlutil.Node) []field {
	fields := []field{
		{"VALOR TOTAL DA CARGA", money(fiscalfmt.FormatNumber(xmlutil.Text(infCarga, "vCarga"), 2))},
		{"PRODUTO PREDOMINATE", xmlutil.Text(infCarga, "proPred")},
	}
	for _, infQ := range infCarga.FindAll("infQ") {
		unit := unitName(xmlutil.Text(infQ, "cUnid"))
		quantity := xmlutil.Text(infQ, "qCarga")
		measure := xmlutil.Text(infQ, "tpMed")
		fields = append(fields,
			field{Label: "TIPO MEDIDA", Value: measure},
			field{Label: "QTD/UN.", Value: strings.Join(nonEmpty(quantity, unit), " ")},
		)
		if strings.EqualFold(measure, "M3") {
			fields = append(fields, field{Label: "CUBAGEM (M³)", Value: strings.Join(nonEmpty(quantity, unit), " ")})
		}
		if xmlutil.Text(infQ, "cUnid") == "03" && !strings.EqualFold(strings.TrimSpace(measure), "PARES") {
			fields = append(fields, field{Label: "QTD DE VOLUMES", Value: strings.Join(nonEmpty(quantity, unit), " ")})
		}
	}
	if !hasField(fields, "CUBAGEM (M³)") {
		fields = append(fields, field{Label: "CUBAGEM (M³)", Value: ""})
	}
	if !hasField(fields, "QTD DE VOLUMES") {
		fields = append(fields, field{Label: "QTD DE VOLUMES", Value: ""})
	}
	return fields
}

func parseLinkedDocuments(infDoc *xmlutil.Node) []linkedDocument {
	var docs []linkedDocument
	for _, node := range infDoc.FindAll("infNFe") {
		if key := firstNonEmpty(xmlutil.Text(node, "chave"), xmlutil.Text(node, "chNFe")); key != "" {
			docs = append(docs, linkedDocument{Type: "NFE", Key: key, SeriesNumber: documentSeriesNumber(key)})
		}
	}
	for _, node := range infDoc.FindAll("infCTe") {
		if key := firstNonEmpty(xmlutil.Text(node, "chave"), xmlutil.Text(node, "chCTe")); key != "" {
			docs = append(docs, linkedDocument{Type: "CTE", Key: key, SeriesNumber: documentSeriesNumber(key)})
		}
	}
	for _, node := range infDoc.FindAll("infOutros") {
		if key := firstNonEmpty(xmlutil.Text(node, "chave"), xmlutil.Text(node, "nDoc")); key != "" {
			docs = append(docs, linkedDocument{Type: "DOC", Key: key, SeriesNumber: documentSeriesNumber(key)})
		}
	}
	return docs
}

func parseObservations(compl *xmlutil.Node) []string {
	var obs []string
	if compl == nil {
		return obs
	}
	for _, node := range compl.Children {
		text := strings.Join(strings.Fields(xmlutil.Text(node, "xTexto")), " ")
		obs = append(obs, text)
	}
	return obs
}

func parseModalSpecific(root, infModal *xmlutil.Node, code string) []field {
	switch code {
	case "02":
		aereo := firstNonNil(root.Find("aereo"), infModal)
		return []field{
			{"NÚMERO OPERACIONAL AÉREO", xmlutil.Text(aereo, "nOCA")},
			{"CLASSE", xmlutil.Text(aereo, "CL")},
			{"CÓDIGO DA TARIFA", xmlutil.Text(aereo, "cTar")},
			{"VALOR DA TARIFA", money(fiscalfmt.FormatNumber(xmlutil.Text(aereo, "vTar"), 2))},
			{"NÚMERO DA MINUTA", xmlutil.Text(aereo, "nMinu")},
			{"RETIRA", ""},
			{"DADOS RELATIVOS A RETIRADA DA CARGA", ""},
			{"CARACTERÍSTICAS ADICIONAL DO SERVIÇO", ""},
			{"DATA PREVISTA DA ENTREGA", xmlutil.Text(aereo, "dPrevAereo")},
			{"INFORMAÇÕES DE MANUSEIO", handlingInfo(xmlutil.Text(aereo, "cInfManu"))},
			{"DIMENSÃO", fiscalfmt.FormatXDime(xmlutil.Text(aereo, "xDime"))},
		}
	case "03":
		aquav := firstNonNil(root.Find("aquav"), infModal)
		return []field{
			{"LACRE", xmlutil.Text(aquav, "nLacre")},
			{"IDENTIFICAÇÃO DO CONTAINER", xmlutil.Text(aquav, "nCont")},
			{"IDENTIFICAÇÃO DO NAVIO / REBOCADOR", xmlutil.Text(aquav, "xNavio")},
			{"IDENTIFICAÇÃO DA BALSA", strings.Join(balsaNames(aquav), " ")},
			{"VLR DO AFRMM", money(fiscalfmt.FormatNumber(xmlutil.Text(aquav, "vAFRMM"), 2))},
		}
	case "04":
		ferrov := firstNonNil(root.Find("ferrov"), infModal)
		return []field{
			{"TIPO DE TRÁFICO", trafficType(xmlutil.Text(ferrov, "tpTraf"))},
			{"FLUXO FERROVIÁRIO", xmlutil.Text(ferrov, "fluxo")},
			{"VALOR DO FRETE", money(fiscalfmt.FormatNumber(xmlutil.Text(ferrov, "vFrete"), 2))},
			{"FERROVIA EMITENTE DO CT-E", railwayRole(xmlutil.Text(ferrov, "ferrEmi"))},
			{"FERROVIA DO FATURAMENTO", railwayRole(xmlutil.Text(ferrov, "respFat"))},
			{"INFORMAÇÕES DAS FERROVIARIAS ENVOLVIDAS", ""},
			{"CNPJ", fiscalfmt.FormatCPFCNPJ(xmlutil.Text(ferrov, "CNPJ"))},
			{"COD. INTERNO", xmlutil.Text(ferrov, "cInt")},
			{"IE", xmlutil.Text(ferrov, "IE")},
			{"RAZÃO SOCIAL", xmlutil.Text(ferrov, "xNome")},
		}
	case "05":
		duto := firstNonNil(root.Find("duto"), infModal)
		return []field{
			{"VALOR UNITÁRIO", ""},
			{"VALOR DO FRETE", ""},
			{"OUTROS", xmlutil.Text(duto, "xDuto")},
			{"BASE DE CÁLCULO", ""},
			{"ALÍQUOTA", ""},
			{"VALOR DO IMPOSTO", ""},
			{"VALOR TOTAL DO FRETE", money(fiscalfmt.FormatNumber(xmlutil.Text(duto, "vTar"), 2))},
			{"OBSERVAÇÕES", ""},
			{"SÉRIE", ""},
			{"NÚMERO", ""},
			{"EMITENTE", ""},
		}
	case "06":
		multimodal := firstNonNil(root.Find("multimodal"), infModal)
		return []field{
			{"Nº DO CERTIFICADO DO OPERADOR DE TRANSPORTE MULTIMODAL", xmlutil.Text(multimodal, "COTM")},
			{"INDICADOR NEGOCIÁVEL", ""},
			{"NEGOCIÁVEL", ""},
			{"NÃO NEGOCIÁVEL", ""},
			{"CNPJ DA SEGURADO", xmlutil.Text(infModal, "CNPJ")},
			{"NOME DA SEGURADO", xmlutil.Text(infModal, "xSeg")},
			{"NÚMERO DA APÓLICE", xmlutil.Text(infModal, "nApol")},
			{"NÚMERO DE AVERBAÇÃO", xmlutil.Text(infModal, "nAver")},
		}
	default:
		return []field{{"RNTRC", xmlutil.Text(infModal, "RNTRC")}}
	}
}

func draw(pdf *pdfdraw.PDF, data cteData, config Config) {
	drawWatermark(pdf, data, config)
	x := config.Margins.Left
	y := config.Margins.Top
	w := 210 - config.Margins.Left - config.Margins.Right
	h := 297 - config.Margins.Top - config.Margins.Bottom
	pdf.Rect(x, y, w, h, "")
	drawReceipt(pdf, x, y, w, data, config)
	y += 24
	drawHeader(pdf, x, y, w, data, config)
	y += 73
	drawParties(pdf, x+2, y, w-4, data, config)
	y += 51
	drawTomadorCargo(pdf, x+2, y, w-4, data, config)
	y += 36
	drawValuesTaxes(pdf, x+2, y, w-4, data, config)
	y += 39
	y += drawDocumentsObservations(pdf, x+2, y, w-4, data, config)
	drawModalSpecific(pdf, x+2, y, w-4, data, config)
}

func drawContinuation(pdf *pdfdraw.PDF, data cteData, config Config) {
	drawWatermark(pdf, data, config)
	x := config.Margins.Left
	y := config.Margins.Top
	w := 210 - config.Margins.Left - config.Margins.Right
	h := 297 - config.Margins.Top - config.Margins.Bottom
	pdf.Rect(x, y, w, h, "")
	drawReceipt(pdf, x, y, w, data, config)
	y += 24
	drawHeader(pdf, x, y, w, data, config)
	y += 73
	_, remainingObservationText := splitObservationText(observationText(data.Observations))
	remainingDocs := limitDocuments(data.LinkedDocuments, firstPageDocumentLimit, len(data.LinkedDocuments))
	if len(remainingDocs) > 0 {
		rowsPerColumn := (len(remainingDocs) + 1) / 2
		if rowsPerColumn < 1 {
			rowsPerColumn = 1
		}
		docHeight := 8.0 + float64(rowsPerColumn)*4
		y += drawOriginDocuments(pdf, x+2, y, w-4, remainingDocs, rowsPerColumn, 4, docHeight, config)
	}
	if remainingObservationText != "" {
		if len(remainingDocs) > 0 {
			y += 3
		}
		drawObservations(pdf, x+2, y, w-4, remainingObservationText, config)
	}
}

func drawWatermark(pdf *pdfdraw.PDF, data cteData, config Config) {
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
	xCenter := (210 - width) / 2
	yCenter := (297 + height) / 2
	pdf.TransformBegin()
	pdf.TransformRotate(55, xCenter+(width/2), yCenter-(height/2))
	pdf.Text(xCenter, yCenter, text)
	pdf.TransformEnd()
	pdf.SetTextColor(0, 0, 0)
}

func drawReceipt(pdf *pdfdraw.PDF, x, y, w float64, data cteData, config Config) {
	pdf.Rect(x, y, w, 20, "")
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.SetXY(x+1, y+1)
	pdf.MultiCell(w-2, 3, "DECLARO QUE RECEBI OS VOLUMES DESTE CONHECIMENTO EM PERFEITO ESTADO PELO QUE DOU POR CUMPRIDO O PRESENTE CONTRATO DE TRANSPORTE", "", "L", false)
	pdf.Line(x, y+11, x+w, y+11)
	pdf.Line(x+w/4, y+11, x+w/4, y+20)
	pdf.Line(x+w/2, y+11, x+w/2, y+20)
	pdf.Line(x+3*w/4, y+11, x+3*w/4, y+20)

	colW := w / 4
	labelY := y + 13
	valueY := y + 17
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.Text(x+2, labelY, "NOME")
	pdf.SetFont(string(config.FontType), "B", 10)
	pdf.Text(x+3*colW+23, y+12, "CT-E")
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.Text(x+2*colW+8, labelY, "CHEGADA DATA/HORA")
	pdf.Text(x+2, valueY, "RG")
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.Text(x+3*colW+5, labelY, "NRO. DOCUMENTO")
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.Text(x+2*colW+8, valueY, "SAÍDA DATA/HORA")
	pdf.Text(x+3*colW+35, labelY, data.Number)
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.Text(x+3*colW+5, valueY, "SÉRIE")
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.Text(x+3*colW+38, valueY, data.Series)
	pdf.Text(x+colW+8, valueY, "ASSINATURA / CARIMBO")
}

func drawHeader(pdf *pdfdraw.PDF, x, y, w float64, data cteData, config Config) {
	const headerHeight = 70.0
	pdf.Rect(x, y, w, headerHeight, "")
	if len(config.LogoBytes) > 0 {
		pdf.ImageBytes("dacte-logo", config.LogoBytes, x+2, y+2, 22, 0)
	} else if config.Logo != "" {
		imageType, _ := images.TypeFromFile(config.Logo)
		pdf.ImageOptions(config.Logo, x+2, y+2, 22, 0, false, fpdf.ImageOptions{ImageType: imageType}, 0, "")
	}
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.SetXY(x+26, y+3)
	pdf.MultiCell(65, 3, strings.Join(nonEmpty(
		data.Emitter.Name,
		"CNPJ: "+data.Emitter.Doc+" IE: "+data.Emitter.IE,
		strings.Join(nonEmpty(data.Emitter.Street, data.Emitter.Number), ", "),
		data.Emitter.District,
		strings.TrimSpace(data.Emitter.City+" - "+data.Emitter.UF),
		data.Emitter.CEP,
		"Fone: "+data.Emitter.Phone,
	), "\n"), "", "L", false)
	pdf.Line(x+w/2, y, x+w/2, y+headerHeight)
	pdf.SetFont(string(config.FontType), "B", 10)
	pdf.SetXY(x, y+24)
	pdf.MultiCell(w/2, 4, "DACTE\nDOCUMENTO AUXILIAR DO CONHECIMENTO DE TRANSPORTE ELETRÔNICO", "", "C", false)
	drawDenseBox(pdf, x+1, y+37, w/2-2, 31, []field{
		{"MODELO", data.Model},
		{"SÉRIE", data.Series},
		{"NÚMERO", data.Number},
		{"DATA E HORA DE EMISSÃO", strings.TrimSpace(data.EmissionDate + " " + data.EmissionTime)},
		{"FL", pageLabel(pdf, data)},
		{"MODAL", data.ModalName},
		{"TIPO DO CT-E", data.CTeType},
		{"TIPO DO SERVIÇO", data.ServiceType},
		{"TOMADOR DO SERVIÇO", data.TomadorType},
		{"CFOP - NATUREZA DA PRESTAÇÃO", strings.TrimSpace(data.CFOP + " - " + data.Nature)},
	}, config)
	drawQR(pdf, x+w-32, y+2, data.QRCode)
	drawBarcode(pdf, x+w/2+8, y+15, data.Key)
	pdf.SetFont(string(config.FontType), "", 6.5)
	pdf.SetXY(x+w/2+18, y+31)
	pdf.MultiCell(w/2-20, 3, "CHAVE DE ACESSO", "", "L", false)
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x+w/2+18, y+35)
	pdf.MultiCell(w/2-20, 3, data.Key, "", "L", false)
	pdf.SetFont(string(config.FontType), "", 6.5)
	pdf.SetXY(x+w/2+18, y+43)
	pdf.MultiCell(w/2-20, 3, "CONSULTA EM http://www.cte.fazenda.gov.br", "", "L", false)
	pdf.SetXY(x+w/2+18, y+51)
	pdf.MultiCell(w/2-20, 3, "PROTOCOLO DE AUTORIZAÇÃO DE USO", "", "L", false)
	pdf.SetFont(string(config.FontType), "B", 6.5)
	pdf.SetXY(x+w/2+18, y+55)
	pdf.MultiCell(w/2-20, 3, strings.TrimSpace(data.Protocol+" "+data.ProtocolDate+" "+data.ProtocolTime), "", "L", false)
	pdf.SetFont(string(config.FontType), "", 6.5)
	pdf.SetXY(x+w/2+18, y+62)
	pdf.MultiCell(w/2-20, 3, "INÍCIO DA PRESTAÇÃO "+data.StartLocation, "", "L", false)
	pdf.SetXY(x+w/2+18, y+66)
	pdf.MultiCell(w/2-20, 3, "TÉRMINO DA PRESTAÇÃO "+data.EndLocation, "", "L", false)
}

func drawParties(pdf *pdfdraw.PDF, x, y, w float64, data cteData, config Config) {
	blockW := w / 2
	blockH := 24.5
	drawPartyBlock(pdf, x, y, blockW, blockH, "REMETENTE", data.Sender, config)
	drawPartyBlock(pdf, x+blockW, y, blockW, blockH, "DESTINATÁRIO", data.Recipient, config)
	drawPartyBlock(pdf, x, y+blockH, blockW, blockH, "EXPEDIDOR", data.Expeditor, config)
	drawPartyBlock(pdf, x+blockW, y+blockH, blockW, blockH, "RECEBEDOR", data.Receiver, config)
}

func drawPartyBlock(pdf *pdfdraw.PDF, x, y, w, h float64, title string, p party, config Config) {
	pdf.Rect(x, y, w, h, "")
	rows := []field{
		{title, p.Name},
		{"ENDEREÇO", partyAddress(p)},
		{"MUNICÍPIO", location(p.City, p.UF)},
		{"CNPJ/CPF", p.Doc},
		{"PAÍS", p.Country},
		{"CEP", p.CEP},
		{"IE", p.IE},
		{"FONE", p.Phone},
	}
	drawLabelValueRows(pdf, x+1, y+1, w-2, rows, 4.7, 2.8, 15, config)
}

func drawTomadorCargo(pdf *pdfdraw.PDF, x, y, w float64, data cteData, config Config) {
	pdf.Rect(x, y, w, 34, "")
	tomadorFields := []field{
		{"TOMADOR DO SERVIÇO", data.Tomador.Name},
		{"ENDEREÇO", tomadorAddress(data.Tomador)},
		{"MUNICÍPIO", data.Tomador.City},
		{"UF", data.Tomador.UF},
		{"CEP", data.Tomador.CEP},
		{"CNPJ/CPF", data.Tomador.Doc},
		{"IE", data.Tomador.IE},
		{"PAÍS", data.Tomador.Country},
		{"FONE", data.Tomador.Phone},
	}
	drawLabelValueRows(pdf, x+1, y+1, w-2, tomadorFields, 5, 3, 28, config)
	drawFieldGrid(pdf, x+1, y+12, w-2, 20, data.Cargo, 5, 3, 5, config)
}

func drawValuesTaxes(pdf *pdfdraw.PDF, x, y, w float64, data cteData, config Config) {
	titleH := 4.0
	drawSectionTitle(pdf, x, y, w, titleH, "COMPONENTES DO VALOR DA PRESTAÇÃO DO SERVIÇO", config)
	componentFields := []field{{"NOME", "VALOR"}}
	componentFields = append(componentFields, data.Components...)
	componentFields = append(componentFields,
		field{"VALOR TOTAL DO SERVIÇO", data.FreightTotal},
		field{"VALOR TOTAL A RECEBER", data.Receivable},
	)
	drawFieldGrid(pdf, x, y+titleH, w, 15, componentFields, 5.2, 2.8, 4, config)

	taxY := y + titleH + 15
	drawSectionTitle(pdf, x, taxY, w, titleH, "INFORMAÇÕES RELATIVAS AO IMPOSTO", config)
	taxFields := append([]field{}, data.ICMS...)
	taxFields = append(taxFields, data.IBSCBS...)
	drawFieldGrid(pdf, x, taxY+titleH, w, 15, taxFields, 5.1, 2.8, 7, config)
}

func drawDocumentsObservations(pdf *pdfdraw.PDF, x, y, w float64, data cteData, config Config) float64 {
	firstObservationText, _ := splitObservationText(observationText(data.Observations))
	height := drawOriginDocuments(pdf, x, y, w, limitDocuments(data.LinkedDocuments, 0, firstPageDocumentLimit), firstPageDocumentRows, 1.75, 25, config)
	height += drawObservations(pdf, x, y+height, w, firstObservationText, config)
	return height
}

func drawOriginDocuments(pdf *pdfdraw.PDF, x, y, w float64, docs []linkedDocument, rowsPerColumn int, rowH, blockH float64, config Config) float64 {
	titleH := 4.0
	drawSectionTitle(pdf, x, y, w, titleH, "DOCUMENTOS ORIGINÁRIOS", config)
	docY := y + titleH
	pdf.Rect(x, docY, w, blockH, "")
	half := w / 2
	leftEnd := rowsPerColumn
	if leftEnd > len(docs) {
		leftEnd = len(docs)
	}
	rightEnd := rowsPerColumn * 2
	if rightEnd > len(docs) {
		rightEnd = len(docs)
	}
	drawDocumentColumn(pdf, x+1, docY+1, half-2, docs[:leftEnd], rowH, config)
	if leftEnd < len(docs) {
		drawDocumentColumn(pdf, x+half+1, docY+1, half-2, docs[leftEnd:rightEnd], rowH, config)
	} else {
		drawDocumentColumn(pdf, x+half+1, docY+1, half-2, nil, rowH, config)
	}
	return titleH + blockH
}

func drawObservations(pdf *pdfdraw.PDF, x, y, w float64, text string, config Config) float64 {
	titleH := 4.0
	drawSectionTitle(pdf, x, y, w, titleH, "OBSERVAÇÕES", config)
	pdf.Rect(x, y+titleH, w, 11, "")
	pdf.SetFont(string(config.FontType), "", 5.8)
	pdf.SetXY(x+1, y+titleH+1)
	pdf.MultiCell(w-2, 2.7, text, "", "L", false)
	return titleH + 11
}

func drawModalSpecific(pdf *pdfdraw.PDF, x, y, w float64, data cteData, config Config) {
	title := modalSpecificTitle(data.ModalCode, data.ModalName)
	fields := data.ModalSpecific
	if data.ModalCode == "01" || data.ModalCode == "" {
		title = "DADOS ESPECÍFICOS DO MODAL RODOVIÁRIO - CARGA FRACIONADA"
		fields = []field{
			{"RNTRC DA EMPRESA", modalSpecificValue(data.ModalSpecific, "RNTRC")},
			{"CIOT", ""},
			{"DATA PREVISTA DE ENTREGA", ""},
		}
	}
	drawSectionTitle(pdf, x, y, w, 4, title, config)
	drawFieldGrid(pdf, x, y+4, w, 10, fields, 5, 2.8, 4, config)
	if data.ModalCode == "01" || data.ModalCode == "" {
		pdf.SetFont(string(config.FontType), "", 5)
		pdf.SetXY(x+w*0.75+1, y+5)
		pdf.MultiCell(w*0.24, 2.4, "ESTE CONHECIMENTO DE TRANSPORTE ATENDEÀ LEGISLAÇÃO DE TRANSPORTE RODOVIÁRIO EM VIGOR", "", "L", false)
	}
	drawSectionTitle(pdf, x, y+14, w, 4, "USO EXCLUSIVO DO EMISSOR DO CT-E", config)
	pdf.Rect(x, y+18, w, 6, "")
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
	colW := w / 3
	for i, field := range fields {
		col := float64(i % 3)
		row := float64(i / 3)
		fx := x + 1 + col*colW
		fy := startY + row*10
		if fy > y+h-8 {
			return
		}
		pdf.SetXY(fx, fy)
		pdf.SetFont(string(config.FontType), "B", 5.5)
		pdf.CellFormat(colW-2, 2.5, field.Label, "", 2, "L", false, 0, "")
		pdf.SetFont(string(config.FontType), "", 6.3)
		pdf.MultiCell(colW-2, 3, optional(field.Value), "", "L", false)
	}
}

func drawDenseBox(pdf *pdfdraw.PDF, x, y, w, h float64, fields []field, config Config) {
	pdf.Rect(x, y, w, h, "")
	colW := w / 3
	rowH := 7.5
	for i, field := range fields {
		col := float64(i % 3)
		row := float64(i / 3)
		fx := x + 1 + col*colW
		fy := y + 1 + row*rowH
		if fy > y+h-5 {
			return
		}
		pdf.SetXY(fx, fy)
		pdf.SetFont(string(config.FontType), "B", 4.8)
		pdf.CellFormat(colW-2, 2.2, field.Label, "", 2, "L", false, 0, "")
		pdf.SetFont(string(config.FontType), "", 5.8)
		pdf.MultiCell(colW-2, 2.6, optional(field.Value), "", "L", false)
	}
}

func drawSectionTitle(pdf *pdfdraw.PDF, x, y, w, h float64, title string, config Config) {
	pdf.Rect(x, y, w, h, "")
	pdf.SetFont(string(config.FontType), "B", 5.8)
	pdf.SetXY(x+1, y+0.8)
	pdf.CellFormat(w-2, h-1, title, "", 1, "C", false, 0, "")
}

func drawLabelValueRows(pdf *pdfdraw.PDF, x, y, w float64, rows []field, fontSize, rowH, valueX float64, config Config) {
	for i, row := range rows {
		fy := y + float64(i)*rowH
		pdf.SetFont(string(config.FontType), "", fontSize)
		pdf.SetXY(x, fy)
		pdf.CellFormat(valueX-1, rowH, row.Label, "", 0, "L", false, 0, "")
		pdf.SetFont(string(config.FontType), "B", fontSize)
		pdf.SetXY(x+valueX, fy)
		pdf.CellFormat(w-valueX, rowH, row.Value, "", 0, "L", false, 0, "")
	}
}

func drawFieldGrid(pdf *pdfdraw.PDF, x, y, w, h float64, fields []field, fontSize, rowH float64, cols int, config Config) {
	if cols <= 0 {
		return
	}
	pdf.Rect(x, y, w, h, "")
	colW := w / float64(cols)
	for i, field := range fields {
		col := i % cols
		row := i / cols
		fx := x + 1 + float64(col)*colW
		fy := y + 1 + float64(row)*rowH
		if fy > y+h-rowH {
			return
		}
		pdf.SetFont(string(config.FontType), "", fontSize)
		pdf.SetXY(fx, fy)
		pdf.CellFormat(colW-2, rowH, field.Label, "", 2, "L", false, 0, "")
		if field.Value != "" {
			pdf.SetFont(string(config.FontType), "B", fontSize)
			pdf.SetXY(fx, fy+rowH)
			pdf.CellFormat(colW-2, rowH, field.Value, "", 0, "L", false, 0, "")
		}
	}
}

func drawDocumentColumn(pdf *pdfdraw.PDF, x, y, w float64, docs []linkedDocument, rowH float64, config Config) {
	colW := w / 3
	headers := []string{"TIPO DOC", "CNPJ/CHAVE", "SÉRIE/NRO. DOCUMENTO"}
	pdf.SetFont(string(config.FontType), "", 4.8)
	for i, header := range headers {
		pdf.SetXY(x+float64(i)*colW, y)
		pdf.CellFormat(colW-1, 2.5, header, "", 0, "L", false, 0, "")
	}
	pdf.SetFont(string(config.FontType), "B", 4.8)
	for i, doc := range docs {
		fy := y + 3 + float64(i)*rowH
		pdf.SetXY(x, fy)
		pdf.CellFormat(colW-1, rowH, doc.Type, "", 0, "L", false, 0, "")
		pdf.SetXY(x+colW, fy)
		pdf.CellFormat(colW-1, rowH, doc.Key, "", 0, "L", false, 0, "")
		pdf.SetXY(x+2*colW, fy)
		pdf.CellFormat(colW-1, rowH, doc.SeriesNumber, "", 0, "L", false, 0, "")
	}
}

func drawQR(pdf *pdfdraw.PDF, x, y float64, data string) {
	if data == "" {
		return
	}
	pngBytes, err := qrcode.PNG(data, 120)
	if err != nil {
		return
	}
	name := "dacte-qr"
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, 28, 28, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

func drawBarcode(pdf *pdfdraw.PDF, x, y float64, key string) {
	if key == "" {
		return
	}
	pngBytes, err := barcode.Code128PNG(key, 430, 70)
	if err != nil {
		return
	}
	name := "dacte-code128-" + key
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, 86, 14, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

func modalName(code string) string {
	switch code {
	case "02":
		return string(ModalTypeAereo)
	case "03":
		return string(ModalTypeAquaviario)
	case "04":
		return string(ModalTypeFerroviario)
	case "05":
		return string(ModalTypeDutoviario)
	case "06":
		return string(ModalTypeMultimodal)
	default:
		return string(ModalTypeRodoviario)
	}
}

func modalSpecificTitle(code, name string) string {
	switch code {
	case "03":
		return "INFORMAÇÕES ESPECÍFICAS DO MODAL AQUAVIÁRIO"
	case "04":
		return "INFORMAÇÕES ESPECÍFICAS DO MODAL FERROVIÁRIO"
	case "06":
		return "INFORMAÇÕES E ESPECIFICAÇÕES DO TRANSPORTE MULTIMODAL DE CAMADAS"
	default:
		return "DADOS ESPECÍFICOS DO MODAL " + name
	}
}

func firstNonNil(nodes ...*xmlutil.Node) *xmlutil.Node {
	for _, node := range nodes {
		if node != nil {
			return node
		}
	}
	return nil
}

func handlingInfo(code string) string {
	switch code {
	case "01":
		return "Certificado do expedidor para embarque de animal vivo"
	case "02":
		return "Artigo perigoso conforme Declaração do Expedidor anexa"
	case "03":
		return "Somente em aeronave cargueira"
	case "04":
		return "Artigo perigoso - declaração do expedidor não requerida"
	case "05":
		return "Artigo perigoso em quantidade isenta"
	case "06":
		return "Gelo seco para refrigeração (especificar no campo observações a quantidade)"
	case "07":
		return "Não restrito (especificar a Disposição Especial no campo observações)"
	case "08":
		return "Artigo perigoso em carga consolidada (especificar a quantidade no campo observações)"
	case "09":
		return "Autorização da autoridade governamental anexa (especificar no campo observações)"
	case "10":
		return "Baterias de íons de lítio em conformidade com a Seção II da PI965 - CAO"
	case "11":
		return "Baterias de íons de lítio em conformidade com a Seção II da PI966"
	case "12":
		return "Baterias de íons de lítio em conformidade com a Seção II da PI967"
	case "13":
		return "Baterias de metal lítio em conformidade com a Seção II da PI968 - CAO"
	case "14":
		return "Baterias de metal lítio em conformidade com a Seção II da PI969"
	case "15":
		return "Baterias de metal lítio em conformidade com a Seção II da PI970"
	case "99":
		return "Outro (especificar no campo observações)"
	default:
		return ""
	}
}

func balsaNames(aquav *xmlutil.Node) []string {
	var values []string
	for _, balsa := range aquav.FindAll("balsa") {
		if name := xmlutil.Text(balsa, "xBalsa"); name != "" {
			values = append(values, name)
		}
	}
	return values
}

func trafficType(code string) string {
	switch code {
	case "0":
		return "PRÓPRIO"
	case "1":
		return "MÚTUO"
	case "2":
		return "RODOFERROVIÁRIO"
	case "3":
		return "RODOVIÁRIO"
	default:
		return ""
	}
}

func railwayRole(code string) string {
	switch code {
	case "1":
		return "FERROVIA DE ORIGEM"
	case "2":
		return "FERROVIA DE DESTINO"
	default:
		return ""
	}
}

func icmsDescription(code string) string {
	switch code {
	case "00":
		return "TRIBUTAÇÃO NORMAL"
	case "20":
		return "COM REDUÇÃO DA BC"
	case "40":
		return "ISENTA"
	case "41":
		return "NÃO TRIBUTADA"
	case "51":
		return "DIFERIMENTO"
	case "60":
		return "ICMS COBRADO ANTERIORMENTE POR SUBSTITUIÇÃO TRIBUTÁRIA"
	case "90":
		return "OUTROS"
	default:
		return ""
	}
}

func cteTypeName(code string) string {
	switch code {
	case "1":
		return "COMPLEMENTAR"
	case "2":
		return "ANULADO"
	case "3":
		return "SUBSTITUTO"
	default:
		return "NORMAL"
	}
}

func serviceTypeName(code string) string {
	switch code {
	case "1":
		return "SUBCONTRATAÇÃO"
	case "2":
		return "REDESPACHO"
	case "3":
		return "REDESPACHO INTERMEDIÁRIO"
	case "4":
		return "MULTIMODAL"
	default:
		return "NORMAL"
	}
}

func tomadorName(code string) string {
	switch code {
	case "1":
		return "EXPEDIDOR"
	case "2":
		return "RECEBEDOR"
	case "3":
		return "DESTINATÁRIO"
	case "4":
		return "OUTRO"
	default:
		return "REMETENTE"
	}
}

func unitName(code string) string {
	switch code {
	case "00":
		return "M3"
	case "01":
		return "KG"
	case "02":
		return "TON"
	case "03":
		return "UN"
	case "04":
		return "LT"
	default:
		return code
	}
}

func location(city, uf string) string {
	return strings.Join(nonEmpty(city, uf), " - ")
}

func pageLabel(pdf *pdfdraw.PDF, data cteData) string {
	if data.NeedsContinuationPage {
		return fmt.Sprintf("%d/2", pdf.PageNo())
	}
	return "1/1"
}

func partyAddress(p party) string {
	return strings.Join(nonEmpty(p.Street, p.District, p.Number), ", ")
}

func tomadorAddress(p party) string {
	return strings.Join(nonEmpty(p.Street, p.Number, p.District), " ")
}

func documentSeriesNumber(key string) string {
	if len(key) < 34 {
		return ""
	}
	return key[22:25] + "/" + key[25:34]
}

func limitDocuments(values []linkedDocument, start, end int) []linkedDocument {
	if start >= len(values) {
		return nil
	}
	if end > len(values) {
		end = len(values)
	}
	return values[start:end]
}

func modalSpecificValue(fields []field, label string) string {
	for _, field := range fields {
		if field.Label == label {
			return field.Value
		}
	}
	return ""
}

func hasField(fields []field, label string) bool {
	for _, field := range fields {
		if field.Label == label {
			return true
		}
	}
	return false
}

func observationText(obs []string) string {
	return strings.Join(obs, " ")
}

func splitObservationText(text string) (string, string) {
	runes := []rune(text)
	if len(runes) <= observationContinuationLen {
		return text, ""
	}
	return string(runes[:observationContinuationLen]), string(runes[observationContinuationLen:])
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
	return fiscalfmt.FormatNumber(value, 2) + "%"
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
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}
