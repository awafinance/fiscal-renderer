package xmlutil

import "testing"

func TestParseFindTextAndAttr(t *testing.T) {
	root, err := ParseString(`<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe"><NFe><infNFe Id="NFe123"><ide><nNF>42</nNF></ide></infNFe></NFe></nfeProc>`)
	if err != nil {
		t.Fatal(err)
	}
	if got := Text(root, "nNF"); got != "42" {
		t.Fatalf("Text(nNF) = %q", got)
	}
	if got := root.Find("infNFe").Attr("Id"); got != "NFe123" {
		t.Fatalf("Attr(Id) = %q", got)
	}
	if got := Text(root, "missing"); got != "" {
		t.Fatalf("missing text = %q", got)
	}
}

func TestFindAll(t *testing.T) {
	root, err := ParseString(`<root><det><x>1</x></det><det><x>2</x></det></root>`)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(root.FindAll("det")); got != 2 {
		t.Fatalf("FindAll(det) count = %d", got)
	}
}
