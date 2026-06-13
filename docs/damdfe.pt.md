# DAMDFE

DAMDFE é o documento auxiliar do Manifesto Eletrônico de Documentos Fiscais
(MDF-e). Use o pacote `damdfe` para renderizar XML de MDF-e em PDF.

## Uso Básico

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

Use `Output(path)` para gravar diretamente em um arquivo, ou `Write(w)` para
enviar o PDF a qualquer `io.Writer`.

## Configuração

`damdfe.New` aceita um `*damdfe.Config` opcional.

| Campo | Tipo | Padrão | Descrição |
|-------|------|--------|-----------|
| `Logo` | `string` | vazio | Caminho opcional para o logo. |
| `LogoBytes` | `[]byte` | vazio | Bytes opcionais do logo em memória. Tem precedência sobre `Logo`. |
| `Margins` | `damdfe.Margins` | `5, 5, 5, 5` | Margens da página em milímetros. |
| `DecimalConfig` | `damdfe.DecimalConfig` | `4, 4` | Precisão de preço e quantidade. |
| `FontType` | `damdfe.FontType` | `damdfe.FontTypeTimes` | Fonte principal do PDF. |
| `DisplayOrigemDestinoPrestacao` | `bool` | `false` | Renderiza campos de origem/destino da prestação. |

## Constantes

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

## Configuração Personalizada

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

O CLI aplica `LOGO` e as margens configuradas no `config.yaml`.
