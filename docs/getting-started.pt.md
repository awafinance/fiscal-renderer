# Começando

## Instalação

### Biblioteca

Adicione o módulo Go nativo ao seu projeto:

```bash
go get github.com/awafinance/fiscal-renderer
```

### CLI

Instale o comando `bfrep` com:

```bash
go install github.com/awafinance/fiscal-renderer/cmd/bfrep@latest
```

## Início Rápido

### Usando código Go

Gere um DANFE em PDF a partir de um arquivo XML de NF-e em poucas linhas:

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

O mesmo padrão se aplica a todos os tipos de documentos:

=== "DANFE"

    ```go
    doc, err := danfe.New(string(xmlContent), nil)
    if err != nil {
    	panic(err)
    }
    err = doc.Output("danfe.pdf")
    ```

=== "DACCe"

    ```go
    doc, err := dacce.New(string(xmlContent), nil)
    if err != nil {
    	panic(err)
    }
    err = doc.Output("dacce.pdf")
    ```

=== "DACTE"

    ```go
    doc, err := dacte.New(string(xmlContent), nil)
    if err != nil {
    	panic(err)
    }
    err = doc.Output("dacte.pdf")
    ```

=== "DAMDFE"

    ```go
    doc, err := damdfe.New(string(xmlContent), nil)
    if err != nil {
    	panic(err)
    }
    err = doc.Output("damdfe.pdf")
    ```

=== "DANFSE"

    ```go
    doc, err := danfse.New(string(xmlContent), nil)
    if err != nil {
    	panic(err)
    }
    err = doc.Output("danfse.pdf")
    ```

`Output(path)` grava o PDF em um caminho no sistema de arquivos. Use `Write(w)`
quando já tiver um `io.Writer`, como uma resposta HTTP ou um `bytes.Buffer`.

### Usando o CLI

Para geração rápida pelo terminal:

```bash
bfrep danfe /path/to/nfe.xml
bfrep dacce /path/to/cce.xml
bfrep dacte /path/to/cte.xml
bfrep damdfe /path/to/mdfe.xml
bfrep danfse /path/to/nfse.xml
```

Veja a [documentação do CLI](cli.md) para opções de configuração.

## Próximos passos

- Conheça as opções de personalização para cada tipo de documento: [DANFE](danfe.md), [DACTE](dacte.md), [DAMDFE](damdfe.md), [DACCe](dacce.md), [DANFSE](danfse.md)
- Configure o [CLI](cli.md) para geração em lote
