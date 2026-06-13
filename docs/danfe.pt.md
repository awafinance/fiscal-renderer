# DANFE

DANFE é o documento auxiliar da Nota Fiscal Eletrônica (NF-e). Use o pacote
`danfe` para renderizar XML de NF-e em PDF.

## Uso Básico

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

Use `Output(path)` para gravar diretamente em um arquivo, ou `Write(w)` para
enviar o PDF a qualquer `io.Writer`.

## Configuração

`danfe.New` aceita um `*danfe.Config` opcional. Campos vazios são normalizados
para os padrões do projeto upstream.

| Campo | Tipo | Padrão | Descrição |
|-------|------|--------|-----------|
| `Logo` | `string` | vazio | Caminho opcional para o logo. |
| `LogoBytes` | `[]byte` | vazio | Bytes opcionais do logo em memória. Tem precedência sobre `Logo`. |
| `Margins` | `danfe.Margins` | `5, 5, 5, 5` | Margens da página em milímetros. |
| `ReceiptPosition` | `danfe.ReceiptPosition` | `danfe.ReceiptPositionTop` | Posição do recibo. |
| `DecimalConfig` | `danfe.DecimalConfig` | `4, 4` | Precisão de preço e quantidade. |
| `TaxConfiguration` | `danfe.TaxConfiguration` | `danfe.TaxConfigurationStandardICMSIPI` | Modo da tabela de impostos dos produtos. |
| `InvoiceDisplay` | `danfe.InvoiceDisplay` | `danfe.InvoiceDisplayFullDetails` | Modo de exibição de fatura/duplicatas. |
| `FontType` | `danfe.FontType` | `danfe.FontTypeTimes` | Fonte principal do PDF. |
| `FontSize` | `danfe.FontSize` | `danfe.FontSizeSmall` | Multiplicador do tamanho da fonte. |
| `DisplayPISCOFINS` | `bool` | `false` | Inclui colunas e totais de PIS/COFINS. |
| `WatermarkCancelled` | `bool` | `false` | Renderiza a marca d'água de cancelamento. |
| `InfCplSemicolonNewline` | `bool` | `false` | Quebra informações adicionais em ponto e vírgula. |
| `ProductDescriptionConfig` | `danfe.ProductDescriptionConfig` | informações adicionais habilitadas | Controla detalhes extras na descrição dos produtos. |
| `FooterStamp` | `danfe.FooterStamp` | altura `5`, largura máx. `20`, espaçamento `1` | Rodapé opcional com logo e texto. |

## Constantes

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

## Opções da Descrição dos Produtos

`danfe.ProductDescriptionConfig` controla textos extras anexados à descrição de
cada produto.

| Campo | Descrição |
|-------|-----------|
| `DisplayBranch` | Inclui informações de lote/rastro. |
| `DisplayANP` | Inclui informações de combustível ANP. |
| `DisplayANVISA` | Inclui informações de medicamento ANVISA. |
| `BranchInfoPrefix` | Prefixo usado antes de linhas de lote/rastro. |
| `DisplayAdditionalInfo` | Inclui o texto de `infAdProd`. Habilitado por padrão. |

## Rodapé

`danfe.FooterStamp` pode renderizar um pequeno rodapé em todas as páginas.

| Campo | Descrição |
|-------|-----------|
| `Logo` | Caminho opcional para o logo do rodapé. |
| `LogoBytes` | Bytes opcionais do logo do rodapé em memória. Tem precedência sobre `Logo`. |
| `Text` | Texto opcional do rodapé. |
| `Height` | Altura do rodapé em milímetros. |
| `LogoMaxWidth` | Largura máxima do logo. |
| `Spacing` | Espaço entre o conteúdo e o rodapé. |

## Configuração Personalizada

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

O CLI aplica `LOGO` e as margens configuradas no `config.yaml`.
