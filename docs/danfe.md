# DANFE

DANFE is the auxiliary document for an Electronic Invoice (NF-e). Use the
`danfe` package to render NF-e XML into PDF.

## Basic Usage

```go
package main

import (
	"os"

	"github.com/awafinance/fiscal-renderer/danfe"
)

func main() {
	xmlContent, err := os.ReadFile("nfe.xml")
	if err != nil {
		panic(err)
	}

	doc, err := danfe.New(string(xmlContent), nil)
	if err != nil {
		panic(err)
	}
	if err := doc.Output("danfe.pdf"); err != nil {
		panic(err)
	}
}
```

Use `Output(path)` to write directly to a file, or `Write(w)` to stream the PDF
to any `io.Writer`.

## Configuration

`danfe.New` accepts an optional `*danfe.Config`. Empty fields are normalized to
the upstream defaults.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Logo` | `string` | empty | Optional logo image path. |
| `LogoBytes` | `[]byte` | empty | Optional in-memory logo image bytes. Takes precedence over `Logo`. |
| `Margins` | `danfe.Margins` | `5, 5, 5, 5` | Page margins in millimeters. |
| `ReceiptPosition` | `danfe.ReceiptPosition` | `danfe.ReceiptPositionTop` | Receipt placement. |
| `DecimalConfig` | `danfe.DecimalConfig` | `4, 4` | Price and quantity precision. |
| `TaxConfiguration` | `danfe.TaxConfiguration` | `danfe.TaxConfigurationStandardICMSIPI` | Product tax table layout mode. |
| `InvoiceDisplay` | `danfe.InvoiceDisplay` | `danfe.InvoiceDisplayFullDetails` | Invoice/duplicata display mode. |
| `FontType` | `danfe.FontType` | `danfe.FontTypeTimes` | Core PDF font. |
| `FontSize` | `danfe.FontSize` | `danfe.FontSizeSmall` | Font size multiplier. |
| `DisplayPISCOFINS` | `bool` | `false` | Include PIS/COFINS columns and totals. |
| `WatermarkCancelled` | `bool` | `false` | Render the cancelled watermark. |
| `InfCplSemicolonNewline` | `bool` | `false` | Split additional information at semicolons. |
| `ProductDescriptionConfig` | `danfe.ProductDescriptionConfig` | additional info enabled | Controls extra product description details. |
| `FooterStamp` | `danfe.FooterStamp` | height `5`, logo max width `20`, spacing `1` | Optional footer stamp with logo and text. |

## Constants

```go
danfe.TaxConfigurationStandardICMSIPI
danfe.TaxConfigurationICMSST
danfe.TaxConfigurationWithoutIPI

danfe.InvoiceDisplayDuplicatesOnly
danfe.InvoiceDisplayFullDetails

danfe.FontTypeCourier
danfe.FontTypeTimes

danfe.FontSizeSmall
danfe.FontSizeBig

danfe.ReceiptPositionTop
danfe.ReceiptPositionBottom
danfe.ReceiptPositionLeft
```

## Product Description Options

`danfe.ProductDescriptionConfig` controls extra text appended to each product
description.

| Field | Description |
|-------|-------------|
| `DisplayBranch` | Include batch/rastro information. |
| `DisplayANP` | Include ANP fuel information. |
| `DisplayANVISA` | Include ANVISA medication information. |
| `BranchInfoPrefix` | Prefix used before batch/rastro lines. |
| `DisplayAdditionalInfo` | Include `infAdProd` text. Enabled by default. |

## Footer Stamp

`danfe.FooterStamp` can render a small footer on every page.

| Field | Description |
|-------|-------------|
| `Logo` | Optional footer logo path. |
| `LogoBytes` | Optional in-memory footer logo bytes. Takes precedence over `Logo`. |
| `Text` | Optional footer text. |
| `Height` | Footer height in millimeters. |
| `LogoMaxWidth` | Maximum logo width. |
| `Spacing` | Space between content and footer. |

## Custom Configuration

```go
cfg := danfe.Config{
	Logo:            "logo.png",
	Margins:         danfe.Margins{Top: 2, Right: 2, Bottom: 2, Left: 2},
	ReceiptPosition: danfe.ReceiptPositionBottom,
	DecimalConfig:   danfe.DecimalConfig{PricePrecision: 3, QuantityPrecision: 2},
	TaxConfiguration: danfe.TaxConfigurationICMSST,
	InvoiceDisplay:  danfe.InvoiceDisplayFullDetails,
	FontType:        danfe.FontTypeTimes,
	FontSize:        danfe.FontSizeBig,
	DisplayPISCOFINS: true,
	ProductDescriptionConfig: danfe.ProductDescriptionConfig{
		DisplayBranch:         true,
		DisplayANP:            true,
		DisplayANVISA:         true,
		BranchInfoPrefix:      "=>",
		DisplayAdditionalInfo: false,
	},
	FooterStamp: danfe.FooterStamp{
		Logo: "logo.png",
		Text: "Powered by Fiscal Renderer",
	},
}

doc, err := danfe.New(string(xmlContent), &cfg)
if err != nil {
	panic(err)
}
err = doc.Output("danfe.pdf")
```

## CLI

```bash
bfrep danfe /path/to/nfe.xml
```

The CLI applies `LOGO` and margin values from `config.yaml`.
