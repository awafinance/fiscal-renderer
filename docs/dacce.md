# DACCe

DACCe is the auxiliary document for an Electronic Correction Letter (CC-e).
Use the `dacce` package to render a CC-e XML into a PDF.

## Basic Usage

```go
package main

import (
	"os"

	"github.com/awafinance/fiscal-renderer/dacce"
)

func main() {
	xmlContent, err := os.ReadFile("cce.xml")
	if err != nil {
		panic(err)
	}

	doc, err := dacce.New(string(xmlContent), nil)
	if err != nil {
		panic(err)
	}
	if err := doc.Output("dacce.pdf"); err != nil {
		panic(err)
	}
}
```

Use `Output(path)` to write directly to a file, or `Write(w)` to stream the PDF
to any `io.Writer`.

## Configuration

`dacce.New` accepts an optional `*dacce.Config`.

| Field | Type | Description |
|-------|------|-------------|
| `Issuer` | `dacce.Issuer` | Issuer information printed in the header. If empty, `dacce.DefaultIssuer()` is used. |
| `Image` | `string` | Optional logo image path. |
| `ImageBytes` | `[]byte` | Optional in-memory logo image bytes. Takes precedence over `Image`. |

`dacce.Issuer` contains:

| Field | Description |
|-------|-------------|
| `Name` | Issuer name |
| `Address` | Street and number |
| `Neighborhood` | District or neighborhood |
| `CEP` | Postal code |
| `City` | City |
| `UF` | State |
| `Phone` | Phone number |

## Custom Issuer and Logo

```go
cfg := dacce.Config{
	Issuer: dacce.Issuer{
		Name:         "EMPRESA LTDA",
		Address:      "AV. TEST, 100",
		Neighborhood: "CENTRO",
		CEP:          "01010-000",
		City:         "SAO PAULO",
		UF:           "SP",
		Phone:        "(11) 1234-5678",
	},
	Image: "logo.png",
}

doc, err := dacce.New(string(xmlContent), &cfg)
if err != nil {
	panic(err)
}
err = doc.Output("dacce.pdf")
```

## CLI

```bash
bfrep dacce /path/to/cce.xml
```

The CLI reads `ISSUER` from `config.yaml`. If it is missing, the default DACCe
issuer is used.
