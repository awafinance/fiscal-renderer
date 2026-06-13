package fiscalfmt

import (
	"reflect"
	"testing"
)

func TestFormatCPFCNPJ(t *testing.T) {
	if got := FormatCPFCNPJ("76586507812"); got != "765.865.078-12" {
		t.Fatalf("FormatCPFCNPJ(CPF) = %q", got)
	}
	if got := FormatCPFCNPJ("12345678000199"); got != "12.345.678/0001-99" {
		t.Fatalf("FormatCPFCNPJ(CNPJ) = %q", got)
	}
}

func TestFormatNumber(t *testing.T) {
	if got := FormatNumber("19500", 0); got != "19.500" {
		t.Fatalf("FormatNumber precision 0 = %q", got)
	}
	if got := FormatNumber("19500.25", 2); got != "19.500,25" {
		t.Fatalf("FormatNumber precision 2 = %q", got)
	}
	if got := FormatNumber("not-a-number", 2); got != "" {
		t.Fatalf("FormatNumber invalid = %q", got)
	}
}

func TestFormatCEP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "12345678", want: "12345-678"},
		{input: "123456789", want: "12345-678"},
		{input: "12345", want: "12345-"},
		{input: "123", want: "123-"},
		{input: "", want: "-"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := FormatCEP(tt.input); got != tt.want {
				t.Fatalf("FormatCEP(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatPhone(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "landline", input: "1132345678", want: "(11) 3234-5678"},
		{name: "mobile", input: "11912345678", want: "(11) 91234-5678"},
		{name: "local landline", input: "32345678", want: "3234-5678"},
		{name: "local mobile", input: "932345678", want: "93234-5678"},
		{name: "country code landline", input: "551132345678", want: "(11) 3234-5678"},
		{name: "country code mobile", input: "5511932345678", want: "(11) 93234-5678"},
		{name: "country code invalid old mobile", input: "551199999999", want: "1199999999"},
		{name: "all zero invalid number", input: "0000000000", want: "000000000"},
		{name: "invalid landline", input: "3100000000", want: "3100000000"},
		{name: "invalid old mobile", input: "6599999999", want: "6599999999"},
		{name: "already formatted", input: "(11) 1234-5678", want: "(11) 1234-5678"},
		{name: "invalid passthrough", input: "123", want: "123"},
		{name: "empty", input: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatPhone(tt.input); got != tt.want {
				t.Fatalf("FormatPhone(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDateUTC(t *testing.T) {
	date, hour := DateUTC("2024-04-03T12:34:56-03:00")
	if date != "03/04/2024" || hour != "12:34:56" {
		t.Fatalf("DateUTC = %q %q", date, hour)
	}
}

func TestChunks(t *testing.T) {
	got := Chunks("123456789", 4)
	want := []string{"1234", "5678", "9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Chunks = %#v", got)
	}
}

func TestLimitText(t *testing.T) {
	if got := LimitText("alpha beta gamma", 10); got != "alpha beta" {
		t.Fatalf("LimitText = %q", got)
	}
}
