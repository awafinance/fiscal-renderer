# Contribuindo

Contribuições são bem-vindas. Este repositório é uma reescrita nativa em Go que
mantém o código Python upstream, fixtures e PDFs gerados como referências de
paridade.

## Configuração do Ambiente de Desenvolvimento

1. Clone o repositório:

    ```bash
    git clone https://github.com/awafinance/fiscal-renderer.git
    cd fiscal-renderer
    ```

2. Baixe as dependências Go:

    ```bash
    go mod download
    ```

3. Instale as ferramentas Poppler se quiser executar as verificações locais mais fortes de paridade de PDF:

    ```bash
    # macOS
    brew install poppler

    # Ubuntu/Debian
    sudo apt-get install poppler-utils
    ```

4. Compile o CLI:

    ```bash
    go build ./cmd/bfrep
    ```

## Executando Testes

Execute a suíte completa de testes Go:

```bash
go test ./...
```

Execute os testes de um renderer específico:

```bash
go test ./danfe
go test ./dacte
go test ./damdfe
go test ./dacce
go test ./danfse
```

Os testes de paridade validam que os PDFs gerados são válidos, têm a mesma
quantidade de páginas e a mesma geometria dos PDFs dourados upstream, além de
cobrir todos os arquivos em `tests/generated`. Quando `pdfinfo` e `pdftoppm`
estão disponíveis, os testes dos renderers também rasterizam todas as páginas e
comparam o resultado com o PDF dourado usando limites visuais específicos para
cada renderer.

## Estilo de Código

Formate o código Go antes de enviar:

```bash
gofmt -w $(rg --files -g '*.go')
```

## Material de Referência

Não remova a implementação Python upstream, `tests/fixtures` ou
`tests/generated` enquanto o trabalho de paridade estiver em andamento. Eles são
a referência autoritativa para comportamento, padrões de configuração e
cobertura dos documentos.

## Enviando Alterações

1. Faça um fork do repositório.
2. Crie uma branch para sua alteração.
3. Mantenha as mudanças focadas em um renderer, helper compartilhado ou área de documentação quando possível.
4. Execute `go test ./...` e `go build ./cmd/bfrep`.
5. Faça push da branch e abra um Pull Request.
