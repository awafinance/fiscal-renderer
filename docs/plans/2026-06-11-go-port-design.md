# Native Go Port Design

Date: 2026-06-11

## Context

This repository is a clone of `Engenere/BrazilFiscalReport` at upstream commit
`9bd69f3`. The current implementation is a Python package that generates
Brazilian fiscal auxiliary documents as PDFs from XML:

- DANFE from NF-e XML
- DACCe from CC-e XML
- DACTE from CT-e XML
- DAMDFE from MDF-e XML
- DANFSE from NFS-e XML

The Go port must be a native Go library and CLI, not a wrapper around the
Python implementation. The Python source, docs, fixtures, and generated PDFs
remain in the repository during the port as authoritative reference material.

## Goals

- Provide a Go module with public packages matching the document families:
  `danfe`, `dacce`, `dacte`, `damdfe`, and `danfse`.
- Preserve upstream CLI behavior for `bfrep danfe|dacce|dacte|damdfe|danfse
  <xml>`.
- Preserve upstream configuration semantics, defaults, exported enum values,
  and document-specific options.
- Preserve document layout and rendering behavior against the existing
  fixture/golden-PDF matrix.
- Preserve English and Portuguese documentation coverage while adapting API
  examples from Python to Go.

## Non-Goals

- Do not shell out to Python or embed Python as the runtime renderer.
- Do not replace the fiscal document layouts with HTML templates.
- Do not broaden fiscal XML support beyond the upstream project while the
  parity port is incomplete.
- Do not remove the upstream Python reference until the Go parity gates are
  covered by tests and docs.

## Public API Shape

Each document package exposes a constructor or render function that accepts XML
content and an optional config struct. The API should feel idiomatic in Go while
remaining traceable to the Python public surface.

Expected package-level concepts:

- `danfe`: `Config`, `Margins`, `DecimalConfig`, `TaxConfiguration`,
  `InvoiceDisplay`, `FontType`, `FontSize`, `ReceiptPosition`,
  `ProductDescriptionConfig`, `FooterStamp`, and PDF output helpers.
- `dacte`: `Config`, `Margins`, `DecimalConfig`, `FontType`, `ReceiptPosition`,
  `ModalType`, watermark cancellation, and IBS/CBS display support.
- `damdfe`: `Config`, `Margins`, `DecimalConfig`, `FontType`, `ModalType`,
  `EmissionType`, logo support, and origin/destination prestação display.
- `danfse`: `Config`, `Margins`, `DecimalConfig`, `FontType`, and cancelled
  watermark support.
- `dacce`: XML content plus optional issuer information and optional logo image.

Every config default must match the Python dataclass defaults unless a Go API
requires a zero-value-compatible wrapper. Where the zero value cannot represent
the upstream default, provide `DefaultConfig()` and make constructors normalize
missing fields.

## CLI Design

The Go CLI binary is `bfrep`.

Commands:

- `bfrep danfe <xml>`
- `bfrep dacce <xml>`
- `bfrep dacte <xml>`
- `bfrep damdfe <xml>`
- `bfrep danfse <xml>`
- `bfrep --version` and `bfrep -v`

Behavior to preserve from `brazilfiscalreport/cli.py`:

- Read `config.yaml` from the current working directory.
- Print the missing-config message and use defaults when `config.yaml` is not
  present.
- Write output to the current working directory using the input XML stem and
  `.pdf` suffix.
- Resolve `LOGO` paths relative to the current working directory and proceed
  without a logo when the file is missing.
- Apply `TOP_MARGIN`, `RIGHT_MARGIN`, `BOTTOM_MARGIN`, and `LEFT_MARGIN` to
  DANFE, DACTE, DAMDFE, and DANFSE.
- Use the default DACCe issuer when `ISSUER` is absent.

## Internal Architecture

Shared packages should keep renderer packages focused on document semantics:

- `internal/xmlutil`: namespace-aware XML lookup, optional text extraction,
  date/time extraction, and safe missing-tag behavior.
- `internal/fiscalfmt`: CPF/CNPJ, CEP, phone, fiscal number, decimal,
  measurement, chunking, merge, and text-limit formatting helpers.
- `internal/pdfdraw`: thin wrapper around the selected PDF backend for margins,
  line/cell/text primitives, CP1252-compatible core font behavior, text boxes,
  long-field fitting, page setup, and deterministic metadata hooks.
- `internal/barcode`: Code128 rendering for DANFE and DACCe access keys.
- `internal/qrcode`: QR code rendering for DACTE, DAMDFE, and DANFSE.
- `internal/images`: logo loading from paths and byte slices, image type
  detection, and fitting into bounding boxes.
- `internal/golden`: test helpers for writing deterministic PDFs and comparing
  them to `tests/generated`.

The renderer packages should port behavior renderer by renderer, preserving the
same section boundaries that exist in the Python implementation where that helps
traceability.

## PDF Backend

Use a native Go PDF backend with an FPDF-style drawing model. The initial target
is `github.com/go-pdf/fpdf` because the upstream implementation is built on
FPDF2 primitives such as pages, margins, cells, multi-cells, rectangles, lines,
text, images, and font selection.

`qpdf` or `pdfcpu` may be used by tests to normalize or inspect generated PDFs,
but they are not the primary rendering engine.

## Data Flow

1. Public package receives XML content and config.
2. Config is normalized to upstream defaults.
3. XML is parsed into a lightweight document model or accessed through shared
   namespace-aware helpers.
4. The renderer computes document-specific fields, watermarks, table splits,
   page breaks, and mode-specific sections.
5. Shared PDF primitives write deterministic PDF output to an `io.Writer` or
   file path.
6. CLI commands read XML/config files, call the same public package APIs, and
   write the output PDF.

## Error Handling

Python often allows missing XML fields to render as blank strings. The Go port
should preserve that behavior for optional fields, while returning explicit
errors for invalid XML, unreadable files, unsupported image payloads, and output
write failures.

Renderer methods should return errors instead of panicking. Panics are acceptable
only for programmer errors inside tests.

## Documentation

Documentation parity covers the existing docs:

- `docs/index.md` and `docs/index.pt.md`
- `docs/getting-started.md` and `docs/getting-started.pt.md`
- `docs/cli.md` and `docs/cli.pt.md`
- `docs/danfe.md` and `docs/danfe.pt.md`
- `docs/dacce.md` and `docs/dacce.pt.md`
- `docs/dacte.md` and `docs/dacte.pt.md`
- `docs/damdfe.md` and `docs/damdfe.pt.md`
- `docs/danfse.md`
- `docs/about*.md`, `docs/changelog*.md`, and `docs/contributing*.md`

The docs should be updated incrementally as each Go package and CLI command
becomes functional.

## Test and Parity Gates

The existing Python tests define the initial parity matrix:

- 27 DANFE golden-PDF scenarios
- 15 DACTE golden-PDF scenarios
- 11 DAMDFE golden-PDF scenarios
- 4 DANFSE golden-PDF scenarios
- 1 DACCe golden-PDF scenario
- CLI command coverage for `dacce`, `danfe`, `dacte`, and `damdfe`
- utility formatting tests
- core-font encoding regression tests

The repository currently contains 42 XML/image fixtures in `tests/fixtures` and
57 expected PDFs in `tests/generated`. Each renderer is complete only when its
matching Go golden tests cover every upstream generated PDF for that renderer.

PDF equality can be stricter or looser by phase:

1. Initial renderer slice: generated PDF exists, has valid pages, includes key
   text/barcode/QR/image content, and passes targeted structural checks.
2. Parity slice: normalized PDF comparison through `qpdf` or `pdfcpu` passes
   against the expected file, with deterministic metadata and IDs filtered.
3. Completion audit: all 57 golden PDFs pass, CLI behavior is covered, and docs
   reflect the Go API and CLI.

## Implementation Order

1. Establish Go module, shared config/default patterns, CLI shell, and shared
   helper packages.
2. Port formatting and XML helpers with direct unit tests.
3. Port PDF drawing primitives and deterministic PDF test harness.
4. Port DACCe first because it is a single-page renderer with barcode and logo
   support.
5. Port DANFSE next because it exercises QR code, watermark, fiscal formatting,
   and larger structured blocks.
6. Port DAMDFE, including modal-specific sections and dynamic page layout.
7. Port DACTE, including all transport modes, multi-page behavior, watermarks,
   and IBS/CBS display.
8. Port DANFE last because it has the broadest public config surface and the
   largest golden matrix.
9. Update docs and release metadata after each renderer reaches its parity gate.

## Completion Criteria

The objective is complete only when current evidence proves:

- The repository builds as a Go module.
- The public Go library exposes all five fiscal document families.
- The Go `bfrep` CLI implements all five commands and version output.
- CLI config semantics match the Python implementation.
- Existing fixtures are preserved.
- All 57 upstream golden PDFs have corresponding passing Go parity tests.
- Docs are updated for Go usage in English and Portuguese where upstream docs
  exist.
- No required runtime dependency on Python remains for the Go library or CLI.
