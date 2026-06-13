Gere documentos DANFE, DACCe, DACTE, DAMDFE e DANFSE diretamente pelo terminal.
O PDF é salvo no diretório atual usando o nome do XML com extensão `.pdf`.
Você pode criar um arquivo `config.yaml` com dados do emitente, logo e margens.

## Instalação

Instale o CLI nativo em Go com:

```bash
go install github.com/awafinance/fiscal-renderer/cmd/bfrep@latest
```

## Versão

Use a opção `--version` ou `-v` para verificar a versão instalada:

```bash
bfrep --version
bfrep -v
```

## Comandos

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

## Arquivo de Configuração

Crie um arquivo `config.yaml` no diretório onde você executa o comando.

#### Exemplo de `config.yaml`

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

`ISSUER` é usado apenas pelo comando `dacce`. `LOGO` é usado por `danfe`,
`dacte` e `damdfe`. As margens se aplicam a `danfe`, `dacte`, `damdfe` e
`danfse`. Se nenhum `config.yaml` for encontrado, os valores padrão são usados.
