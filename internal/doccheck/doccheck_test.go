package doccheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var userFacingDocs = []string{
	"README.md",
	"docs/about.md",
	"docs/about.pt.md",
	"docs/cli.md",
	"docs/cli.pt.md",
	"docs/contributing.md",
	"docs/contributing.pt.md",
	"docs/dacce.md",
	"docs/dacce.pt.md",
	"docs/dacte.md",
	"docs/dacte.pt.md",
	"docs/damdfe.md",
	"docs/damdfe.pt.md",
	"docs/danfe.md",
	"docs/danfe.pt.md",
	"docs/danfse.md",
	"docs/danfse.pt.md",
	"docs/getting-started.md",
	"docs/getting-started.pt.md",
	"docs/index.md",
	"docs/index.pt.md",
}

func TestUserFacingDocsUseGoModulePath(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, path := range userFacingDocs {
		t.Run(path, func(t *testing.T) {
			content := readDoc(t, root, path)
			for _, stale := range []string{
				"from brazilfiscalreport",
				"pip install brazilfiscalreport",
				"brazilfiscalreport[",
				"site_name: BrazilFiscalReport",
				"https://engenere.github.io/BrazilFiscalReport/",
				"go install github.com/Engenere",
				"go get github.com/Engenere",
			} {
				if strings.Contains(content, stale) {
					t.Fatalf("%s contains stale reference %q", path, stale)
				}
			}
			if path != "README.md" {
				for _, stale := range []string{
					"github.com/Engenere/BrazilFiscalReport",
					"Engenere/BrazilFiscalReport",
				} {
					if strings.Contains(content, stale) {
						t.Fatalf("%s contains stale reference %q", path, stale)
					}
				}
			}
		})
	}
}

func TestReadmeEndsWithUpstreamAttribution(t *testing.T) {
	root := filepath.Join("..", "..")
	content := strings.TrimSpace(readDoc(t, root, "README.md"))
	want := "This is a fork/port to Go of the\n[BrazilFiscalReport](https://github.com/Engenere/BrazilFiscalReport?tab=readme-ov-file)\nproject, which was a fork of the\n[nfe_utils](https://github.com/edsonbernar/nfe_utils) project from Edson\nBernardino."
	if !strings.HasSuffix(content, want) {
		t.Fatalf("README.md does not end with upstream attribution:\n%s", content)
	}
}

func TestPrimaryDocsMentionAwafinanceModule(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, path := range []string{
		"README.md",
		"docs/cli.md",
		"docs/cli.pt.md",
		"docs/getting-started.md",
		"docs/getting-started.pt.md",
	} {
		t.Run(path, func(t *testing.T) {
			content := readDoc(t, root, path)
			if !strings.Contains(content, "github.com/awafinance/fiscal-renderer") {
				t.Fatalf("%s does not mention github.com/awafinance/fiscal-renderer", path)
			}
		})
	}
}

func TestMkDocsConfigUsesAwafinanceIdentity(t *testing.T) {
	root := filepath.Join("..", "..")
	content := readDoc(t, root, "mkdocs.yml")
	for _, expected := range []string{
		"site_name: Fiscal Renderer",
		"site_url: https://awafinance.github.io/fiscal-renderer/",
		"repo_url: https://github.com/awafinance/fiscal-renderer",
		"repo_name: awafinance/fiscal-renderer",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("mkdocs.yml does not mention %s", expected)
		}
	}
	for _, stale := range []string{
		"site_name: BrazilFiscalReport",
		"https://engenere.github.io/BrazilFiscalReport/",
		"github.com/Engenere/BrazilFiscalReport",
		"Engenere/BrazilFiscalReport",
	} {
		if strings.Contains(content, stale) {
			t.Fatalf("mkdocs.yml contains stale reference %q", stale)
		}
	}
}

func TestRootIsGoModuleNotPythonPackage(t *testing.T) {
	root := filepath.Join("..", "..")
	content := readDoc(t, root, "go.mod")
	if !strings.Contains(content, "module github.com/awafinance/fiscal-renderer") {
		t.Fatal("go.mod does not declare module github.com/awafinance/fiscal-renderer")
	}

	for _, removed := range []string{
		"pyproject.toml",
		"requirements.txt",
		"streamlit_app.py",
		".streamlit/config.toml",
		"brazilfiscalreport",
		"tests/__init__.py",
		"tests/conftest.py",
		"tests/test_cli.py",
		"tests/test_dacce.py",
		"tests/test_dacte.py",
		"tests/test_damdfe.py",
		"tests/test_danfe.py",
		"tests/test_danfse.py",
		"tests/test_utils.py",
		"tests/test_xfpdf.py",
		"setup.py",
		"setup.cfg",
		"Pipfile",
		"poetry.lock",
	} {
		if _, err := os.Stat(filepath.Join(root, removed)); !os.IsNotExist(err) {
			t.Fatalf("%s should not exist in the Go module root; stat err=%v", removed, err)
		}
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" || strings.HasSuffix(d.Name(), ".egg-info") {
				t.Fatalf("Python cache/package directory should not exist: %s", rel)
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".py") || strings.HasSuffix(d.Name(), ".pyc") || strings.HasSuffix(d.Name(), ".pyo") {
			t.Fatalf("Python source/cache file should not exist: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGitHubWorkflowUsesGoCI(t *testing.T) {
	root := filepath.Join("..", "..")
	content := readDoc(t, root, ".github/workflows/tests.yml")
	for _, expected := range []string{
		"actions/setup-go@v5",
		"go-version-file: go.mod",
		"poppler-utils",
		"gofmt -l",
		"go test -count=1 ./...",
		"go build -o /tmp/fiscal-renderer-bfrep ./cmd/bfrep",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("tests workflow does not mention %s", expected)
		}
	}
	for _, stale := range []string{
		"actions/setup-python",
		"pytest",
		"codecov",
		"Python 3.",
		"Engenere/BrazilFiscalReport",
		"PyPI",
		"pypi",
	} {
		if strings.Contains(content, stale) {
			t.Fatalf("tests workflow contains stale reference %q", stale)
		}
	}
	for _, removed := range []string{
		".github/workflows/publish.yml",
		".github/workflows/pre-commit.yml",
		"action.yml",
		".pre-commit-config.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, removed)); !os.IsNotExist(err) {
			t.Fatalf("%s should not exist; stat err=%v", removed, err)
		}
	}
}

func TestDocsMentionWriterOutputAPI(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, path := range []string{
		"README.md",
		"docs/dacce.md",
		"docs/dacce.pt.md",
		"docs/dacte.md",
		"docs/dacte.pt.md",
		"docs/damdfe.md",
		"docs/damdfe.pt.md",
		"docs/danfe.md",
		"docs/danfe.pt.md",
		"docs/danfse.md",
		"docs/danfse.pt.md",
		"docs/getting-started.md",
		"docs/getting-started.pt.md",
	} {
		t.Run(path, func(t *testing.T) {
			content := readDoc(t, root, path)
			for _, expected := range []string{"Output", "Write"} {
				if !strings.Contains(content, expected) {
					t.Fatalf("%s does not mention %s", path, expected)
				}
			}
		})
	}
}

func TestDocsMentionByteBackedImageFields(t *testing.T) {
	root := filepath.Join("..", "..")
	requirements := map[string][]string{
		"docs/dacce.md":     {"ImageBytes"},
		"docs/dacce.pt.md":  {"ImageBytes"},
		"docs/dacte.md":     {"LogoBytes"},
		"docs/dacte.pt.md":  {"LogoBytes"},
		"docs/damdfe.md":    {"LogoBytes"},
		"docs/damdfe.pt.md": {"LogoBytes"},
		"docs/danfe.md":     {"LogoBytes", "FooterStamp"},
		"docs/danfe.pt.md":  {"LogoBytes", "FooterStamp"},
	}
	for path, expectedTokens := range requirements {
		t.Run(path, func(t *testing.T) {
			content := readDoc(t, root, path)
			for _, expected := range expectedTokens {
				if !strings.Contains(content, expected) {
					t.Fatalf("%s does not mention %s", path, expected)
				}
			}
		})
	}
}

func readDoc(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
