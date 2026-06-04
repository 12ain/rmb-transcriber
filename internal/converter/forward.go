package converter

import (
	"strconv"
	"strings"
)

var (
	digitChars = []rune{'零', '壹', '贰', '叁', '肆', '伍', '陆', '柒', '捌', '玖'}
	subUnits   = []string{"", "拾", "佰", "仟"}
	segUnits   = []string{"", "万", "亿", "万亿"}
)

const maxIntDigits = 12

func Forward(amount string) (string, error) {
	negative := strings.HasPrefix(amount, "-")
	if negative {
		amount = amount[1:]
	}
	if !validNumberFormat(amount) {
		return "", &ConverterError{Code: ErrInvalidFormat, Message: "amount must be a valid number with optional decimals"}
	}
	intPart, decPart := splitDecimal(amount)
	stripped := strings.TrimLeft(intPart, "0")
	if len(stripped) > maxIntDigits {
		return "", &ConverterError{Code: ErrOutOfRange, Message: "absolute value exceeds 999,999,999,999.99"}
	}

	var b strings.Builder
	intStr := formatInteger(intPart)
	allZero := stripped == "" && (decPart == "00")
	if negative && !allZero {
		b.WriteString("负")
	}
	b.WriteString(intStr)
	b.WriteString("圆")
	b.WriteString(formatDecimal(decPart))
	return b.String(), nil
}

func validNumberFormat(s string) bool {
	if s == "" {
		return false
	}
	dotSeen := false
	decCount := 0
	for i, c := range s {
		if c == '.' {
			if dotSeen || i == 0 || i == len(s)-1 {
				return false
			}
			dotSeen = true
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		if dotSeen {
			decCount++
		}
	}
	return true
}

func splitDecimal(amount string) (string, string) {
	if idx := strings.Index(amount, "."); idx >= 0 {
		intPart := amount[:idx]
		decPart := amount[idx+1:]
		if len(decPart) == 1 {
			decPart += "0"
		}
		if len(decPart) > 2 {
			decPart = decPart[:2]
		}
		return intPart, decPart
	}
	return amount, "00"
}

func formatInteger(intPart string) string {
	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		return "零"
	}
	n := len(intPart)
	pad := (4 - n%4) % 4
	padded := strings.Repeat("0", pad) + intPart
	segments := len(padded) / 4

	var parts []string
	for i := 0; i < segments; i++ {
		seg := padded[i*4 : i*4+4]
		segIdx := segments - 1 - i
		rendered := renderSegment(seg)
		if rendered == "" {
			if hasNonZeroLater(padded, i+1, segments) && !endsWithZero(parts) {
				parts = append(parts, "零")
			}
			continue
		}
		if seg[0] == '0' && len(parts) > 0 && !endsWithZero(parts) {
			parts = append(parts, "零")
		}
		parts = append(parts, rendered+segUnits[segIdx])
	}
	return strings.Join(parts, "")
}

func renderSegment(seg string) string {
	var sb strings.Builder
	started := false
	pendingZero := false
	for i, r := range seg {
		d := int(r - '0')
		pos := 3 - i
		if d == 0 {
			if started {
				pendingZero = true
			}
			continue
		}
		if pendingZero {
			sb.WriteRune('零')
			pendingZero = false
		}
		sb.WriteRune(digitChars[d])
		sb.WriteString(subUnits[pos])
		started = true
	}
	return sb.String()
}

func hasNonZeroLater(padded string, fromSeg, totalSegs int) bool {
	for j := fromSeg; j < totalSegs; j++ {
		if strings.TrimLeft(padded[j*4:j*4+4], "0") != "" {
			return true
		}
	}
	return false
}

func endsWithZero(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	return strings.HasSuffix(parts[len(parts)-1], "零")
}

func formatDecimal(dec string) string {
	if dec == "00" {
		return "整"
	}
	var sb strings.Builder
	jiao := int(dec[0] - '0')
	fen := int(dec[1] - '0')
	if jiao != 0 {
		sb.WriteRune(digitChars[jiao])
		sb.WriteRune('角')
	} else if fen != 0 {
		sb.WriteRune('零')
	}
	if fen != 0 {
		sb.WriteRune(digitChars[fen])
		sb.WriteRune('分')
	}
	return sb.String()
}

var _ = strconv.Itoa // reserve strconv for future numeric formatting
