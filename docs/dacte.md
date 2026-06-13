# DACTE

DACTE is the auxiliary document for an Electronic Transport Knowledge document
(CT-e). Use the `dacte` package to render CT-e XML into PDF.

## Basic Usage

```go
package main

import (
	"os"

	"github.com/awafinance/fiscal-renderer/dacte"
)

func main() {
	xmlContent, err := os.ReadFile("cte.xml")
	if err != nil {
		panic(err)
	}

	doc, err := dacte.New(string(xmlContent), nil)
	if err != nil {
		panic(err)
	}
	if err := doc.Output("dacte.pdf"); err != nil {
		panic(err)
	}
}
```

Use `Output(path)` to write directly to a file, or `Write(w)` to stream the PDF
to any `io.Writer`.

## Configuration

`dacte.New` accepts an optional `*dacte.Config`. Empty fields are normalized to
the upstream defaults.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Logo` | `string` | empty | Optional logo image path. |
| `LogoBytes` | `[]byte` | empty | Optional in-memory logo image bytes. Takes precedence over `Logo`. |
| `Margins` | `dacte.Margins` | `5, 5, 5, 5` | Page margins in millimeters. |
| `ReceiptPosition` | `dacte.ReceiptPosition` | `dacte.ReceiptPositionTop` | Receipt placement. |
| `DecimalConfig` | `dacte.DecimalConfig` | `4, 4` | Price and quantity precision. |
| `FontType` | `dacte.FontType` | `dacte.FontTypeTimes` | Core PDF font. |
| `WatermarkCancelled` | `bool` | `false` | Render the cancelled watermark. |
| `DisplayIBSCBS` | `bool` | `false` | Render IBS/CBS tax fields when present. |

## Constants

```go
dacte.FontTypeCourier
dacte.FontTypeTimes

dacte.ReceiptPositionTop
dacte.ReceiptPositionBottom
dacte.ReceiptPositionLeft

dacte.ModalTypeRodoviario
dacte.ModalTypeAereo
dacte.ModalTypeAquaviario
dacte.ModalTypeFerroviario
dacte.ModalTypeDutoviario
dacte.ModalTypeMultimodal
```

## Custom Configuration

```go
cfg := dacte.Config{
	Logo:            "logo.png",
	Margins:         dacte.Margins{Top: 10, Right: 10, Bottom: 10, Left: 10},
	ReceiptPosition: dacte.ReceiptPositionTop,
	DecimalConfig:   dacte.DecimalConfig{PricePrecision: 2, QuantityPrecision: 3},
	FontType:        dacte.FontTypeCourier,
	DisplayIBSCBS:   true,
}

doc, err := dacte.New(string(xmlContent), &cfg)
if err != nil {
	panic(err)
}
err = doc.Output("dacte.pdf")
```

## CLI

```bash
bfrep dacte /path/to/cte.xml
```

The CLI applies `LOGO` and margin values from `config.yaml`.
