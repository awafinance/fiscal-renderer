package pdfdraw

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/awafinance/fiscal-renderer/internal/images"
	"github.com/awafinance/fiscal-renderer/internal/qrcode"
	"github.com/go-pdf/fpdf"
)

type Margins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

func DefaultMargins() Margins {
	return Margins{Top: 5, Right: 5, Bottom: 5, Left: 5}
}

type Document struct {
	pdf *PDF
}

type PDF struct {
	*fpdf.Fpdf
	translate func(string) string
}

func NewPDF(orientationStr, unitStr, sizeStr, fontDirStr string) *PDF {
	return Wrap(fpdf.New(orientationStr, unitStr, sizeStr, fontDirStr))
}

func Wrap(pdf *fpdf.Fpdf) *PDF {
	return &PDF{
		Fpdf:      pdf,
		translate: pdf.UnicodeTranslatorFromDescriptor(""),
	}
}

func (p *PDF) Text(x, y float64, txtStr string) {
	p.Fpdf.Text(x, y, p.Encode(txtStr))
}

func (p *PDF) MultiCell(w, h float64, txtStr, borderStr, alignStr string, fill bool) {
	p.Fpdf.MultiCell(w, h, p.Encode(txtStr), borderStr, alignStr, fill)
}

func (p *PDF) CellFormat(w, h float64, txtStr, borderStr string, ln int, alignStr string, fill bool, link int, linkStr string) {
	p.Fpdf.CellFormat(w, h, p.Encode(txtStr), borderStr, ln, alignStr, fill, link, linkStr)
}

func (p *PDF) Cell(w, h float64, txtStr string) {
	p.Fpdf.Cell(w, h, p.Encode(txtStr))
}

func (p *PDF) Encode(txtStr string) string {
	if txtStr == "" || p.translate == nil {
		return txtStr
	}
	return p.translate(txtStr)
}

func (p *PDF) ImageBytes(name string, data []byte, x, y, w, h float64) bool {
	imageType := images.TypeFromBytes(data)
	if imageType == "" {
		return false
	}
	p.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: imageType}, bytes.NewReader(data))
	p.ImageOptions(name, x, y, w, h, false, fpdf.ImageOptions{ImageType: imageType}, 0, "")
	return true
}

func (p *PDF) QRCode(data string, x, y, size float64, borderModules int) error {
	if data == "" {
		return nil
	}
	if size <= 0 {
		return fmt.Errorf("QR code size must be positive")
	}
	if borderModules < 0 {
		return fmt.Errorf("QR code border modules cannot be negative")
	}
	bitmap, err := qrcode.BitmapWithoutBorder(data)
	if err != nil {
		return err
	}
	totalModules := len(bitmap) + 2*borderModules
	if totalModules <= 0 {
		return fmt.Errorf("empty QR bitmap")
	}
	moduleSize := size / float64(totalModules)

	p.SetFillColor(255, 255, 255)
	p.Rect(x, y, size, size, "F")
	p.SetFillColor(0, 0, 0)
	for row, modules := range bitmap {
		for col, black := range modules {
			if !black {
				continue
			}
			p.Rect(
				x+float64(col+borderModules)*moduleSize,
				y+float64(row+borderModules)*moduleSize,
				moduleSize,
				moduleSize,
				"F",
			)
		}
	}
	p.SetFillColor(255, 255, 255)
	return nil
}

func New(title string, margins Margins) *Document {
	pdf := NewPDF("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetTitle(title, false)
	pdf.SetMargins(margins.Left, margins.Top, margins.Right)
	pdf.SetAutoPageBreak(false, margins.Bottom)
	pdf.AddPage()
	pdf.SetFont("Times", "", 10)
	return &Document{pdf: pdf}
}

func (d *Document) PDF() *PDF {
	return d.pdf
}

func (d *Document) WriteReferencePage(title string, fields map[string]string) {
	d.pdf.SetFont("Times", "B", 14)
	d.pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
	d.pdf.Ln(2)
	d.pdf.SetFont("Times", "", 9)
	for label, value := range fields {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		d.pdf.SetFont("Times", "B", 8)
		d.pdf.CellFormat(35, 5, label, "", 0, "L", false, 0, "")
		d.pdf.SetFont("Times", "", 8)
		d.pdf.MultiCell(0, 5, value, "", "L", false)
	}
}

func (d *Document) Output(w io.Writer) error {
	return d.pdf.Output(w)
}

func RenderReference(w io.Writer, title string, margins Margins, fields map[string]string) error {
	doc := New(title, margins)
	doc.WriteReferencePage(title, fields)
	return doc.Output(w)
}
