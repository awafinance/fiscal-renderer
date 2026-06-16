[![tests](https://github.com/awafinance/fiscal-renderer/workflows/tests/badge.svg)](https://github.com/awafinance/fiscal-renderer/actions)
[![license](https://img.shields.io/github/license/awafinance/fiscal-renderer)](https://github.com/awafinance/fiscal-renderer/blob/main/LICENSE)
[![contributors](https://img.shields.io/github/contributors/awafinance/fiscal-renderer)](https://github.com/awafinance/fiscal-renderer/graphs/contributors)

# Fiscal Renderer

Native Go library and blazing-fast CLI for generating Brazilian auxiliary
fiscal documents as PDFs from fiscal XML documents, optimized for high-volume
rendering workloads.

> Biblioteca e CLI nativos em Go para gerar **DANFE**, **DACTE**,
> **DAMDFE**, **DACCe** e **DANFSE** em PDF a partir de XML de NF-e, CT-e,
> MDF-e, CC-e e NFS-e.

## Supported Documents

| Document | Description | XML Source |
|----------|-------------|------------|
| **DANFE** | Documento Auxiliar da Nota Fiscal Eletrônica | NF-e |
| **DACCe** | Documento Auxiliar da Carta de Correção Eletrônica | CC-e |
| **DACTE** | Documento Auxiliar do Conhecimento de Transporte Eletrônico | CT-e |
| **DAMDFE** | Documento Auxiliar do Manifesto Eletrônico de Documentos Fiscais | MDF-e |
| **DANFSE** | Documento Auxiliar da Nota Fiscal de Serviços Eletrônica | NFS-e |

## Installation

```bash
go get github.com/awafinance/fiscal-renderer
go install github.com/awafinance/fiscal-renderer/cmd/bfrep@latest
```

## Quick Start

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

The same constructor/output pattern is available in the `danfe`, `dacce`,
`dacte`, `damdfe`, and `danfse` packages.

Use `doc.Output(path)` to write directly to a file, or `doc.Write(w)` to stream
the PDF to any `io.Writer`, such as an HTTP response or in-memory buffer.

## Footer note

Every document family accepts an optional `FooterStamp` — a thin rule plus an
optional logo and note drawn at the bottom of each page. It is entirely
caller-driven; the library ships no default text. Leave it unset for no footer.

The note supports markdown-ish inline formatting: `**bold**`, `*italic*`, and
`[label](url)` clickable links. Use `\` to escape a literal `*`, `_`, `[`, or `\`.

```go
doc, _ := danfe.New(xml, &danfe.Config{
	FooterStamp: danfe.FooterStamp{
		Text: "Gerado por **Acme** — [acme.com](https://acme.com)",
	},
})
```

The same `FooterStamp` field is available on the `danfe`, `dacte`, `damdfe`,
`dacce`, and `danfse` `Config` types.

## CLI

Generate PDFs directly from the terminal:

```bash
bfrep danfe /path/to/nfe.xml
bfrep dacte /path/to/cte.xml
bfrep damdfe /path/to/mdfe.xml
bfrep dacce /path/to/cce.xml
bfrep danfse /path/to/nfse.xml
```

The CLI reads `config.yaml` from the current directory, writes the output PDF to
the current directory, supports `--version` and `-v`, and preserves the upstream
logo, margin, and DACCe issuer configuration behavior.

## Performance

This Go port is designed for low-latency rendering and fast command-line
startup. Benchmarks on Apple M5 Pro with Go 1.26.1 and Python 3.14 showed:

API median, with XML already in memory and rendering to bytes:

| Case | Python | Go | Speedup |
|------|--------|----|---------|
| Production NF-e DANFE | 37.0 ms | 0.7 ms | 52.6x |
| NF-e one-page | 29.5 ms | 0.7 ms | 44.2x |
| NF-e multipage | 173.5 ms | 1.8 ms | 98.4x |
| CT-e DACTE | 20.6 ms | 0.9 ms | 22.7x |
| MDF-e DAMDFE | 21.9 ms | 1.0 ms | 22.4x |
| NFS-e DANFSE | 20.1 ms | 2.0 ms | 10.2x |

CLI median, including subprocess startup, XML file read, and PDF file write:

| Case | Python CLI | Go CLI | Speedup |
|------|------------|--------|---------|
| Production NF-e DANFE | 207.7 ms | 4.8 ms | 43.6x |
| NF-e one-page | 228.4 ms | 4.4 ms | 51.7x |
| NF-e multipage | 343.6 ms | 6.4 ms | 54.0x |
| CT-e DACTE | 201.7 ms | 5.8 ms | 34.6x |
| MDF-e DAMDFE | 211.2 ms | 4.7 ms | 45.3x |
| NFS-e DANFSE | 228.7 ms | 6.2 ms | 36.9x |

## Dependencies

- [go-pdf/fpdf](https://github.com/go-pdf/fpdf) for PDF rendering
- [boombuler/barcode](https://github.com/boombuler/barcode) for Code128 barcodes
- [skip2/go-qrcode](https://github.com/skip2/go-qrcode) for QR codes
- [yaml.v3](https://gopkg.in/yaml.v3) for CLI configuration

## Upstream Reference

This is a fork/port to Go of the
[BrazilFiscalReport](https://github.com/Engenere/BrazilFiscalReport?tab=readme-ov-file)
project, which was a fork of the
[nfe_utils](https://github.com/edsonbernar/nfe_utils) project from Edson
Bernardino.
