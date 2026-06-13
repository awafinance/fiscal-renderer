# Getting Started

## Installation

### Library

Add the native Go module to your project:

```bash
go get github.com/awafinance/fiscal-renderer
```

### CLI

Install the `bfrep` command with:

```bash
go install github.com/awafinance/fiscal-renderer/cmd/bfrep@latest
```

## Quick Start

### Using Go code

Generate a DANFE PDF from an NF-e XML file in a few lines:

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

The same pattern applies to all document types:

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

`Output(path)` writes the PDF to a filesystem path. Use `Write(w)` when you
already have an `io.Writer`, such as an HTTP response or a `bytes.Buffer`.

### Using the CLI

For quick generation from the terminal:

```bash
bfrep danfe /path/to/nfe.xml
bfrep dacce /path/to/cce.xml
bfrep dacte /path/to/cte.xml
bfrep damdfe /path/to/mdfe.xml
bfrep danfse /path/to/nfse.xml
```

See the [CLI documentation](cli.md) for configuration options.

## Next steps

- Learn about customization options for each document type: [DANFE](danfe.md), [DACTE](dacte.md), [DAMDFE](damdfe.md), [DACCe](dacce.md), [DANFSE](danfse.md)
- Configure the [CLI](cli.md) for batch generation
