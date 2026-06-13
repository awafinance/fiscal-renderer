package fiscalfmt

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// NumberFilter removes every non-decimal digit from doc.
func NumberFilter(doc string) string {
	var b strings.Builder
	for _, r := range doc {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FormatCPFCNPJ mirrors the Python helper: values with more than 11 digits are
// treated as CNPJ, otherwise as CPF, with left-zero padding.
func FormatCPFCNPJ(doc string) string {
	doc = NumberFilter(doc)
	if doc == "" {
		return ""
	}
	if len(doc) > 11 {
		doc = leftPad(doc, 14)
		if len(doc) > 14 {
			doc = doc[:14]
		}
		return fmt.Sprintf("%s.%s.%s/%s-%s", doc[:2], doc[2:5], doc[5:8], doc[8:12], doc[12:14])
	}
	doc = leftPad(doc, 11)
	if len(doc) > 11 {
		doc = doc[:11]
	}
	return fmt.Sprintf("%s.%s.%s-%s", doc[:3], doc[3:6], doc[6:9], doc[9:11])
}

func leftPad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat("0", width-len(s)) + s
}

func FormatCEP(cep string) string {
	end := 8
	if len(cep) < end {
		end = len(cep)
	}
	mid := 5
	if len(cep) < mid {
		mid = len(cep)
	}
	return cep[:mid] + "-" + cep[mid:end]
}

func FormatPhone(phone string) string {
	digits := NumberFilter(phone)
	if strings.HasPrefix(digits, "55") {
		local := digits[2:]
		switch len(local) {
		case 10:
			if len(local) >= 3 && local[2] == '9' {
				return local
			}
			digits = local
		case 11:
			digits = local
		}
	}
	switch len(digits) {
	case 8:
		return fmt.Sprintf("%s-%s", digits[:4], digits[4:])
	case 9:
		return fmt.Sprintf("%s-%s", digits[:5], digits[5:])
	case 10:
		if digits == strings.Repeat("0", 10) {
			return strings.Repeat("0", 9)
		}
		if digits[2] == '0' || digits[2] == '9' {
			return digits
		}
		return fmt.Sprintf("(%s) %s-%s", digits[:2], digits[2:6], digits[6:])
	case 11:
		return fmt.Sprintf("(%s) %s-%s", digits[:2], digits[2:7], digits[7:])
	default:
		return phone
	}
}

func DateUTC(dateUTC string) (date string, hour string) {
	if len(dateUTC) < 10 {
		return "", ""
	}
	parts := strings.Split(dateUTC[:10], "-")
	if len(parts) != 3 {
		return "", ""
	}
	date = parts[2] + "/" + parts[1] + "/" + parts[0]
	if len(dateUTC) >= 19 {
		hour = dateUTC[11:19]
	}
	return date, hour
}

func Chunks(s string, n int) []string {
	if n <= 0 {
		return nil
	}
	out := make([]string, 0, int(math.Ceil(float64(len(s))/float64(n))))
	for start := 0; start < len(s); start += n {
		end := start + n
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[start:end])
	}
	return out
}

func FormatNumber(value string, precision int) string {
	if value == "" {
		value = "0"
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return ""
	}
	formatted := fmt.Sprintf("%.*f", precision, number)
	parts := strings.SplitN(formatted, ".", 2)
	intPart := groupThousands(parts[0])
	if precision == 0 {
		return intPart
	}
	return intPart + "," + parts[1]
}

func groupThousands(value string) string {
	sign := ""
	if strings.HasPrefix(value, "-") {
		sign = "-"
		value = strings.TrimPrefix(value, "-")
	}
	if len(value) <= 3 {
		return sign + value
	}
	var groups []string
	for len(value) > 3 {
		groups = append([]string{value[len(value)-3:]}, groups...)
		value = value[:len(value)-3]
	}
	groups = append([]string{value}, groups...)
	return sign + strings.Join(groups, ".")
}

func MergeIfDifferent(value1, value2 string) string {
	if strings.ToLower(value1) != strings.ToLower(value2) {
		return value1 + "\n" + value2
	}
	return value1
}

func FormatXDime(value string) string {
	parts := strings.Split(value, "X")
	if len(parts) != 3 {
		return value
	}
	for _, part := range parts {
		if part == "" || NumberFilter(part) != part {
			return value
		}
	}
	return value + " (cm)"
}

func LimitText(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	words := strings.Fields(text)
	var result []string
	length := 0
	for _, word := range words {
		extra := 0
		if len(result) > 0 {
			extra = 1
		}
		if length+len(word)+extra > maxLen {
			break
		}
		result = append(result, word)
		length += len(word) + extra
	}
	return strings.Join(result, " ")
}
