# DAMDFE

DAMDFE is the auxiliary document for an Electronic Manifest of Fiscal Documents
(MDF-e). Use the `damdfe` package to render MDF-e XML into PDF.

## Basic Usage

```go
package main

import (
	"os"

	"github.com/awafinance/fiscal-renderer/damdfe"
)

func main() {
	xmlContent, err := os.ReadFile("mdfe.xml")
	if err != nil {
		panic(err)
	}

	doc, err := damdfe.New(string(xmlContent), nil)
	if err != nil {
		panic(err)
	}
	if err := doc.Output("damdfe.pdf"); err != nil {
		panic(err)
	}
}
```

Use `Output(path)` to write directly to a file, or `Write(w)` to stream the PDF
to any `io.Writer`.

## Configuration

`damdfe.New` accepts an optional `*damdfe.Config`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Logo` | `string` | empty | Optional logo image path. |
| `LogoBytes` | `[]byte` | empty | Optional in-memory logo image bytes. Takes precedence over `Logo`. |
| `Margins` | `damdfe.Margins` | `5, 5, 5, 5` | Page margins in millimeters. |
| `DecimalConfig` | `damdfe.DecimalConfig` | `4, 4` | Price and quantity precision. |
| `FontType` | `damdfe.FontType` | `damdfe.FontTypeTimes` | Core PDF font. |
| `DisplayOrigemDestinoPrestacao` | `bool` | `false` | Render origin/destination prestação route fields. |

## Constants

```go
damdfe.FontTypeCourier
damdfe.FontTypeTimes

damdfe.ModalTypeRodoviario
damdfe.ModalTypeAereo
damdfe.ModalTypeAquaviario
damdfe.ModalTypeFerroviario

damdfe.EmissionTypeNormal
damdfe.EmissionTypeContingencia
```

## Custom Configuration

```go
cfg := damdfe.Config{
	Logo:                          "logo.png",
	Margins:                       damdfe.Margins{Top: 10, Right: 10, Bottom: 10, Left: 10},
	DecimalConfig:                 damdfe.DecimalConfig{PricePrecision: 2, QuantityPrecision: 3},
	FontType:                      damdfe.FontTypeTimes,
	DisplayOrigemDestinoPrestacao: true,
}

doc, err := damdfe.New(string(xmlContent), &cfg)
if err != nil {
	panic(err)
}
err = doc.Output("damdfe.pdf")
```

## CLI

```bash
bfrep damdfe /path/to/mdfe.xml
```

The CLI applies `LOGO` and margin values from `config.yaml`.
