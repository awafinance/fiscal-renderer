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
}

func New(xml string, config *Config) (*Document, error) {
	if _, err := xmlutil.ParseString(xml); err != nil {
		return nil, err
	}
	return &Document{XML: xml, Config: normalizeConfig(config)}, nil
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
	root, err := xmlutil.ParseString(d.XML)
	if err != nil {
		return err
	}
	data := parseData(root, d.Config)
	pdf := pdfdraw.NewPDF("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(d.Config.Margins.Left, d.Config.Margins.Top, d.Config.Margins.Right)
	pdf.SetAutoPageBreak(false, d.Config.Margins.Bottom)
	pdf.SetTitle("DANFSE", false)
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

func draw(pdf *pdfdraw.PDF, data nfseData, config Config) {
	drawWatermark(pdf, data, config)
	x := config.Margins.Left
	y := config.Margins.Top
	w := 210 - config.Margins.Left - config.Margins.Right
	h := 297 - config.Margins.Top - config.Margins.Bottom
	pdf.Rect(x, y, w, h, "")
	drawHeader(pdf, x, y, w, data, config)
	y += 44
	drawIssuer(pdf, x+2, y, w-4, data.Issuer, config)
	y += 31
	drawParty(pdf, x+2, y, w-4, 28, "TOMADOR DO SERVIÇO", data.Taker, config)
	y += 30
	drawIntermediary(pdf, x+2, y, w-4, config)
	y += 6
	drawBox(pdf, x+2, y, w-4, 27, "SERVIÇO PRESTADO", []field{
		{"Código de Tributação Nacional", data.ServiceCode},
		{"Código de Tributação Municipal", data.ServiceMunicipal},
		{"Local da Prestação", data.ServicePlace},
		{"País da Prestação", data.ServiceCountry},
		{"Descrição do Serviço", data.ServiceDesc},
	}, config)
	y += 29
	drawBox(pdf, x+2, y, w-4, 34, "TRIBUTAÇÃO MUNICIPAL", []field{
		{"Tributação do ISSQN", data.MunicipalTaxes.Taxation},
		{"País Resultado da Prestação do Serviço", data.MunicipalTaxes.Country},
		{"Município de Incidência do ISSQN", data.MunicipalTaxes.IncidenceCity},
		{"Regime Especial de Tributação", data.MunicipalTaxes.SpecialTaxRegime},
		{"Tipo de Imunidade", data.MunicipalTaxes.ImmunityType},
		{"Suspensão da Exigibilidade do ISSQN", data.MunicipalTaxes.Suspension},
		{"Número Processo Suspensão", data.MunicipalTaxes.SuspensionProcess},
		{"Benefício Municipal", data.MunicipalTaxes.MunicipalBenefit},
		{"Valor do Serviço", data.ServiceAmount},
		{"Desconto Incondicionado", "-"},
		{"Total Deduções/Reduções", data.MunicipalTaxes.Deductions},
		{"Cálculo do BM", data.MunicipalTaxes.BenefitCalculation},
		{"BC ISSQN", data.CalculationBasis},
		{"Alíquota Aplicada", data.AppliedRate},
		{"Retenção do ISSQN", data.MunicipalTaxes.RetentionType},
		{"ISSQN Apurado", data.MunicipalTaxes.ClearedISSQN},
	}, config)
	y += 36
	drawBox(pdf, x+2, y, w-4, 20, "TRIBUTAÇÃO FEDERAL", []field{
		{"IRRF", data.FederalTaxes.IRRF},
		{"Contribuição Previdenciária - Retida", data.FederalTaxes.SocialSecurityContribution},
		{"Contribuições Sociais - Retidas", data.FederalTaxes.SocialContributions},
		{"Descrição Contrib. Sociais - Retidas", data.FederalTaxes.SocialContributionsDesc},
		{"PIS - Débito Apuração Própria", data.FederalTaxes.PISDebit},
		{"COFINS - Débito Apuração Própria", data.FederalTaxes.COFINSDebit},
	}, config)
	y += 22
	drawBox(pdf, x+2, y, w-4, 20, "VALOR TOTAL DA NFS-E", []field{
		{"Valor do Serviço", data.ServiceAmount},
		{"Desconto Condicionado", "-"},
		{"Desconto Incondicionado", "-"},
		{"ISSQN Retido", data.ISSQNRetained},
		{"Total das Retenções Federais", data.FederalTaxes.TotalFederalRetentions},
		{"PIS/COFINS - Débito Apur. Própria", data.FederalTaxes.PISCOFINSDebit},
		{"Valor Líquido da NFS-e", data.NetValue},
	}, config)
	y += 22
	drawBox(pdf, x+2, y, w-4, 12, "TOTAIS APROXIMADOS DOS TRIBUTOS", []field{
		{"Federais", data.FederalApprox},
		{"Estaduais", data.StateApprox},
		{"Municipais", data.MunicipalApprox},
	}, config)
	y += 15
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
	pdf.SetFont(string(config.FontType), "B", 10)
	pdf.SetXY(x, y+4)
	pdf.MultiCell(w, 3, "DANFSe v1.0\nDocumento Auxiliar da NFS-e", "", "C", false)
	drawQR(pdf, x+w-34, y+2, data.Key)
	drawBox(pdf, x+2, y+17, w-38, 24, "", []field{
		{"Chave de Acesso da NFS-e", data.Key},
		{"Número da NFS-e", data.Number},
		{"Competência da NFS-e", data.Competence},
		{"Data e Hora da emissão da NFS-e", strings.TrimSpace(data.ProcessedDate + " " + data.ProcessedTime)},
		{"Número da DPS", data.DPSNumber},
		{"Série da DPS", data.DPSSeries},
		{"Data e Hora da emissão da DPS", strings.TrimSpace(data.DPSEmissionDate + " " + data.DPSEmissionTime)},
	}, config)
	pdf.SetFont(string(config.FontType), "", 6)
	pdf.SetXY(x+w-43, y+32)
	pdf.MultiCell(38, 2.5, "A autenticidade desta NFS-e pode ser verificada pela leitura deste código QR ou pela consulta da chave de acesso no portal nacional da NFS-e", "", "L", false)
}

func drawQR(pdf *pdfdraw.PDF, x, y float64, key string) {
	if key == "" {
		return
	}
	pngBytes, err := qrcode.PNG("https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave="+key, 128)
	if err != nil {
		return
	}
	name := "danfse-qr-" + key
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, x, y, 30, 30, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

type field struct {
	Label string
	Value string
}

func drawIssuer(pdf *pdfdraw.PDF, x, y, w float64, p party, config Config) {
	drawBox(pdf, x, y, w, 36, "EMITENTE DA NFS-e", []field{
		{"Prestador do Serviço", ""},
		{"CNPJ / CPF / NIF", p.ID},
		{"Inscrição Municipal", p.MunicipalRegistration},
		{"Telefone", p.Phone},
		{"Nome / Nome Empresarial", p.Name},
		{"E-mail", p.Email},
		{"Endereço", p.Address},
		{"Município", p.City},
		{"CEP", p.CEP},
		{"Simples Nacional na Data de Competência", p.SimpleNational},
		{"Regime de Apuração Tributária pelo SN", p.TaxRegime},
	}, config)
}

func drawParty(pdf *pdfdraw.PDF, x, y, w, h float64, title string, p party, config Config) {
	drawBox(pdf, x, y, w, h, title, []field{
		{"CNPJ / CPF / NIF", p.ID},
		{"Inscrição Municipal", p.MunicipalRegistration},
		{"Telefone", p.Phone},
		{"Nome / Nome Empresarial", p.Name},
		{"E-mail", p.Email},
		{"Endereço", p.Address},
		{"Município", p.City},
		{"CEP", p.CEP},
	}, config)
}

func drawIntermediary(pdf *pdfdraw.PDF, x, y, w float64, config Config) {
	pdf.SetFont(string(config.FontType), "B", 8)
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 3, "INTERMEDIÁRIO DO SERVIÇO NÃO IDENTIFICADO NA NFS-e", "", 1, "C", false, 0, "")
}

func drawComplementaryInfo(pdf *pdfdraw.PDF, x, y, w float64, value string, config Config) {
	pdf.SetFont(string(config.FontType), "B", 8)
	pdf.SetXY(x+1, y+1)
	pdf.CellFormat(w-2, 3, "INFORMAÇÕES COMPLEMENTARES", "", 1, "L", false, 0, "")
	pdf.SetFont(string(config.FontType), "", 7)
	pdf.SetXY(x+1, y+5)
	pdf.CellFormat(w-2, 3, longField(optional(value), w-2, 7, string(config.FontType)), "", 1, "L", false, 0, "")
}

func drawBox(pdf *pdfdraw.PDF, x, y, w, h float64, title string, fields []field, config Config) {
	pdf.Rect(x, y, w, h, "")
	if title != "" {
		pdf.SetFont(string(config.FontType), "B", 8)
		pdf.SetXY(x+1, y+1)
		pdf.CellFormat(w-2, 4, title, "", 1, "L", false, 0, "")
	}
	startY := y + 5
	if title == "" {
		startY = y + 1
	}
	colW := w / 4
	rowH := 6.2
	for i, field := range fields {
		col := float64(i % 4)
		row := float64(i / 4)
		fx := x + 1 + col*colW
		fy := startY + row*rowH
		if fy+rowH > y+h {
			return
		}
		pdf.SetXY(fx, fy)
		pdf.SetFont(string(config.FontType), "B", 6)
		pdf.CellFormat(colW-2, 2.5, field.Label, "", 2, "L", false, 0, "")
		pdf.SetFont(string(config.FontType), "", 7)
		pdf.CellFormat(colW-2, 3, longField(field.Value, colW-2, 7, string(config.FontType)), "", 0, "L", false, 0, "")
	}
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
