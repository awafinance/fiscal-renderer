package danfse

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/awafinance/fiscal-renderer/internal/fiscalfmt"
	"github.com/awafinance/fiscal-renderer/internal/footer"
	"github.com/awafinance/fiscal-renderer/internal/pdfdraw"
	"github.com/awafinance/fiscal-renderer/internal/qrcode"
	"github.com/awafinance/fiscal-renderer/internal/xmlutil"
	"github.com/go-pdf/fpdf"
)

//go:embed nfse_logo.png
var nfseLogoPNG []byte

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
	pdf.SetAutoPageBreak(false, bottomMargin(d.Config))
	pdf.SetTitle("DANFSE", false)
	pdf.AddPage()
	draw(pdf, data, d.Config)
	drawFooterStamp(pdf, d.Config)
	return pdf.Output(w)
}

func RenderFile(xml string, path string, config *Config) error {
	doc, err := New(xml, config)
	if err != nil {
		return err
	}
	return doc.Output(path)
}

func toPDFMargins(m Margins) pdfdraw.Margins {
	return pdfdraw.Margins{Top: m.Top, Right: m.Right, Bottom: m.Bottom, Left: m.Left}
}

type nfseData struct {
	Environment       string
	Key               string
	Number            string
	Competence        string
	ProcessedDate     string
	ProcessedTime     string
	DPSNumber         string
	DPSSeries         string
	DPSEmissionDate   string
	DPSEmissionTime   string
	Issuer            party
	Taker             party
	ServiceCode       string
	ServiceMunicipal  string
	ServicePlace      string
	ServiceCountry    string
	ServiceDesc       string
	MunicipalTaxes    municipalTaxes
	FederalTaxes      federalTaxes
	ServiceAmount     string
	CalculationBasis  string
	AppliedRate       string
	ISSQN             string
	ISSQNRetained     string
	TotalRetentions   string
	NetValue          string
	FederalApprox     string
	StateApprox       string
	MunicipalApprox   string
	ComplementaryInfo string
}

type party struct {
	ID                    string
	MunicipalRegistration string
	Phone                 string
	Name                  string
	Email                 string
	Address               string
	City                  string
	CEP                   string
	SimpleNational        string
	TaxRegime             string
}

type municipalTaxes struct {
	Taxation           string
	Country            string
	IncidenceCity      string
	SpecialTaxRegime   string
	ImmunityType       string
	Suspension         string
	SuspensionProcess  string
	MunicipalBenefit   string
	Deductions         string
	BenefitCalculation string
	RetentionType      string
	ClearedISSQN       string
}

type federalTaxes struct {
	IRRF                       string
	SocialSecurityContribution string
	SocialContributions        string
	SocialContributionsDesc    string
	PISDebit                   string
	COFINSDebit                string
	TotalFederalRetentions     string
	PISCOFINSDebit             string
}

func parseData(root *xmlutil.Node, config Config) nfseData {
	infNFSe := root.Find("infNFSe")
	dps := root.Find("DPS")
	emit := root.Find("emit")
	issuerAddress := emit.Find("enderNac")
	prest := dps.Find("prest")
	regTrib := prest.Find("regTrib")
	takerNode := dps.Find("toma")
	service := dps.Find("serv")
	values := infNFSe.Find("valores")
	if values == nil {
		values = root.Find("valores")
	}

	competenceDate, _ := fiscalfmt.DateUTC(xmlutil.Text(dps, "dCompet"))
	processedDate, processedTime := fiscalfmt.DateUTC(xmlutil.Text(infNFSe, "dhProc"))
	dpsDate, dpsTime := fiscalfmt.DateUTC(xmlutil.Text(dps, "dhEmi"))

	serviceAmount := fiscalfmt.FormatNumber(xmlutil.Text(dps, "vServ"), config.DecimalConfig.PricePrecision)
	if serviceAmount == "0,0000" {
		serviceAmount = fiscalfmt.FormatNumber(xmlutil.Text(dps, "vServPrest"), config.DecimalConfig.PricePrecision)
	}

	key := strings.TrimPrefix(infNFSe.Attr("Id"), "NFS")
	issqnType := xmlutil.Text(dps, "tpRetISSQN")
	issqnValue := xmlutil.Text(values, "vISSQN")
	if issqnValue == "" {
		issqnValue = xmlutil.Text(dps, "vISSQN")
	}
	issqnRetained := "0"
	if issqnType == "2" || issqnType == "3" {
		issqnRetained = issqnValue
	}
	retentionType := map[string]string{
		"1": "Não Retido",
		"2": "Retido pelo Tomador",
		"3": "Retido pelo Intermediario",
	}[issqnType]
	if retentionType == "" {
		retentionType = "-"
	}

	totalFederalRetentions := "0"
	if totalRetentions := xmlutil.Text(values, "vTotalRet"); totalRetentions != "" {
		totalFederalRetentions = totalRetentions
	}

	federalTaxes := parseFederalTaxes(dps, values, issqnRetained, totalFederalRetentions, config.DecimalConfig.PricePrecision)

	return nfseData{
		Environment:      xmlutil.Text(dps, "tpAmb"),
		Key:              key,
		Number:           xmlutil.Text(infNFSe, "nNFSe"),
		Competence:       competenceDate,
		ProcessedDate:    processedDate,
		ProcessedTime:    processedTime,
		DPSNumber:        xmlutil.Text(dps, "nDPS"),
		DPSSeries:        xmlutil.Text(dps, "serie"),
		DPSEmissionDate:  dpsDate,
		DPSEmissionTime:  dpsTime,
		Issuer:           parseIssuer(infNFSe, emit, issuerAddress, regTrib),
		Taker:            parseTaker(takerNode),
		ServiceCode:      serviceNationalTaxCode(xmlutil.Text(service, "cTribNac"), xmlutil.Text(service, "xDescServ"), config.Margins, config.FontType),
		ServiceMunicipal: optional(xmlutil.Text(service, "cTribMun")),
		ServicePlace:     strings.TrimSpace(xmlutil.Text(infNFSe, "xLocPrestacao") + " - " + xmlutil.Text(emit, "UF")),
		ServiceCountry:   optional(xmlutil.Text(service, "cPaisPrestacao")),
		ServiceDesc:      xmlutil.Text(service, "xDescServ"),
		MunicipalTaxes: municipalTaxes{
			Taxation:           issqnTaxation(xmlutil.Text(dps, "tribISSQN")),
			Country:            optional(xmlutil.Text(dps, "cPaisResult")),
			IncidenceCity:      optional(strings.TrimSpace(xmlutil.Text(infNFSe, "xLocIncid") + " - " + xmlutil.Text(emit, "UF"))),
			SpecialTaxRegime:   specialTaxRegime(xmlutil.Text(regTrib, "regEspTrib")),
			ImmunityType:       optional(xmlutil.Text(dps, "tpImunidade")),
			Suspension:         "Não",
			SuspensionProcess:  "-",
			MunicipalBenefit:   "-",
			Deductions:         "-",
			BenefitCalculation: "-",
			RetentionType:      retentionType,
			ClearedISSQN:       money(fiscalfmt.FormatNumber(clearedISSQN(issqnType, issqnValue), config.DecimalConfig.PricePrecision)),
		},
		FederalTaxes:      federalTaxes,
		ServiceAmount:     money(serviceAmount),
		CalculationBasis:  money(fiscalfmt.FormatNumber(xmlutil.Text(values, "vBC"), config.DecimalConfig.PricePrecision)),
		AppliedRate:       rate(xmlutil.Text(values, "pAliqAplic"), config.DecimalConfig.PricePrecision),
		ISSQN:             money(fiscalfmt.FormatNumber(issqnValue, config.DecimalConfig.PricePrecision)),
		ISSQNRetained:     money(fiscalfmt.FormatNumber(issqnRetained, config.DecimalConfig.PricePrecision)),
		TotalRetentions:   money(fiscalfmt.FormatNumber(xmlutil.Text(values, "vTotalRet"), config.DecimalConfig.PricePrecision)),
		NetValue:          money(fiscalfmt.FormatNumber(xmlutil.Text(values, "vLiq"), config.DecimalConfig.PricePrecision)),
		FederalApprox:     approxTax(xmlutil.Text(dps, "vTotTribFed"), config.DecimalConfig.PricePrecision),
		StateApprox:       approxTax(xmlutil.Text(dps, "vTotTribEst"), config.DecimalConfig.PricePrecision),
		MunicipalApprox:   approxTax(xmlutil.Text(dps, "vTotTribMun"), config.DecimalConfig.PricePrecision),
		ComplementaryInfo: optional(xmlutil.Text(service, "xInfComp")),
	}
}

func serviceNationalTaxCode(nationalTax, description string, margins Margins, fontType FontType) string {
	formattedTax := nationalTax
	if len(nationalTax) >= 6 {
		formattedTax = nationalTax[:2] + "." + nationalTax[2:4] + "." + nationalTax[4:]
	}
	formattedDescription := strings.TrimSpace(description)
	if before, after, ok := strings.Cut(formattedDescription, " - "); ok && before != "" {
		formattedDescription = strings.TrimSpace(after)
	}
	return longField(strings.TrimSpace(formattedTax+" - "+formattedDescription), serviceColumnWidth(margins), 8, string(fontType))
}

func serviceColumnWidth(margins Margins) float64 {
	return (210 - margins.Left - margins.Right) / 4
}

func longField(text string, limitMM float64, fontSize float64, fontFamily string) string {
	if strings.TrimSpace(text) == "" || limitMM <= 0 {
		return ""
	}
	pdf := pdfdraw.NewPDF("P", "mm", "A4", "")
	pdf.SetFont(fontFamily, "", fontSize)
	safeLimit := limitMM
	if pdf.GetStringWidth(pdf.Encode(text)) <= safeLimit {
		return text
	}
	words := strings.Fields(text)
	for len(words) > 0 && pdf.GetStringWidth(pdf.Encode(strings.Join(words, " ")+"...")) > safeLimit {
		words = words[:len(words)-1]
	}
	if len(words) > 0 {
		return strings.Join(words, " ") + "..."
	}
	runes := []rune(text)
	for len(runes) > 0 && pdf.GetStringWidth(pdf.Encode(string(runes)+"...")) > safeLimit {
		runes = runes[:len(runes)-1]
	}
	if len(runes) == 0 {
		return ""
	}
	return string(runes) + "..."
}

func parseIssuer(infNFSe, emit, address, regTrib *xmlutil.Node) party {
	return party{
		ID:                    firstNonEmpty(fiscalfmt.FormatCPFCNPJ(xmlutil.Text(emit, "CNPJ")), fiscalfmt.FormatCPFCNPJ(xmlutil.Text(emit, "CPF")), "-"),
		MunicipalRegistration: optional(xmlutil.Text(emit, "IM")),
		Phone:                 optional(fiscalfmt.FormatPhone(xmlutil.Text(emit, "fone"))),
		Name:                  firstNonEmpty(xmlutil.Text(emit, "xNome"), xmlutil.Text(emit, "xFant"), "-"),
		Email:                 optional(xmlutil.Text(emit, "email")),
		Address:               formatAddress(address),
		City:                  strings.TrimSpace(xmlutil.Text(infNFSe, "xLocEmi") + " - " + xmlutil.Text(emit, "UF")),
		CEP:                   fiscalfmt.FormatCEP(xmlutil.Text(address, "CEP")),
		SimpleNational:        simpleNational(xmlutil.Text(regTrib, "opSimpNac")),
		TaxRegime:             taxRegime(xmlutil.Text(regTrib, "regApTribSN"), xmlutil.Text(regTrib, "opSimpNac")),
	}
}

func parseTaker(taker *xmlutil.Node) party {
	if taker == nil {
		return party{ID: "-", MunicipalRegistration: "-", Phone: "-", Name: "-", Email: "-", Address: "-", City: "-", CEP: "-"}
	}
	address := taker.Find("end")
	return party{
		ID:                    firstNonEmpty(fiscalfmt.FormatCPFCNPJ(xmlutil.Text(taker, "CNPJ")), fiscalfmt.FormatCPFCNPJ(xmlutil.Text(taker, "CPF")), "-"),
		MunicipalRegistration: optional(xmlutil.Text(taker, "IM")),
		Phone:                 optional(fiscalfmt.FormatPhone(xmlutil.Text(taker, "fone"))),
		Name:                  optional(xmlutil.Text(taker, "xNome")),
		Email:                 optional(xmlutil.Text(taker, "email")),
		Address:               formatAddress(address),
		City:                  strings.TrimSpace(xmlutil.Text(address, "xMun") + " - " + xmlutil.Text(address, "UF")),
		CEP:                   fiscalfmt.FormatCEP(xmlutil.Text(address, "CEP")),
	}
}

func formatAddress(node *xmlutil.Node) string {
	parts := []string{
		xmlutil.Text(node, "xLgr"),
		xmlutil.Text(node, "nro"),
		xmlutil.Text(node, "xCpl"),
		xmlutil.Text(node, "xBairro"),
	}
	var filtered []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filtered = append(filtered, strings.TrimSpace(part))
		}
	}
	if len(filtered) == 0 {
		return "-"
	}
	return strings.Join(filtered, ", ")
}

func simpleNational(code string) string {
	switch code {
	case "2":
		return "Optante - Microempreendedor Individual (MEI)"
	case "3":
		return "Optante - Microempresa ou Empresa de Pequeno Porte (ME/EPP)"
	default:
		return "Não Optante"
	}
}

func taxRegime(code, simplesCode string) string {
	if simplesCode != "3" {
		return "-"
	}
	switch code {
	case "1":
		return "Regime de apuração dos tributos federais e municipal pelo Simples Nacional"
	case "2":
		return "Regime de apuração dos tributos federais pelo SN e o ISSQN pela NFS-e conforme respectiva legislação municipal do tributo"
	case "3":
		return "Regime de apuração dos tributos federais e municipal pela NFS-e conforme respectivas legislações federal e municipal de cada tributo"
	default:
		return "-"
	}
}

func issqnTaxation(code string) string {
	switch code {
	case "2":
		return "Imunidade"
	case "3":
		return "Exportação de serviço"
	case "4":
		return "Não Incidência"
	default:
		return "Operação Tributável"
	}
}

func specialTaxRegime(code string) string {
	switch code {
	case "1":
		return "Ato Cooperado (Cooperativa)"
	case "2":
		return "Estimativa"
	case "3":
		return "Microempresa Municipal"
	case "4":
		return "Notário ou Registrador"
	case "5":
		return "Profissional Autônomo"
	case "6":
		return "Sociedade de Profissionais"
	case "9":
		return "Outros"
	default:
		return "Nenhum"
	}
}

func clearedISSQN(retentionType, value string) string {
	if retentionType == "1" {
		return value
	}
	return "0"
}

func parseFederalTaxes(dps, values *xmlutil.Node, issqnRetained, totalRetentions string, precision int) federalTaxes {
	taxes := federalTaxes{
		IRRF:                       "-",
		SocialSecurityContribution: "-",
		SocialContributions:        "-",
		SocialContributionsDesc:    "-",
		PISDebit:                   "-",
		COFINSDebit:                "-",
		TotalFederalRetentions:     money(fiscalfmt.FormatNumber("0", precision)),
		PISCOFINSDebit:             "-",
	}
	tribFed := dps.Find("tribFed")
	if tribFed != nil {
		if value := xmlutil.Text(tribFed, "vRetIRRF"); value != "" {
			taxes.IRRF = value
		}
		if value := xmlutil.Text(tribFed, "vRetCP"); value != "" {
			taxes.SocialSecurityContribution = money(fiscalfmt.FormatNumber(value, precision))
		}
		if value := xmlutil.Text(tribFed, "vRetCSLL"); value != "" {
			taxes.SocialContributions = money(fiscalfmt.FormatNumber(value, precision))
		}
		if pisCofins := tribFed.Find("piscofins"); pisCofins != nil {
			pis := xmlutil.Text(pisCofins, "vPis")
			cofins := xmlutil.Text(pisCofins, "vCofins")
			taxes.PISDebit = money(fiscalfmt.FormatNumber(pis, precision))
			taxes.COFINSDebit = money(fiscalfmt.FormatNumber(cofins, precision))
			pisValue, _ := parseFloat(pis)
			cofinsValue, _ := parseFloat(cofins)
			taxes.PISCOFINSDebit = money(fiscalfmt.FormatNumber(formatFloat(pisValue+cofinsValue), precision))
		}
	}
	if totalRetentions != "" {
		total, okTotal := parseFloat(totalRetentions)
		issqn, okISSQN := parseFloat(issqnRetained)
		if okTotal && okISSQN {
			taxes.TotalFederalRetentions = money(fiscalfmt.FormatNumber(formatFloat(total-issqn), precision))
		}
	}
	if taxes.TotalFederalRetentions == "" && xmlutil.Text(values, "vTotalRet") != "" {
		taxes.TotalFederalRetentions = money(fiscalfmt.FormatNumber(xmlutil.Text(values, "vTotalRet"), precision))
	}
	return taxes
}

func drawFooterStamp(pdf *pdfdraw.PDF, config Config) {
	footer.Draw(pdf, config.FooterStamp, "danfse-footer-logo",
		config.Margins.Left, config.Margins.Right, config.Margins.Bottom,
		string(config.FontType), 6)
}

func draw(pdf *pdfdraw.PDF, data nfseData, config Config) {
	drawWatermark(pdf, data, config)
	x := config.Margins.Left
	y := config.Margins.Top
	w := 210 - config.Margins.Left - config.Margins.Right
	h := 297 - config.Margins.Top - bottomMargin(config)
	pdf.Rect(x, y, w, h, "")
	drawHeader(pdf, x, y, w, data, config)
	y += 39
	drawIssuer(pdf, x+2, y, w-4, data.Issuer, config)
	y += 23
	drawParty(pdf, x+2, y, w-4, 28, "TOMADOR DO SERVIÇO", data.Taker, config)
	y += 30
	drawIntermediary(pdf, x+2, y, w-4, config)
	y -= 2
	drawServiceProvided(pdf, x+2, y, w-4, data, config)
	y += 25
	drawTaxes(pdf, x, y, w, data, config)
	y += 53
	drawAmount(pdf, x, y, w, data, config)
	y += 35
	drawComplementaryInfo(pdf, x+2, y, w-4, data.ComplementaryInfo, config)
}

func drawWatermark(pdf *pdfdraw.PDF, data nfseData, config Config) {
	text := ""
	size := 60.0
	if config.WatermarkCancelled {
		if data.Environment == "1" {
			text = "CANCELADA"
		} else {
			text = "CANCELADA - SEM VALOR FISCAL"
			size = 45
		}
	} else if data.Environment != "1" {
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

func drawHeader(pdf *pdfdraw.PDF, x, y, w float64, data nfseData, config Config) {
	if len(nfseLogoPNG) > 0 {
		pdf.ImageBytes("danfse-logo", nfseLogoPNG, x+2, y+2, 42, 0)
	}
	colW := w / 4
	sectionY := y + 12
	pdf.SetFont(string(config.FontType), "B", 10)
	pdf.SetXY(x, y+4)
	pdf.MultiCell(w, 2.5, "DANFSe v1.0\nDocumento Auxiliar da NFS-e", "", "C", false)
	pdf.Line(x+2, y+13, x+w-2, y+13)

	drawDANFSEField(pdf, x+3, sectionY+2, colW*3, "Chave de Acesso da NFS-e", data.Key, 7, 8, 0, config)
	drawDANFSEField(pdf, x+3, sectionY+9, colW, "Número da NFS-e", data.Number, 7, 8, 0, config)
	drawDANFSEField(pdf, x+colW, sectionY+9, colW, "Competência da NFS-e", data.Competence, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+9, colW, "Data e Hora da emissão da NFS-e", strings.TrimSpace(data.ProcessedDate+" "+data.ProcessedTime), 7, 8, 0, config)
	drawDANFSEField(pdf, x+3, sectionY+16, colW, "Número da DPS", data.DPSNumber, 7, 8, 0, config)
	drawDANFSEField(pdf, x+colW, sectionY+16, colW, "Série da DPS", data.DPSSeries, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+16, colW, "Data e Hora da emissão da DPS", strings.TrimSpace(data.DPSEmissionDate+" "+data.DPSEmissionTime), 7, 8, 0, config)

	drawQR(pdf, 173.87, 15.77, 19, data.Key)
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.SetXY(x+(colW*3)-2, sectionY+18)
	pdf.MultiCell(colW, 3, "A autenticidade desta NFS-e pode ser verificada pela leitura deste código QR ou pela consulta da chave de acesso no portal nacional da NFS-e", "", "L", false)
	pdf.Line(x+2, y+40, x+w-2, y+40)
}

func drawQR(pdf *pdfdraw.PDF, x, y, size float64, key string) {
	if key == "" {
		return
	}
	pngBytes, err := qrcode.PNG("https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave="+key, 128)
	if err != nil {
		return
	}
	name := "danfse-qr-" + key
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, size, size, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

type field struct {
	Label string
	Value string
}

func drawIssuer(pdf *pdfdraw.PDF, x, y, w float64, p party, config Config) {
	colW := w / 4
	sectionY := y + 2
	drawDANFSETitle(pdf, x+1, sectionY, "EMITENTE DA NFS-e", 8, config)
	drawDANFSEValueOnly(pdf, x+1, sectionY, colW, "Prestador do Serviço", 8, config)
	drawDANFSEField(pdf, x+colW, sectionY, colW, "CNPJ / CPF / NIF", p.ID, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY, colW, "Inscrição Municipal", p.MunicipalRegistration, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*3), sectionY, colW, "Telefone", p.Phone, 7, 8, 0, config)
	drawDANFSEField(pdf, x+1, sectionY+7, colW, "Nome / Nome Empresarial", p.Name, 7, 8, colW*2, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+7, colW, "E-mail", p.Email, 7, 8, colW*2, config)
	drawDANFSEField(pdf, x+1, sectionY+14, colW, "Endereço", p.Address, 7, 8, colW*2, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+14, colW, "Município", p.City, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*3), sectionY+14, colW, "CEP", p.CEP, 7, 8, 0, config)
	drawDANFSEField(pdf, x+1, sectionY+21, colW, "Simples Nacional na Data de Competência", p.SimpleNational, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+21, colW, "Regime de Apuração Tributária pelo SN", p.TaxRegime, 7, 8, 0, config)
	pdf.Line(x, y+30, x+w, y+30)
}

func drawParty(pdf *pdfdraw.PDF, x, y, w, h float64, title string, p party, config Config) {
	colW := w / 4
	sectionY := y + 9
	drawDANFSETitle(pdf, x+1, sectionY, title, 9, config)
	drawDANFSEField(pdf, x+colW, sectionY, colW, "CNPJ / CPF / NIF", p.ID, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY, colW, "Inscrição Municipal", p.MunicipalRegistration, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*3), sectionY, colW, "Telefone", p.Phone, 7, 8, 0, config)
	drawDANFSEField(pdf, x+1, sectionY+6, colW, "Nome / Nome Empresarial", p.Name, 7, 8, colW*2, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+6, colW, "E-mail", p.Email, 7, 8, colW*2, config)
	drawDANFSEField(pdf, x+1, sectionY+13, colW, "Endereço", p.Address, 7, 8, colW*2, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+13, colW, "Município", p.City, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*3), sectionY+13, colW, "CEP", p.CEP, 7, 8, 0, config)
	pdf.Line(x, y+h, x+w, y+h)
}

func drawIntermediary(pdf *pdfdraw.PDF, x, y, w float64, config Config) {
	pdf.SetFont(string(config.FontType), "B", 8)
	pdf.SetXY(x+1, y-2)
	pdf.CellFormat(w-2, 3, "INTERMEDIÁRIO DO SERVIÇO NÃO IDENTIFICADO NA NFS-e", "", 1, "C", false, 0, "")
	pdf.Line(x, y+1, x+w, y+1)
}

func drawServiceProvided(pdf *pdfdraw.PDF, x, y, w float64, data nfseData, config Config) {
	colW := w / 4
	sectionY := y + 5
	drawDANFSETitle(pdf, x+1, sectionY, "SERVIÇO PRESTADO", 9, config)
	drawDANFSEMultilineField(pdf, x+1, sectionY+4, colW, "Código de Tributação Nacional", data.ServiceCode, colW, config)
	drawDANFSEField(pdf, x+colW, sectionY+4, colW, "Código de Tributação Municipal", data.ServiceMunicipal, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+4, colW, "Local da Prestação", data.ServicePlace, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*3), sectionY+4, colW, "País da Prestação", data.ServiceCountry, 7, 8, 0, config)
	drawDANFSEField(pdf, x+1, sectionY+14, colW, "Descrição do Serviço", data.ServiceDesc, 7, 8, w-1, config)
	pdf.Line(x, y+25, x+w, y+25)
}

func drawTaxes(pdf *pdfdraw.PDF, x, y, w float64, data nfseData, config Config) {
	colW := w / 4
	sectionY := y + 1
	drawDANFSETitle(pdf, x+3, sectionY, "TRIBUTAÇÃO MUNICIPAL", 9, config)

	municipalRows := [][]field{
		{
			{"Tributação do ISSQN", data.MunicipalTaxes.Taxation},
			{"País Resultado da Prestação do Serviço", data.MunicipalTaxes.Country},
			{"Município de Incidência do ISSQN", data.MunicipalTaxes.IncidenceCity},
			{"Regime Especial de Tributação", data.MunicipalTaxes.SpecialTaxRegime},
		},
		{
			{"Tipo de Imunidade", data.MunicipalTaxes.ImmunityType},
			{"Suspensão da Exigibilidade do ISSQN", data.MunicipalTaxes.Suspension},
			{"Número Processo Suspensão", data.MunicipalTaxes.SuspensionProcess},
			{"Benefício Municipal", data.MunicipalTaxes.MunicipalBenefit},
		},
		{
			{"Valor do Serviço", data.ServiceAmount},
			{"Desconto Incondicionado", "-"},
			{"Total Deduções/Reduções", data.MunicipalTaxes.Deductions},
			{"Cálculo do BM", data.MunicipalTaxes.BenefitCalculation},
		},
		{
			{"BC ISSQN", data.CalculationBasis},
			{"Alíquota Aplicada", data.AppliedRate},
			{"Retenção do ISSQN", data.MunicipalTaxes.RetentionType},
			{"ISSQN Apurado", data.MunicipalTaxes.ClearedISSQN},
		},
	}
	drawDANFSEFieldRows(pdf, x, sectionY+5, colW, municipalRows, 7, config)
	pdf.Line(x+2, y+34, x+w-2, y+34)

	federalY := y + 35
	drawDANFSETitle(pdf, x+3, federalY, "TRIBUTAÇÃO FEDERAL", 9, config)
	federalRows := [][]field{
		{
			{"IRRF", data.FederalTaxes.IRRF},
			{"Contribuição Previdenciária - Retida", data.FederalTaxes.SocialSecurityContribution},
			{"Contribuições Sociais - Retidas", data.FederalTaxes.SocialContributions},
			{"Descrição Contrib. Sociais - Retidas", data.FederalTaxes.SocialContributionsDesc},
		},
		{
			{"PIS - Débito Apuração Própria", data.FederalTaxes.PISDebit},
			{"COFINS - Débito Apuração Própria", data.FederalTaxes.COFINSDebit},
		},
	}
	drawDANFSEFieldRows(pdf, x, federalY+4, colW, federalRows, 7, config)
	pdf.Line(x+2, y+53, x+w-2, y+53)
}

func drawAmount(pdf *pdfdraw.PDF, x, y, w float64, data nfseData, config Config) {
	colW := w / 4
	sectionY := y + 1
	drawDANFSETitle(pdf, x+3, sectionY, "VALOR TOTAL DA NFS-E", 9, config)
	drawDANFSEField(pdf, x+3, sectionY+5, colW, "Valor do Serviço", data.ServiceAmount, 7, 8, 0, config)
	drawDANFSEField(pdf, x+colW, sectionY+5, colW, "Desconto Condicionado", "-", 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*2), sectionY+5, colW, "Desconto Incondicionado", "-", 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*3), sectionY+5, colW, "ISSQN Retido", data.ISSQNRetained, 7, 8, 0, config)
	drawDANFSEField(pdf, x+3, sectionY+12, colW, "Total das Retenções Federais", data.FederalTaxes.TotalFederalRetentions, 7, 8, 0, config)
	drawDANFSEField(pdf, x+colW, sectionY+12, colW, "PIS/COFINS - Débito Apur. Própria", data.FederalTaxes.PISCOFINSDebit, 7, 8, 0, config)
	drawDANFSEField(pdf, x+(colW*3), sectionY+12, colW, "Valor Líquido da NFS-e", data.NetValue, 7, 8, 0, config)
	pdf.Line(x+2, y+21, x+w-2, y+21)

	taxColW := w / 3
	totalsY := sectionY + 23
	drawDANFSETitle(pdf, x+3, totalsY, "TOTAIS APROXIMADOS DOS TRIBUTOS", 9, config)
	drawDANFSECenteredField(pdf, x+3, totalsY+5, taxColW, "Federais", data.FederalApprox, config)
	drawDANFSECenteredField(pdf, x+taxColW, totalsY+5, taxColW, "Estaduais", data.StateApprox, config)
	drawDANFSECenteredField(pdf, x+(taxColW*2), totalsY+5, taxColW, "Municipais", data.MunicipalApprox, config)
	pdf.Line(x+2, y+35, x+w-2, y+35)
}

func drawDANFSEFieldRows(pdf *pdfdraw.PDF, x, y, colW float64, rows [][]field, rowH float64, config Config) {
	for rowIndex, row := range rows {
		rowY := y + float64(rowIndex)*rowH
		for colIndex, field := range row {
			fieldX := x + float64(colIndex)*colW
			if colIndex == 0 {
				fieldX = x + 3
			}
			drawDANFSEField(pdf, fieldX, rowY, colW, field.Label, field.Value, 7, 8, 0, config)
		}
	}
}

func drawDANFSECenteredField(pdf *pdfdraw.PDF, x, y, w float64, label, value string, config Config) {
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 3, label, "", 0, "C", false, 0, "")
	pdf.SetFont(string(config.FontType), "", 8)
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 8, value, "", 0, "C", false, 0, "")
}

func drawDANFSETitle(pdf *pdfdraw.PDF, x, y float64, text string, size float64, config Config) {
	pdf.SetFont(string(config.FontType), "B", size)
	pdf.SetXY(x, y)
	pdf.CellFormat(0, 3, text, "", 0, "L", false, 0, "")
}

func drawDANFSEValueOnly(pdf *pdfdraw.PDF, x, y, w float64, value string, size float64, config Config) {
	pdf.SetFont(string(config.FontType), "", size)
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 8, value, "", 0, "L", false, 0, "")
}

func drawDANFSEField(pdf *pdfdraw.PDF, x, y, w float64, label, value string, labelSize, valueSize, limit float64, config Config) {
	pdf.SetFont(string(config.FontType), "B", labelSize)
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 3, label, "", 0, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", valueSize)
	pdf.SetXY(x, y)
	if limit > 0 {
		value = longFieldPDF(pdf, value, limit)
	}
	pdf.CellFormat(w, 8, value, "", 0, "L", false, 0, "")
}

func drawDANFSEMultilineField(pdf *pdfdraw.PDF, x, y, w float64, label, value string, limit float64, config Config) {
	pdf.SetFont(string(config.FontType), "B", 7)
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 3, label, "", 0, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", 8)
	pdf.SetXY(x, y+3)
	if limit > 0 {
		value = longFieldPDF(pdf, value, limit)
	}
	pdf.MultiCell(w, 2.5, value, "", "L", false)
}

func drawComplementaryInfo(pdf *pdfdraw.PDF, x, y, w float64, value string, config Config) {
	pdf.SetFont(string(config.FontType), "B", 8)
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 3, "INFORMAÇÕES COMPLEMENTARES", "", 1, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.SetXY(x+1, y+5)
	pdf.CellFormat(w-2, 3, longFieldPDF(pdf, optional(value), w-2), "", 1, "L", false, 0, "")
}

func longFieldPDF(pdf *pdfdraw.PDF, text string, limitMM float64) string {
	if strings.TrimSpace(text) == "" || limitMM <= 0 {
		return ""
	}
	if pdf.GetStringWidth(pdf.Encode(text)) <= limitMM {
		return text
	}
	words := strings.Fields(text)
	for len(words) > 0 && pdf.GetStringWidth(pdf.Encode(strings.Join(words, " ")+"...")) > limitMM {
		words = words[:len(words)-1]
	}
	if len(words) > 0 {
		return strings.Join(words, " ") + "..."
	}
	runes := []rune(text)
	for len(runes) > 0 && pdf.GetStringWidth(pdf.Encode(string(runes)+"...")) > limitMM {
		runes = runes[:len(runes)-1]
	}
	if len(runes) == 0 {
		return ""
	}
	return string(runes) + "..."
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

func money(value string) string {
	if value == "" {
		value = "0,0000"
	}
	return "R$ " + value
}

func rate(value string, precision int) string {
	if value == "" {
		return "-"
	}
	return fiscalfmt.FormatNumber(value, precision) + "%"
}

func approxTax(value string, precision int) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return money(fiscalfmt.FormatNumber(value, precision))
}

func parseFloat(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed, err == nil
}

func formatFloat(value float64) string {
	return fmt.Sprintf("%.10f", value)
}
