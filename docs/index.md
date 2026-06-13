[![image](https://github.com/awafinance/fiscal-renderer/workflows/tests/badge.svg)](https://github.com/awafinance/fiscal-renderer/actions)
[![image](https://img.shields.io/github/license/awafinance/fiscal-renderer)](https://github.com/awafinance/fiscal-renderer/blob/main/LICENSE)

# Fiscal Renderer

![Fiscal Renderer - XML to PDF](assets/banner.svg)

Native Go library and CLI for generating Brazilian auxiliary fiscal documents
as PDFs from fiscal XML documents.

## Supported Documents

| Document | Description | XML Source |
|----------|-------------|------------|
| [**DANFE**](danfe.md) | Documento Auxiliar da Nota Fiscal Eletrônica | NF-e |
| [**DACCe**](dacce.md) | Documento Auxiliar da Carta de Correção Eletrônica | CC-e |
| [**DACTE**](dacte.md) | Documento Auxiliar do Conhecimento de Transporte Eletrônico | CT-e |
| [**DAMDFE**](damdfe.md) | Documento Auxiliar do Manifesto Eletrônico de Documentos Fiscais | MDF-e |
| [**DANFSE**](danfse.md) | Documento Auxiliar da Nota Fiscal de Serviços Eletrônica | NFS-e |

## Usage Modes

### 1. Go Code

Use the Go packages directly when you need to integrate PDF generation into an
application or service. Each document family exposes a package-level constructor
and configuration struct.

[Get started :material-arrow-right:](getting-started.md){ .md-button }

### 2. CLI

Use `bfrep` for quick PDF generation from the terminal with a simple
`config.yaml` file.

[CLI documentation :material-arrow-right:](cli.md){ .md-button }

## Dependencies

- [go-pdf/fpdf](https://github.com/go-pdf/fpdf) for PDF rendering
- [boombuler/barcode](https://github.com/boombuler/barcode) for Code128 barcodes
- [skip2/go-qrcode](https://github.com/skip2/go-qrcode) for QR codes
- [yaml.v3](https://gopkg.in/yaml.v3) for CLI configuration
