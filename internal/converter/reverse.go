package converter

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	bigDigitMap = map[rune]int{
		'零': 0, '壹': 1, '贰': 2, '叁': 3, '肆': 4,
		'伍': 5, '陆': 6, '柒': 7, '捌': 8, '玖': 9,
	}
	bigSubUnits = map[rune]int{
		'拾': 1, '佰': 2, '仟': 3,
	}
)

var reverseRegex = regexp.MustCompile(
	`^(负)?([零壹贰叁肆伍陆柒捌玖拾佰仟万亿]+)?([圆元])(零?[壹贰叁肆伍陆柒捌玖]角)?(零?[壹贰叁肆伍陆柒捌玖]分)?(整|正)?$`,
)

func Reverse(chinese string) (string, error) {
	m := reverseRegex.FindStringSubmatch(chinese)
	if m == nil {
		return "", &ConverterError{Code: ErrUnparsableChinese, Message: "input does not match expected pattern", At: 0}
	}
	neg := m[1] != ""
	intStr := m[2]
	jiao := m[4]
	fen := m[5]
	zheng := m[6]

	var intVal int64
	if intStr != "" && intStr != "零" {
		v, ok := parseIntegerChinese(intStr)
		if !ok {
			return "", &ConverterError{Code: ErrUnparsableChinese, Message: "integer part not parsable", At: 0}
		}
		intVal = v
	}

	jiaoDigit := extractDecimalDigit(jiao)
	fenDigit := extractDecimalDigit(fen)

	if zheng != "" && (jiaoDigit != 0 || fenDigit != 0) {
		return "", &ConverterError{Code: ErrUnparsableChinese, Message: "'整/正' present but decimals are not zero", At: 0}
	}

	var sb strings.Builder
	if neg && (intVal != 0 || jiaoDigit != 0 || fenDigit != 0) {
		sb.WriteRune('-')
	}
	sb.WriteString(strconv.FormatInt(intVal, 10))
	sb.WriteRune('.')
	sb.WriteRune(rune('0' + jiaoDigit))
	sb.WriteRune(rune('0' + fenDigit))
	return sb.String(), nil
}

func extractDecimalDigit(seg string) int {
	if seg == "" {
		return 0
	}
	runes := []rune(seg)
	for _, r := range runes {
		if d, ok := bigDigitMap[r]; ok && r != '零' {
			return d
		}
	}
	return 0
}

func parseIntegerChinese(s string) (int64, bool) {
	var yi, wan, ge int64
	rest := s

	if idx := strings.Index(rest, "亿"); idx >= 0 {
		v, ok := parseFourDigits(strings.TrimLeft(rest[:idx], "零"))
		if !ok {
			return 0, false
		}
		yi = v
		rest = rest[idx+len("亿"):]
	}
	if idx := strings.Index(rest, "万"); idx >= 0 {
		v, ok := parseFourDigits(strings.TrimLeft(rest[:idx], "零"))
		if !ok {
			return 0, false
		}
		wan = v
		rest = rest[idx+len("万"):]
	}
	rest = strings.TrimLeft(rest, "零")
	if rest != "" {
		v, ok := parseFourDigits(rest)
		if !ok {
			return 0, false
		}
		ge = v
	}
	return yi*100000000 + wan*10000 + ge, true
}

func parseFourDigits(s string) (int64, bool) {
	if s == "" {
		return 0, true
	}
	var sum int64
	pending := -1
	for _, r := range s {
		if d, ok := bigDigitMap[r]; ok {
			if pending >= 0 {
				sum += int64(pending)
			}
			pending = d
		} else if u, ok := bigSubUnits[r]; ok {
			if pending < 0 {
				return 0, false
			}
			sum += int64(pending) * pow10(u)
			pending = -1
		} else {
			return 0, false
		}
	}
	if pending >= 0 {
		sum += int64(pending)
	}
	return sum, true
}

func pow10(n int) int64 {
	r := int64(1)
	for i := 0; i < n; i++ {
		r *= 10
	}
	return r
}
