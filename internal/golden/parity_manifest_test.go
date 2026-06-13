package golden

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/awafinance/fiscal-renderer/dacce"
	"github.com/awafinance/fiscal-renderer/dacte"
	"github.com/awafinance/fiscal-renderer/damdfe"
	"github.com/awafinance/fiscal-renderer/danfe"
	"github.com/awafinance/fiscal-renderer/danfse"
)

var coveredGoldenPDFs = map[string]struct{}{
	"dacce/cce.pdf": {},

	"dacte/dacte_default.pdf":                          {},
	"dacte/dacte_default_aereo.pdf":                    {},
	"dacte/dacte_default_aquaviario.pdf":               {},
	"dacte/dacte_default_dutoviario.pdf":               {},
	"dacte/dacte_default_ferroviario.pdf":              {},
	"dacte/dacte_default_logo.pdf":                     {},
	"dacte/dacte_default_multimodal.pdf":               {},
	"dacte/dacte_multi_pages.pdf":                      {},
	"dacte/dacte_overload.pdf":                         {},
	"dacte/dacte_reforma_tributaria.pdf":               {},
	"dacte/dacte_tomador_outros.pdf":                   {},
	"dacte/dacte_watermark_cancelled_homologation.pdf": {},
	"dacte/dacte_watermark_cancelled_production.pdf":   {},
	"dacte/dacte_watermark_homologation_only.pdf":      {},
	"dacte/dacte_without_compl.pdf":                    {},

	"damdfe/damdfe_aereo.pdf":                    {},
	"damdfe/damdfe_aereo_contingencia.pdf":       {},
	"damdfe/damdfe_aquaviario.pdf":               {},
	"damdfe/damdfe_default.pdf":                  {},
	"damdfe/damdfe_default_cte.pdf":              {},
	"damdfe/damdfe_default_logo.pdf":             {},
	"damdfe/damdfe_default_logo_margins.pdf":     {},
	"damdfe/damdfe_ferroviario.pdf":              {},
	"damdfe/damdfe_multi_municipio.pdf":          {},
	"damdfe/damdfe_no_authorization.pdf":         {},
	"damdfe/damdfe_origem_destino_prestacao.pdf": {},

	"danfe/danfe_add_info_below_prod.pdf":            {},
	"danfe/danfe_add_info_next_page.pdf":             {},
	"danfe/danfe_anp.pdf":                            {},
	"danfe/danfe_anvisa.pdf":                         {},
	"danfe/danfe_big_font_size.pdf":                  {},
	"danfe/danfe_branch.pdf":                         {},
	"danfe/danfe_branch_with_prefix.pdf":             {},
	"danfe/danfe_default.pdf":                        {},
	"danfe/danfe_default_production.pdf":             {},
	"danfe/danfe_duplicatas_only.pdf":                {},
	"danfe/danfe_footer_stamp.pdf":                   {},
	"danfe/danfe_footer_stamp_logo_only.pdf":         {},
	"danfe/danfe_footer_stamp_multipage.pdf":         {},
	"danfe/danfe_footer_stamp_text_only.pdf":         {},
	"danfe/danfe_infcpl_semicolon_newline.pdf":       {},
	"danfe/danfe_mei.pdf":                            {},
	"danfe/danfe_minimal.pdf":                        {},
	"danfe/danfe_multipage_landscape.pdf":            {},
	"danfe/danfe_overload.pdf":                       {},
	"danfe/danfe_pis_confins.pdf":                    {},
	"danfe/danfe_reforma_tributaria.pdf":             {},
	"danfe/danfe_retirada.pdf":                       {},
	"danfe/danfe_retirada_entrega.pdf":               {},
	"danfe/danfe_sn.pdf":                             {},
	"danfe/danfe_with_production_environment.pdf":    {},
	"danfe/danfe_without_production_environment.pdf": {},

	"danfse/danfse_cancelled_hom.pdf":  {},
	"danfse/danfse_cancelled_prod.pdf": {},
	"danfse/danfse_default_hom.pdf":    {},
	"danfse/danfse_default_prod.pdf":   {},
}

func TestGoldenPDFManifestCoversAllGeneratedPDFs(t *testing.T) {
	generatedRoot := filepath.Join("..", "..", "tests", "generated")
	var generated []string
	err := filepath.WalkDir(generatedRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".pdf" {
			return nil
		}
		rel, err := filepath.Rel(generatedRoot, path)
		if err != nil {
			return err
		}
		generated = append(generated, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(generated)
	if len(generated) != 57 {
		t.Fatalf("generated PDF count = %d, want 57", len(generated))
	}
	var missing []string
	for _, path := range generated {
		if _, ok := coveredGoldenPDFs[path]; !ok {
			missing = append(missing, path)
		}
	}
	var stale []string
	for path := range coveredGoldenPDFs {
		if !contains(generated, path) {
			stale = append(stale, path)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 || len(stale) > 0 {
		t.Fatalf("golden manifest mismatch: missing=%v stale=%v", missing, stale)
	}
}

func TestAllXMLFixturesRenderWithNativeGoAPI(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "tests", "fixtures")
	xmlFixtures := collectFixtureXMLs(t, fixtureRoot)
	if len(xmlFixtures) != 41 {
		t.Fatalf("XML fixture count = %d, want 41: %v", len(xmlFixtures), xmlFixtures)
	}
	renderers := map[string]func(string) ([]byte, error){
		"dacce":  renderDACCe,
		"dacte":  renderDACTE,
		"damdfe": renderDAMDFE,
		"danfe":  renderDANFE,
		"danfse": renderDANFSE,
	}
	for _, rel := range xmlFixtures {
		t.Run(rel, func(t *testing.T) {
			family := strings.Split(rel, "/")[0]
			render, ok := renderers[family]
			if !ok {
				t.Fatalf("no renderer registered for fixture family %q", family)
			}
			data, err := os.ReadFile(filepath.Join(fixtureRoot, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			pdfBytes, err := render(string(data))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
				t.Fatalf("rendered output is not a PDF: %q", string(pdfBytes[:min(len(pdfBytes), 32)]))
			}
		})
	}
}

func collectFixtureXMLs(t *testing.T, fixtureRoot string) []string {
	t.Helper()
	var fixtures []string
	err := filepath.WalkDir(fixtureRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}
		rel, err := filepath.Rel(fixtureRoot, path)
		if err != nil {
			return err
		}
		fixtures = append(fixtures, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(fixtures)
	return fixtures
}

func contains(values []string, target string) bool {
	i := sort.SearchStrings(values, target)
	return i < len(values) && values[i] == target
}

func renderDACCe(xml string) ([]byte, error) {
	doc, err := dacce.New(xml, nil)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := doc.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderDACTE(xml string) ([]byte, error) {
	doc, err := dacte.New(xml, nil)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := doc.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderDAMDFE(xml string) ([]byte, error) {
	doc, err := damdfe.New(xml, nil)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := doc.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderDANFE(xml string) ([]byte, error) {
	doc, err := danfe.New(xml, nil)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := doc.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderDANFSE(xml string) ([]byte, error) {
	doc, err := danfse.New(xml, nil)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := doc.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
