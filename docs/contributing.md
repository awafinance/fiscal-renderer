# Contributing

Contributions are welcome. This repository is a native Go rewrite that keeps
the upstream Python source, fixtures, and generated PDFs as parity references.

## Development Setup

1. Clone the repository:

    ```bash
    git clone https://github.com/awafinance/fiscal-renderer.git
    cd fiscal-renderer
    ```

2. Download Go dependencies:

    ```bash
    go mod download
    ```

3. Install Poppler tools if you want the strongest local PDF parity checks:

    ```bash
    # macOS
    brew install poppler

    # Ubuntu/Debian
    sudo apt-get install poppler-utils
    ```

4. Build the CLI:

    ```bash
    go build ./cmd/bfrep
    ```

## Running Tests

Run the complete Go test suite:

```bash
go test ./...
```

Run tests for a specific renderer:

```bash
go test ./danfe
go test ./dacte
go test ./damdfe
go test ./dacce
go test ./danfse
```

The parity tests validate that generated PDFs are valid, match upstream page
counts and page geometry, and cover all files in `tests/generated`. When
`pdfinfo` and `pdftoppm` are available, the renderer tests also rasterize every
page and compare it against the upstream golden PDF with renderer-specific
visual thresholds.

## Code Style

Format Go code before submitting:

```bash
gofmt -w $(rg --files -g '*.go')
```

## Reference Material

Do not remove the upstream Python implementation, `tests/fixtures`, or
`tests/generated` while parity work is still in progress. They are the
authoritative reference for behavior, config defaults, and document coverage.

## Submitting Changes

1. Fork the repository.
2. Create a feature branch.
3. Keep changes scoped to one renderer, shared helper, or docs area when possible.
4. Run `go test ./...` and `go build ./cmd/bfrep`.
5. Push your branch and open a Pull Request.
