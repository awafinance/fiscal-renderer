Generate DANFE, DACCe, DACTE, DAMDFE, and DANFSE documents directly from the terminal.
The PDF is saved in the current directory using the XML filename with a `.pdf`
extension. You can create a `config.yaml` file with issuer details, logo, and
margin configuration.

## Installation

Install the native Go CLI with:

```bash
go install github.com/awafinance/fiscal-renderer/cmd/bfrep@latest
```

## Version

Use the `--version` or `-v` option to check the installed version:

```bash
bfrep --version
bfrep -v
```

## Commands

### DANFE

```bash
bfrep danfe /path/to/nfe.xml
```

### DACCe

```bash
bfrep dacce /path/to/cce.xml
```

### DACTE

```bash
bfrep dacte /path/to/cte.xml
```

### DAMDFE

```bash
bfrep damdfe /path/to/mdfe.xml
```

### DANFSE

```bash
bfrep danfse /path/to/nfse.xml
```

## Configuration File

Create a `config.yaml` file in the directory where you run the command.

#### Example `config.yaml`

```yaml
ISSUER:
  nome: "EMPRESA LTDA"
  end: "AV. TEST, 100"
  bairro: "CENTRO"
  cep: "01010-000"
  cidade: "SÃO PAULO"
  uf: "SP"
  fone: "(11) 1234-5678"

LOGO: "/path/to/logo.jpg"
TOP_MARGIN: 5.0
RIGHT_MARGIN: 5.0
BOTTOM_MARGIN: 5.0
LEFT_MARGIN: 5.0
```

`ISSUER` is used only by the `dacce` command. `LOGO` is used by `danfe`,
`dacte`, and `damdfe`. The margin settings apply to `danfe`, `dacte`,
`damdfe`, and `danfse`. If no `config.yaml` is found, default values are used.
