# DANFSE

DANFSE é o documento auxiliar da Nota Fiscal de Serviços Eletrônica (NFS-e).
Use o pacote `danfse` para renderizar XML de NFS-e em PDF.

## Uso Básico

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

Use `Output(path)` para gravar diretamente em um arquivo, ou `Write(w)` para
enviar o PDF a qualquer `io.Writer`.

## Configuração

`danfse.New` aceita um `*danfse.Config` opcional.

| Campo | Tipo | Padrão | Descrição |
|-------|------|--------|-----------|
| `Margins` | `danfse.Margins` | `5, 5, 5, 5` | Margens da página em milímetros. |
| `DecimalConfig` | `danfse.DecimalConfig` | `4, 4` | Precisão de preço e quantidade. |
| `FontType` | `danfse.FontType` | `danfse.FontTypeTimes` | Fonte principal do PDF. |
| `WatermarkCancelled` | `bool` | `false` | Renderiza a marca d'água de cancelamento. |

## Constantes

```go
danfse.FontTypeCourier
danfse.FontTypeTimes
```

## Configuração Personalizada

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

O CLI aplica as margens configuradas no `config.yaml`.
