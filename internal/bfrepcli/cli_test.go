package bfrepcli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awafinance/fiscal-renderer/internal/golden"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &stderr, t.TempDir())
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "bfrep version 0.8.0" {
		t.Fatalf("version output = %q", got)
	}
}

func TestShortVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-v"}, &stdout, &stderr, t.TempDir())
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "bfrep version 0.8.0" {
		t.Fatalf("version output = %q", got)
	}
}

func TestRootHelpMatchesClickShape(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "config.yaml"), []byte(":"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Usage: bfrep [OPTIONS] COMMAND [ARGS]...",
		"-v, --version  Show the version and exit.",
		"--help         Show this message and exit.",
		"Commands:",
		"  dacce",
		"  dacte",
		"  damdfe",
		"  danfe",
		"  danfse",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q in %q", want, output)
		}
	}
}

func TestNoArgsPrintsRootHelpLikeClick(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(nil, &stdout, &stderr, t.TempDir())
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: bfrep [OPTIONS] COMMAND [ARGS]...") ||
		!strings.Contains(stdout.String(), "Commands:") {
		t.Fatalf("root help output missing expected Click shape: %q", stdout.String())
	}
}

func TestCommandHelpDoesNotReadConfigOrXML(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "config.yaml"), []byte(":"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"danfe", "--help"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Usage: bfrep danfe [OPTIONS] XML",
		"Options:",
		"--help  Show this message and exit.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("command help output missing %q in %q", want, output)
		}
	}
}

func TestDanfeCommandWritesPDFWithDefaultConfig(t *testing.T) {
	cwd := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "tests", "fixtures", "danfe", "nfe_test_1.xml"), filepath.Join(cwd, "nfe_test_1.xml"))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"danfe", "nfe_test_1.xml"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Config file 'config.yaml' not found. Using default configuration.") {
		t.Fatalf("missing config message not found in %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "DANFE generated successfully:") {
		t.Fatalf("success message not found in %q", stdout.String())
	}
	assertPDF(t, filepath.Join(cwd, "nfe_test_1.pdf"))
}

func TestDacceCommandWritesPDF(t *testing.T) {
	cwd := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "tests", "fixtures", "dacce", "xml_cce_1.xml"), filepath.Join(cwd, "xml_cce_1.xml"))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dacce", "xml_cce_1.xml"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "DACCe generated successfully:") {
		t.Fatalf("success message not found in %q", stdout.String())
	}
	assertPDF(t, filepath.Join(cwd, "xml_cce_1.pdf"))
}

func TestDacceCommandUsesIssuerFromConfig(t *testing.T) {
	if !golden.PDFTextAvailable() {
		t.Skip("pdftotext not available")
	}
	cwd := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "tests", "fixtures", "dacce", "xml_cce_1.xml"), filepath.Join(cwd, "xml_cce_1.xml"))
	if err := os.WriteFile(filepath.Join(cwd, "config.yaml"), []byte(`
ISSUER:
  nome: "AWAFINANCE LTDA"
  end: "RUA DO PORTO, 500"
  bairro: "CENTRO"
  cep: "01010-000"
  cidade: "SÃO PAULO"
  uf: "SP"
  fone: "(11) 1111-2222"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dacce", "xml_cce_1.xml"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	out := filepath.Join(cwd, "xml_cce_1.pdf")
	assertPDF(t, out)
	text, err := golden.ExtractText(out)
	if err != nil {
		t.Fatal(err)
	}
	text = golden.NormalizeExtractedText(text)
	for _, want := range []string{"AWAFINANCE LTDA", "RUA DO PORTO, 500", "CENTRO", "SÃO PAULO - SP (11) 1111-2222"} {
		if !strings.Contains(text, want) {
			t.Fatalf("configured issuer text missing %q in %q", want, text)
		}
	}
}

func TestDacteCommandWritesPDF(t *testing.T) {
	cwd := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "tests", "fixtures", "dacte", "dacte_test_1.xml"), filepath.Join(cwd, "dacte_test_1.xml"))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dacte", "dacte_test_1.xml"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "DACTE generated successfully:") {
		t.Fatalf("success message not found in %q", stdout.String())
	}
	assertPDF(t, filepath.Join(cwd, "dacte_test_1.pdf"))
}

func TestDamdfeCommandWritesPDF(t *testing.T) {
	cwd := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "tests", "fixtures", "damdfe", "mdf-e_test_1.xml"), filepath.Join(cwd, "mdf-e_test_1.xml"))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"damdfe", "mdf-e_test_1.xml"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "DAMDFE generated successfully:") {
		t.Fatalf("success message not found in %q", stdout.String())
	}
	assertPDF(t, filepath.Join(cwd, "mdf-e_test_1.pdf"))
}

func TestDanfseCommandWritesPDF(t *testing.T) {
	cwd := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "tests", "fixtures", "danfse", "nfse_test_prod.xml"), filepath.Join(cwd, "nfse_test_prod.xml"))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"danfse", "nfse_test_prod.xml"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "DANFSE generated successfully:") {
		t.Fatalf("success message not found in %q", stdout.String())
	}
	assertPDF(t, filepath.Join(cwd, "nfse_test_prod.pdf"))
}

func TestCommandWithAbsoluteXMLWritesOutputToCurrentDirectory(t *testing.T) {
	cwd := t.TempDir()
	sourceDir := t.TempDir()
	xmlPath := filepath.Join(sourceDir, "nfse_test_prod.xml")
	copyFixture(t, filepath.Join("..", "..", "tests", "fixtures", "danfse", "nfse_test_prod.xml"), xmlPath)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"danfse", xmlPath}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	assertPDF(t, filepath.Join(cwd, "nfse_test_prod.pdf"))
	if _, err := os.Stat(filepath.Join(sourceDir, "nfse_test_prod.pdf")); !os.IsNotExist(err) {
		t.Fatalf("absolute XML input should not write PDF beside XML; stat err=%v", err)
	}
}

func TestUnknownCommandDoesNotReadConfigOrXML(t *testing.T) {
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"unknown", "missing.xml"}, &stdout, &stderr, cwd)
	if code != 2 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		"Usage: bfrep [OPTIONS] COMMAND [ARGS]...",
		"Try 'bfrep --help' for help.",
		"Error: No such command 'unknown'.",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("unknown command output missing %q in %q", want, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), "Error reading XML file") ||
		strings.Contains(stdout.String(), "Config file 'config.yaml' not found") {
		t.Fatalf("unexpected config/XML side effect stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestMissingCommandArgumentMatchesClickShape(t *testing.T) {
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"danfe"}, &stdout, &stderr, cwd)
	if code != 2 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		"Usage: bfrep danfe [OPTIONS] XML",
		"Try 'bfrep danfe --help' for help.",
		"Error: Missing argument 'XML'.",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("missing argument output missing %q in %q", want, stderr.String())
		}
	}
}

func TestMissingXMLDoesNotReadConfig(t *testing.T) {
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"danfe", "missing.xml"}, &stdout, &stderr, cwd)
	if code != 2 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		"Usage: bfrep danfe [OPTIONS] XML",
		"Try 'bfrep danfe --help' for help.",
		"Error: Invalid value for 'XML': Path 'missing.xml' does not exist.",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("missing XML output missing %q in %q", want, stderr.String())
		}
	}
}

func TestMissingLogoMessage(t *testing.T) {
	cwd := t.TempDir()
	copyFixture(t, filepath.Join("..", "..", "tests", "fixtures", "dacte", "dacte_test_1.xml"), filepath.Join(cwd, "dacte_test_1.xml"))
	if err := os.WriteFile(filepath.Join(cwd, "config.yaml"), []byte("LOGO: missing.jpg\nTOP_MARGIN: 2\nRIGHT_MARGIN: 3\nBOTTOM_MARGIN: 4\nLEFT_MARGIN: 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dacte", "dacte_test_1.xml"}, &stdout, &stderr, cwd)
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Logo file not found, proceeding without logo.") {
		t.Fatalf("missing logo message not found in %q", stdout.String())
	}
	assertPDF(t, filepath.Join(cwd, "dacte_test_1.pdf"))
}

func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertPDF(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Fatalf("%s is not a PDF", path)
	}
}
