[![image](https://github.com/awafinance/fiscal-renderer/workflows/tests/badge.svg)](https://github.com/awafinance/fiscal-renderer/actions)
[![image](https://img.shields.io/github/license/awafinance/fiscal-renderer)](https://github.com/awafinance/fiscal-renderer/blob/main/LICENSE)

# Fiscal Renderer

![Fiscal Renderer - XML para PDF](assets/banner.svg)

Biblioteca e CLI nativos em Go para geração de documentos fiscais auxiliares
brasileiros em PDF a partir de documentos XML.

## Documentos Suportados

| Documento | Descrição | Origem XML |
|-----------|-----------|------------|
| [**DANFE**](danfe.md) | Documento Auxiliar da Nota Fiscal Eletrônica | NF-e |
| [**DACCe**](dacce.md) | Documento Auxiliar da Carta de Correção Eletrônica | CC-e |
| [**DACTE**](dacte.md) | Documento Auxiliar do Conhecimento de Transporte Eletrônico | CT-e |
| [**DAMDFE**](damdfe.md) | Documento Auxiliar do Manifesto Eletrônico de Documentos Fiscais | MDF-e |
| [**DANFSE**](danfse.md) | Documento Auxiliar da Nota Fiscal de Serviços Eletrônica | NFS-e |

## Modos de Uso

### 1. Código Go

Use os pacotes Go diretamente quando precisar integrar a geração de PDFs em uma
aplicação ou serviço. Cada família de documento expõe um construtor e uma struct
de configuração.

[Começar :material-arrow-right:](getting-started.md){ .md-button }

### 2. CLI

Use `bfrep` para geração rápida de PDF pelo terminal com um arquivo
`config.yaml` simples.

[Documentação do CLI :material-arrow-right:](cli.md){ .md-button }

## Dependências

- [go-pdf/fpdf](https://github.com/go-pdf/fpdf) para renderização de PDF
- [boombuler/barcode](https://github.com/boombuler/barcode) para códigos de barras Code128
- [skip2/go-qrcode](https://github.com/skip2/go-qrcode) para QR codes
- [yaml.v3](https://gopkg.in/yaml.v3) para configuração do CLI
