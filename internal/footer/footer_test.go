package footer

import (
	"fmt"
	"testing"
)

func TestParseMarkdown(t *testing.T) {
	mk := func(text, style, link string) span {
		return span{text: text, style: style, link: link}
	}
	tests := []struct {
		in   string
		want []span
	}{
		{"plain", []span{mk("plain", "", "")}},
		{"a **b** c", []span{mk("a ", "", ""), mk("b", "B", ""), mk(" c", "", "")}},
		{"*i* __B__", []span{mk("i", "I", ""), mk(" ", "", ""), mk("B", "B", "")}},
		{"by [Awa](https://awa.finance)", []span{mk("by ", "", ""), mk("Awa", "", "https://awa.finance")}},
		{"**[X](u)**", []span{mk("X", "B", "u")}},
		{`escaped \*not italic\*`, []span{mk("escaped *not italic*", "", "")}},
		{"[broken(link)", []span{mk("[broken(link)", "", "")}},
	}
	for _, tt := range tests {
		got := parseMarkdown(tt.in)
		if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", tt.want) {
			t.Errorf("parseMarkdown(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
