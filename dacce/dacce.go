package dacce

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/awafinance/fiscal-renderer/internal/barcode"
	"github.com/awafinance/fiscal-renderer/internal/fiscalfmt"
	"github.com/awafinance/fiscal-renderer/internal/footer"
	"github.com/awafinance/fiscal-renderer/internal/images"
	"github.com/awafinance/fiscal-renderer/internal/pdfdraw"
	"github.com/awafinance/fiscal-renderer/internal/xmlutil"
	"github.com/go-pdf/fpdf"
)

type Issuer struct {
	Name         string
	Address      string
	Neighborhood string
	CEP          string
	City         string
	UF           string
	Phone        string
}

func DefaultIssuer() Issuer {
	return Issuer{
		Name:         "EMPRESA LTDA",
		Address:      "AV. TEST, 100",
		Neighborhood: "TEST",
		CEP:          "88888-88",
		City:         "SÃO PAULO",
		UF:           "SP",
		Phone:        "(11) 1234-5678",
	}
}

// FooterStamp is the optional marketing/footer note drawn at the bottom of the
// page. Its Text field supports markdown-ish formatting (**bold**, *italic*,
// [label](url)). The zero value draws nothing.
type FooterStamp = footer.Stamp

type Config struct {
	Issuer      Issuer
	Image       string
	ImageBytes  []byte
	FooterStamp FooterStamp
}

func DefaultConfig() Config {
	return Config{Issuer: DefaultIssuer()}
}

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
	normalized := DefaultConfig()
	if config != nil {
		normalized = *config
		if normalized.Issuer == (Issuer{}) {
			normalized.Issuer = DefaultIssuer()
		}
		normalized.FooterStamp = normalized.FooterStamp.Normalize(footer.Default())
	}
	return &Document{XML: xml, Config: normalized, root: root}, nil
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
	pdf := pdfdraw.NewPDF("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetAutoPageBreak(false, 10)
	pdf.SetTitle("DACCe", false)
	pdf.AddPageFormat("P", fpdf.SizeType{Wd: 210, Ht: 297})
	d.draw(pdf, root)
	// ponytail: DACCe uses a fixed 10mm layout margin (not configurable), so
	// the footer matches it rather than reading a Margins field.
	footer.Draw(pdf, d.Config.FooterStamp, "dacce-footer-logo", 10, 10, 10, "Helvetica", 6)
	return pdf.Output(w)
}

func RenderFile(xml string, path string, config *Config) error {
	doc, err := New(xml, config)
	if err != nil {
		return err
	}
	return doc.Output(path)
}

func (d *Document) draw(pdf *pdfdraw.PDF, root *xmlutil.Node) {
	detEvent := root.Find("detEvento")
	infEvent := root.Find("infEvento")
	retEvent := root.Find("retEvento")
	infRetEvent := retEvent.Find("infEvento")

	pdf.Rect(10, 10, 190, 33, "")
	pdf.Line(90, 10, 90, 43)

	issuerName := d.Config.Issuer.Name
	issuerText := strings.Join([]string{
		d.Config.Issuer.Address,
		d.Config.Issuer.Neighborhood,
		fmt.Sprintf("%s - %s %s", d.Config.Issuer.City, d.Config.Issuer.UF, d.Config.Issuer.Phone),
	}, "\n")

	col := 11.0
	colEnd := 24.0
	width := 80.0
	if len(d.Config.ImageBytes) > 0 {
		col = 23
		colEnd = 28
		width = 67
		pdf.ImageBytes("dacce-logo", d.Config.ImageBytes, 12, 12, 12, 0)
	} else if d.Config.Image != "" {
		col = 23
		colEnd = 28
		width = 67
		imageType, _ := images.TypeFromFile(d.Config.Image)
		pdf.ImageOptions(d.Config.Image, 12, 12, 12, 0, false, fpdf.ImageOptions{ImageType: imageType}, 0, "")
	}

	pdf.SetXY(col, 16)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.MultiCell(width, 4, issuerName, "", "C", false)
	pdf.SetXY(11, colEnd)
	pdf.SetFont("Helvetica", "", 8)
	pdf.MultiCell(80, 4, issuerText, "", "C", false)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.Text(118, 16, "Representação Gráfica de CC-e")
	pdf.SetFont("Helvetica", "I", 9)
	pdf.Text(123, 20, "(Carta de Correção Eletrônica)")

	pdf.SetFont("Helvetica", "", 8)
	pdf.Text(92, 30, "ID do Evento: "+strings.TrimPrefix(infEvent.Attr("Id"), "ID"))
	eventDate, eventHour := fiscalfmt.DateUTC(xmlutil.Text(infEvent, "dhEvento"))
	pdf.Text(92, 35, fmt.Sprintf("Criado em: %s %s", eventDate, eventHour))
	regDate, regHour := fiscalfmt.DateUTC(xmlutil.Text(infRetEvent, "dhRegEvento"))
	protocol := xmlutil.Text(infRetEvent, "nProt")
	pdf.Text(92, 40, fmt.Sprintf("Protocolo: %s - Registrado na SEFAZ em: %s %s", protocol, regDate, regHour))

	pdf.Rect(10, 47, 190, 50, "")
	pdf.Line(10, 83, 200, 83)
	pdf.SetXY(11, 48)
	pdf.SetFont("Helvetica", "", 8)
	pdf.MultiCell(185, 4, "De acordo com as determinações legais vigentes, vimos por meio desta comunicar-lhe que a Nota Fiscal, abaixo referenciada, contêm irregularidades que estão destacadas e suas respectivas correções, solicitamos que sejam aplicadas essas correções ao executar seus lançamentos fiscais.", "", "L", false)

	key := xmlutil.Text(infEvent, "chNFe")
	d.drawBarcode(pdf, key)

	pdf.SetFont("Helvetica", "", 7)
	pdf.Text(130, 78, strings.Join(fiscalfmt.Chunks(key, 4), " "))

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Text(12, 71, "CNPJ Destinatário:  "+fiscalfmt.FormatCPFCNPJ(xmlutil.Text(infRetEvent, "CNPJDest")))
	pdf.Text(12, 76, fmt.Sprintf("Nota Fiscal: %s - Série: %s", invoiceNumber(key), slice(key, 22, 25)))

	pdf.SetXY(11, 84)
	pdf.SetFont("Helvetica", "I", 7)
	pdf.MultiCell(185, 3, xmlutil.Text(detEvent, "xCondUso"), "", "L", false)

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Text(11, 103, "CORREÇÕES A SEREM CONSIDERADAS")
	pdf.Rect(10, 104, 190, 170, "")

	pdf.SetXY(11, 106)
	pdf.MultiCell(185, 4, xmlutil.Text(detEvent, "xCorrecao"), "", "L", false)

	pdf.SetXY(11, 265)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.MultiCell(185, 4, "Este documento é uma representação gráfica da CC-e e foi impresso apenas para sua informação e não possue validade fiscal.\nA CC-e deve ser recebida e mantida em arquivo eletrônico XML e pode ser consultada através dos portais das SEFAZ.", "", "C", false)
}

func (d *Document) drawBarcode(pdf *pdfdraw.PDF, key string) {
	if key == "" {
		return
	}
	pngBytes, err := barcode.Code128PNG(key, 365, 40)
	if err != nil {
		return
	}
	name := "dacce-code128-" + key
	pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(pngBytes))
	pdf.ImageOptions(name, 127, 60, 73, 8, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

func invoiceNumber(key string) string {
	raw := slice(key, 25, 34)
	n, err := strconv.Atoi(raw)
	if err != nil {
		return raw
	}
	padded := fmt.Sprintf("%09d", n)
	return strings.Join(fiscalfmt.Chunks(padded, 3), ".")
}

func slice(s string, start, end int) string {
	if start < 0 || start >= len(s) || end <= start {
		return ""
	}
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
