# DANFSE

DANFSE is the auxiliary document for an Electronic Service Invoice (NFS-e).
Use the `danfse` package to render NFS-e XML into PDF.

## Basic Usage

```go
package main

import (
	"os"

	"github.com/awafinance/fiscal-renderer/danfse"
)

func main() {
	xmlContent, err := os.ReadFile("nfse.xml")
	if err != nil {
		panic(err)
	}

	doc, err := danfse.New(string(xmlContent), nil)
	if err != nil {
		panic(err)
	}
	if err := doc.Output("danfse.pdf"); err != nil {
		panic(err)
	}
}
```

Use `Output(path)` to write directly to a file, or `Write(w)` to stream the PDF
to any `io.Writer`.

## Configuration

`danfse.New` accepts an optional `*danfse.Config`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Margins` | `danfse.Margins` | `5, 5, 5, 5` | Page margins in millimeters. |
| `DecimalConfig` | `danfse.DecimalConfig` | `4, 4` | Price and quantity precision. |
| `FontType` | `danfse.FontType` | `danfse.FontTypeTimes` | Core PDF font. |
| `WatermarkCancelled` | `bool` | `false` | Render the cancelled watermark. |

## Constants

```go
danfse.FontTypeCourier
danfse.FontTypeTimes
```

## Custom Configuration

```go
cfg := danfse.Config{
	Margins:            danfse.Margins{Top: 2, Right: 2, Bottom: 2, Left: 2},
	DecimalConfig:      danfse.DecimalConfig{PricePrecision: 2, QuantityPrecision: 2},
	FontType:           danfse.FontTypeTimes,
	WatermarkCancelled: true,
}

doc, err := danfse.New(string(xmlContent), &cfg)
if err != nil {
	panic(err)
}
err = doc.Output("danfse.pdf")
```

## CLI

```bash
bfrep danfse /path/to/nfse.xml
```

The CLI applies margin values from `config.yaml`.
