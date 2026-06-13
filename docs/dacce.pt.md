# DACCe

DACCe é o documento auxiliar da Carta de Correção Eletrônica (CC-e).
Use o pacote `dacce` para renderizar um XML de CC-e em PDF.

## Uso Básico

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

Use `Output(path)` para gravar diretamente em um arquivo, ou `Write(w)` para
enviar o PDF a qualquer `io.Writer`.

## Configuração

`dacce.New` aceita um `*dacce.Config` opcional.

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `Issuer` | `dacce.Issuer` | Dados do emitente impressos no cabeçalho. Se vazio, `dacce.DefaultIssuer()` é usado. |
| `Image` | `string` | Caminho opcional para o logo. |
| `ImageBytes` | `[]byte` | Bytes opcionais do logo em memória. Tem precedência sobre `Image`. |

`dacce.Issuer` contém:

| Campo | Descrição |
|-------|-----------|
| `Name` | Nome do emitente |
| `Address` | Logradouro e número |
| `Neighborhood` | Bairro |
| `CEP` | CEP |
| `City` | Cidade |
| `UF` | Estado |
| `Phone` | Telefone |

## Emitente e Logo Personalizados

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

O CLI lê `ISSUER` do `config.yaml`. Se a seção não existir, o emitente padrão
do DACCe é usado.
