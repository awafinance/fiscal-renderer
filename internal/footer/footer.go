// Package footer renders the optional footer stamp shared by every fiscal
// document family (DANFE, DACTE, DAMDFE, DACCe, DANFSE). The stamp is a thin
// rule, an optional logo, and an optional note that supports markdown-ish
// inline formatting. It is fully caller-driven: the library hardcodes no text.
package footer

import (
	"strings"

	"github.com/awafinance/fiscal-renderer/internal/images"
	"github.com/awafinance/fiscal-renderer/internal/pdfdraw"
	"github.com/go-pdf/fpdf"
)

// Stamp is the configurable footer note. The zero value draws nothing.
type Stamp struct {
	Logo      string
	LogoBytes []byte
	// Text is the footer note. It supports markdown-ish inline formatting:
	// **bold**, *italic*, and [label](url) links. Use \ to escape a literal
	// *, _, [, or \.
	Text         string
	Height       float64
	LogoMaxWidth float64
	Spacing      float64
}

// Default returns the layout defaults applied when a stamp is configured
// without explicit sizing.
func Default() Stamp {
	return Stamp{Height: 5, LogoMaxWidth: 20, Spacing: 1}
}

// Active reports whether the stamp would draw anything.
func (s Stamp) Active() bool {
	return s.Logo != "" || len(s.LogoBytes) > 0 || strings.TrimSpace(s.Text) != ""
}

// IsZero reports whether the stamp is entirely unset.
func (s Stamp) IsZero() bool {
	return s.Logo == "" && len(s.LogoBytes) == 0 && s.Text == "" &&
		s.Height == 0 && s.LogoMaxWidth == 0 && s.Spacing == 0
}

// Reserve is the extra bottom margin the footer occupies, or 0 when inactive.
// Callers add this to their bottom margin so document content leaves room.
func (s Stamp) Reserve() float64 {
	if !s.Active() {
		return 0
	}
	return s.Height + s.Spacing
}

// Normalize fills unset sizing fields from d (a Default()), leaving an entirely
// unset stamp as the zero value so it stays inactive.
func (s Stamp) Normalize(d Stamp) Stamp {
	if s.IsZero() {
		return s
	}
	if s.Height == 0 {
		s.Height = d.Height
	}
	if s.LogoMaxWidth == 0 {
		s.LogoMaxWidth = d.LogoMaxWidth
	}
	if s.Spacing == 0 {
		s.Spacing = d.Spacing
	}
	return s
}

// Draw renders the stamp along the bottom of the current page. imageName is the
// internal key used to register a logo from bytes (any value unique within the
// document). fontSize is the already-resolved point size for the note text.
func Draw(pdf *pdfdraw.PDF, s Stamp, imageName string, marginLeft, marginRight, marginBottom float64, fontType string, fontSize float64) {
	if !s.Active() {
		return
	}
	pageW, pageH := pdf.GetPageSize()
	y := pageH - marginBottom - s.Height
	x := marginLeft
	w := pageW - marginLeft - marginRight
	pdf.SetDrawColor(180, 180, 180)
	pdf.Line(x, y-s.Spacing, x+w, y-s.Spacing)
	pdf.SetDrawColor(0, 0, 0)

	// Resolve the logo (if any), then center the logo+text group on the page.
	logoImage, logoW := "", 0.0
	if len(s.LogoBytes) > 0 {
		logoImage, logoW = imageName, s.LogoMaxWidth
	} else if s.Logo != "" {
		if imageType, _ := images.TypeFromFile(s.Logo); imageType != "" {
			logoImage, logoW = s.Logo, s.LogoMaxWidth
		}
	}
	spans := parseMarkdown(s.Text)
	textW := spansWidth(pdf, fontType, fontSize, spans)
	gap := 0.0
	if logoW > 0 && textW > 0 {
		gap = 2
	}
	cursor := x + (w-logoW-gap-textW)/2

	if len(s.LogoBytes) > 0 {
		pdf.ImageBytes(logoImage, s.LogoBytes, cursor, y, logoW, 0)
	} else if logoImage != "" {
		imageType, _ := images.TypeFromFile(logoImage)
		pdf.ImageOptions(logoImage, cursor, y, logoW, 0, false, fpdf.ImageOptions{ImageType: imageType}, 0, "")
	}
	if logoW > 0 {
		cursor += logoW + gap
	}
	if textW > 0 {
		drawSpans(pdf, spans, fontType, fontSize, cursor, y+1, s.Height)
	}
}

// spansWidth measures the rendered width of the spans at the given font.
func spansWidth(pdf *pdfdraw.PDF, fontType string, fontSize float64, spans []span) float64 {
	total := 0.0
	for _, sp := range spans {
		pdf.SetFont(fontType, sp.style, fontSize)
		total += pdf.GetStringWidth(pdf.Encode(sp.text))
	}
	return total
}

// drawSpans renders the markdown-ish spans inline, starting at (x, y).
func drawSpans(pdf *pdfdraw.PDF, spans []span, fontType string, fontSize, x, y, h float64) {
	pdf.SetXY(x, y)
	for _, sp := range spans {
		pdf.SetFont(fontType, sp.style, fontSize)
		if sp.link != "" {
			pdf.SetTextColor(0, 0, 238)
			pdf.WriteLinkString(h, pdf.Encode(sp.text), sp.link)
			pdf.SetTextColor(0, 0, 0)
		} else {
			pdf.Write(h, pdf.Encode(sp.text))
		}
	}
}

// span is one run of footer text with uniform styling.
type span struct {
	text  string
	style string // fpdf font style: "", "B", "I", or "BI"
	link  string // non-empty => clickable URL
}

// parseMarkdown turns a single line into styled spans. Supported syntax:
// **bold**, *italic* (also __ and _), [label](url) links, and \ escapes.
func parseMarkdown(s string) []span {
	var spans []span
	var buf strings.Builder
	bold, italic := false, false
	style := func() string {
		switch {
		case bold && italic:
			return "BI"
		case bold:
			return "B"
		case italic:
			return "I"
		}
		return ""
	}
	flush := func() {
		if buf.Len() > 0 {
			spans = append(spans, span{text: buf.String(), style: style()})
			buf.Reset()
		}
	}
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\\' && i+1 < len(r):
			buf.WriteRune(r[i+1])
			i++
		case (c == '*' || c == '_') && i+1 < len(r) && r[i+1] == c:
			flush()
			bold = !bold
			i++
		case c == '*' || c == '_':
			flush()
			italic = !italic
		case c == '[':
			if label, url, n := parseLink(r[i:]); n > 0 {
				flush()
				spans = append(spans, span{text: label, style: style(), link: url})
				i += n - 1
				continue
			}
			buf.WriteRune(c)
		default:
			buf.WriteRune(c)
		}
	}
	flush()
	return spans
}

// parseLink matches [label](url) at the start of r, returning the label, url,
// and number of runes consumed (0 if r is not a link).
func parseLink(r []rune) (label, url string, n int) {
	if len(r) == 0 || r[0] != '[' {
		return "", "", 0
	}
	closeBracket := -1
	for i := 1; i < len(r); i++ {
		if r[i] == ']' {
			closeBracket = i
			break
		}
	}
	if closeBracket < 0 || closeBracket+1 >= len(r) || r[closeBracket+1] != '(' {
		return "", "", 0
	}
	for i := closeBracket + 2; i < len(r); i++ {
		if r[i] == ')' {
			return string(r[1:closeBracket]), string(r[closeBracket+2 : i]), i + 1
		}
	}
	return "", "", 0
}
