# DACTE

DACTE é o documento auxiliar do Conhecimento de Transporte Eletrônico (CT-e).
Use o pacote `dacte` para renderizar XML de CT-e em PDF.

## Uso Básico

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

Use `Output(path)` para gravar diretamente em um arquivo, ou `Write(w)` para
enviar o PDF a qualquer `io.Writer`.

## Configuração

`dacte.New` aceita um `*dacte.Config` opcional. Campos vazios são normalizados
para os padrões do projeto upstream.

| Campo | Tipo | Padrão | Descrição |
|-------|------|--------|-----------|
| `Logo` | `string` | vazio | Caminho opcional para o logo. |
| `LogoBytes` | `[]byte` | vazio | Bytes opcionais do logo em memória. Tem precedência sobre `Logo`. |
| `Margins` | `dacte.Margins` | `5, 5, 5, 5` | Margens da página em milímetros. |
| `ReceiptPosition` | `dacte.ReceiptPosition` | `dacte.ReceiptPositionTop` | Posição do recibo. |
| `DecimalConfig` | `dacte.DecimalConfig` | `4, 4` | Precisão de preço e quantidade. |
| `FontType` | `dacte.FontType` | `dacte.FontTypeTimes` | Fonte principal do PDF. |
| `WatermarkCancelled` | `bool` | `false` | Renderiza a marca d'água de cancelamento. |
| `DisplayIBSCBS` | `bool` | `false` | Renderiza campos IBS/CBS quando presentes. |

## Constantes

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

## Configuração Personalizada

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

O CLI aplica `LOGO` e as margens configuradas no `config.yaml`.
