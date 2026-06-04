# RMB v1 Enrichment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add reverse/verify/batch endpoints, an API docs page (with embedded Swagger UI), and reorganize the project into `internal/converter` + `go:embed` static layout. UI gains a 4-mode Tab (转写/还原/校验/批量) with shared theme CSS.

**Architecture:** Refactor into thin `main.go` (HTTP routes only) + `internal/converter` package (forward / reverse / verify / batch + errors). Embed `static/*` via `go:embed`. Frontend reorganized to 4 Tabs sharing one CSS variable file. New `/docs` hand-written reference + `/docs/spec` embedded Swagger UI.

**Tech Stack:** Go 1.21 + Gin v1.10. Vue 3 + axios (CDN). Swagger UI 5 (CDN). No new Go dependencies.

**Module path:** `github.com/12ain/rmb-uppercase-converter` (verified from `go.mod`).

**Spec:** `docs/superpowers/specs/2026-06-04-v1-enrichment-design.md` (read before starting; this plan implements that spec verbatim).

---

## Task 1: Bootstrap `internal/converter` with error types

**Files:**
- Create: `internal/converter/errors.go`
- Create: `internal/converter/errors_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/converter/errors_test.go`:

```go
package converter

import "testing"

func TestConverterError_Error(t *testing.T) {
	e := &ConverterError{Code: ErrInvalidFormat, Message: "bad input"}
	got := e.Error()
	want := "invalid_format: bad input"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestErrorCodes_AreStable(t *testing.T) {
	cases := map[ErrorCode]string{
		ErrInvalidFormat:     "invalid_format",
		ErrOutOfRange:        "out_of_range",
		ErrUnparsableChinese: "unparsable_chinese",
		ErrBatchTooLarge:     "batch_too_large",
	}
	for code, want := range cases {
		if string(code) != want {
			t.Errorf("code %v: got %q, want %q", code, string(code), want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/converter/...`
Expected: build failure (`ConverterError` and error code constants undefined).

- [ ] **Step 3: Implement `errors.go`**

Create `internal/converter/errors.go`:

```go
package converter

type ErrorCode string

const (
	ErrInvalidFormat     ErrorCode = "invalid_format"
	ErrOutOfRange        ErrorCode = "out_of_range"
	ErrUnparsableChinese ErrorCode = "unparsable_chinese"
	ErrBatchTooLarge     ErrorCode = "batch_too_large"
)

type ConverterError struct {
	Code    ErrorCode
	Message string
	At      int
}

func (e *ConverterError) Error() string {
	return string(e.Code) + ": " + e.Message
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/converter/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/converter/errors.go internal/converter/errors_test.go
git commit -m "feat(converter): add ConverterError type and error code constants"
```

---

## Task 2: Implement `Forward` (number → Chinese uppercase) with full edge cases

**Files:**
- Create: `internal/converter/forward.go`
- Create: `internal/converter/forward_test.go`

This task migrates and **fixes** the existing `ConvertToChinese` logic. Known bugs being fixed:
1. `"0"` currently produces `"圆整"` instead of `"零圆整"`.
2. `"100000000"` currently writes a spurious `"万"` for the all-zero 万 segment.
3. No negative number support.

The new algorithm processes the integer in 4-digit segments (个/万/亿) instead of single-digit-with-position.

- [ ] **Step 1: Write the failing table-driven test**

Create `internal/converter/forward_test.go`:

```go
package converter

import "testing"

func TestForward_Success(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"zero", "0", "零圆整"},
		{"zero with decimal", "0.00", "零圆整"},
		{"only fen", "0.01", "零圆零壹分"},
		{"only jiao", "0.10", "零圆壹角"},
		{"jiao and fen", "0.56", "零圆伍角陆分"},
		{"single digit", "5", "伍圆整"},
		{"one hundred", "100", "壹佰圆整"},
		{"one thousand", "1000", "壹仟圆整"},
		{"one wan", "10000", "壹万圆整"},
		{"one hundred million", "100000000", "壹亿圆整"},
		{"ten billion", "10000000000", "壹佰亿圆整"},
		{"full amount", "1234.56", "壹仟贰佰叁拾肆圆伍角陆分"},
		{"internal zeros", "10010010.01", "壹仟零壹万零壹拾圆零壹分"},
		{"thousand and one", "1001", "壹仟零壹圆整"},
		{"ten thousand and one", "10001", "壹万零壹圆整"},
		{"hundred million plus fen", "100000000.01", "壹亿圆零壹分"},
		{"trim one decimal", "12.3", "壹拾贰圆叁角"},
		{"keep two decimals", "12.34", "壹拾贰圆叁角肆分"},
		{"truncate beyond two", "12.345", "壹拾贰圆叁角肆分"},
		{"max boundary", "999999999999.99", "玖仟玖佰玖拾玖亿玖仟玖佰玖拾玖万玖仟玖佰玖拾玖圆玖角玖分"},
		{"negative", "-1234.56", "负壹仟贰佰叁拾肆圆伍角陆分"},
		{"negative zero", "-0", "零圆整"},
		{"leading zeros stripped", "00012.34", "壹拾贰圆叁角肆分"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Forward(c.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("Forward(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestForward_Errors(t *testing.T) {
	cases := []struct {
		name, in string
		code     ErrorCode
	}{
		{"empty", "", ErrInvalidFormat},
		{"letters", "abc", ErrInvalidFormat},
		{"trailing dot", "12.", ErrInvalidFormat},
		{"leading dot", ".5", ErrInvalidFormat},
		{"double dot", "1.2.3", ErrInvalidFormat},
		{"three decimals not truncatable", "1.234a", ErrInvalidFormat},
		{"over ceiling", "1000000000000", ErrOutOfRange},
		{"negative over ceiling", "-1000000000000", ErrOutOfRange},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Forward(c.in)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			ce, ok := err.(*ConverterError)
			if !ok {
				t.Fatalf("expected *ConverterError, got %T", err)
			}
			if ce.Code != c.code {
				t.Errorf("got code %q, want %q", ce.Code, c.code)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/converter/... -run Forward`
Expected: build error (`Forward` undefined).

- [ ] **Step 3: Implement `forward.go`**

Create `internal/converter/forward.go`:

```go
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
		return "", &ConverterError{Code: ErrInvalidFormat, Message: "amount must match ^-?\\d+(\\.\\d{1,2})?$"}
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
	return decCount <= 2
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/converter/... -v -run Forward`
Expected: all sub-cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/converter/forward.go internal/converter/forward_test.go
git commit -m "feat(converter): implement Forward with zero-edge and negative support"
```

---

## Task 3: Implement `Reverse` (Chinese uppercase → number)

**Files:**
- Create: `internal/converter/reverse.go`
- Create: `internal/converter/reverse_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/converter/reverse_test.go`:

```go
package converter

import "testing"

func TestReverse_Success(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"zero integer with 整", "零圆整", "0.00"},
		{"full amount", "壹仟贰佰叁拾肆圆伍角陆分", "1234.56"},
		{"one thousand", "壹仟圆整", "1000.00"},
		{"omit 整", "壹仟圆", "1000.00"},
		{"one yi", "壹亿圆整", "100000000.00"},
		{"internal zeros", "壹仟零壹万零壹拾圆零壹分", "10010010.01"},
		{"only fen", "零圆零壹分", "0.01"},
		{"only jiao", "零圆壹角", "0.10"},
		{"accept 元", "壹仟贰佰叁拾肆元伍角陆分", "1234.56"},
		{"accept 正", "壹仟元正", "1000.00"},
		{"accept 元正 combo", "壹亿元正", "100000000.00"},
		{"negative", "负壹仟贰佰叁拾肆圆伍角陆分", "-1234.56"},
		{"ten wan", "壹拾万圆整", "100000.00"},
		{"max boundary", "玖仟玖佰玖拾玖亿玖仟玖佰玖拾玖万玖仟玖佰玖拾玖圆玖角玖分", "999999999999.99"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Reverse(c.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("Reverse(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestReverse_RoundTripSymmetry(t *testing.T) {
	amounts := []string{"0", "0.01", "1234.56", "10000", "100000000", "10010010.01", "999999999999.99"}
	currencies := []string{"圆", "元"}
	zhengs := []string{"整", "正"}

	for _, a := range amounts {
		canonical, err := Forward(a)
		if err != nil {
			t.Fatalf("Forward(%q) failed: %v", a, err)
		}
		// canonical uses 圆 + 整 by spec; build the alternative variants and verify both reverse equal
		for _, ccy := range currencies {
			for _, zh := range zhengs {
				variant := canonical
				if ccy != "圆" {
					variant = replaceFirst(variant, "圆", ccy)
				}
				if zh != "整" {
					variant = replaceFirst(variant, "整", zh)
				}
				got, rerr := Reverse(variant)
				if rerr != nil {
					t.Errorf("Reverse(%q) error: %v", variant, rerr)
					continue
				}
				want := normalizeAmount(a)
				if got != want {
					t.Errorf("Reverse(%q) = %q, want %q", variant, got, want)
				}
			}
		}
	}
}

func replaceFirst(s, old, new string) string {
	idx := indexRune(s, old)
	if idx < 0 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

func indexRune(s, sub string) int {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func normalizeAmount(a string) string {
	// Forward returns canonical form; Reverse returns "X.YY" always.
	if !contains(a, ".") {
		return a + ".00"
	}
	parts := splitTwo(a, '.')
	dec := parts[1]
	if len(dec) == 1 {
		dec += "0"
	}
	if len(dec) > 2 {
		dec = dec[:2]
	}
	return parts[0] + "." + dec
}

func contains(s, sub string) bool { return indexRune(s, sub) >= 0 }

func splitTwo(s string, sep byte) [2]string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}

func TestReverse_Errors(t *testing.T) {
	cases := []struct {
		name, in string
		code     ErrorCode
	}{
		{"empty", "", ErrUnparsableChinese},
		{"no currency unit", "壹仟贰佰", ErrUnparsableChinese},
		{"unknown char", "壹仟ABC圆整", ErrUnparsableChinese},
		{"zheng with non-zero decimal", "壹圆伍角整", ErrUnparsableChinese},
		{"lone unit", "拾圆整", ErrUnparsableChinese},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Reverse(c.in)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			ce, ok := err.(*ConverterError)
			if !ok {
				t.Fatalf("expected *ConverterError, got %T", err)
			}
			if ce.Code != c.code {
				t.Errorf("got code %q, want %q", ce.Code, c.code)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/converter/... -run Reverse`
Expected: build error (`Reverse` undefined).

- [ ] **Step 3: Implement `reverse.go`**

Create `internal/converter/reverse.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/converter/... -v -run Reverse`
Expected: all sub-cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/converter/reverse.go internal/converter/reverse_test.go
git commit -m "feat(converter): implement Reverse with regex slicing parser"
```

---

## Task 4: Implement `Verify` (bidirectional consistency check)

**Files:**
- Create: `internal/converter/verify.go`
- Create: `internal/converter/verify_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/converter/verify_test.go`:

```go
package converter

import "testing"

func TestVerify_Match(t *testing.T) {
	r, err := Verify("1234.56", "壹仟贰佰叁拾肆圆伍角陆分")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Match {
		t.Errorf("expected match, got mismatch")
	}
	if r.Expected != "壹仟贰佰叁拾肆圆伍角陆分" {
		t.Errorf("expected canonical, got %q", r.Expected)
	}
}

func TestVerify_Mismatch(t *testing.T) {
	r, err := Verify("1234.56", "壹仟贰佰叁拾肆圆伍角柒分")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Match {
		t.Errorf("expected mismatch")
	}
	if r.Expected != "壹仟贰佰叁拾肆圆伍角陆分" {
		t.Errorf("got expected %q", r.Expected)
	}
	if r.DiffAt != 9 {
		t.Errorf("got diffAt %d, want 9", r.DiffAt)
	}
}

func TestVerify_UnparsableChinese(t *testing.T) {
	r, err := Verify("1234.56", "garbage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Match {
		t.Errorf("expected mismatch")
	}
	if r.Message == "" {
		t.Errorf("expected parse failure message")
	}
}

func TestVerify_InvalidAmount(t *testing.T) {
	_, err := Verify("abc", "壹圆整")
	if err == nil {
		t.Fatalf("expected error")
	}
	ce, ok := err.(*ConverterError)
	if !ok || ce.Code != ErrInvalidFormat {
		t.Errorf("got %v, want invalid_format error", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/converter/... -run Verify`
Expected: build error (`Verify` and `VerifyResult` undefined).

- [ ] **Step 3: Implement `verify.go`**

Create `internal/converter/verify.go`:

```go
package converter

type VerifyResult struct {
	Match    bool   `json:"match"`
	Expected string `json:"expected"`
	DiffAt   int    `json:"diffAt,omitempty"`
	Message  string `json:"message,omitempty"`
}

func Verify(amount, chinese string) (*VerifyResult, error) {
	expected, err := Forward(amount)
	if err != nil {
		return nil, err
	}
	if chinese == expected {
		return &VerifyResult{Match: true, Expected: expected}, nil
	}
	r := &VerifyResult{
		Match:    false,
		Expected: expected,
		DiffAt:   firstRuneDiff(chinese, expected),
	}
	if _, perr := Reverse(chinese); perr != nil {
		if ce, ok := perr.(*ConverterError); ok {
			r.Message = ce.Message
		} else {
			r.Message = perr.Error()
		}
	}
	return r, nil
}

func firstRuneDiff(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	n := len(ar)
	if len(br) < n {
		n = len(br)
	}
	for i := 0; i < n; i++ {
		if ar[i] != br[i] {
			return i
		}
	}
	return n
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/converter/... -v -run Verify`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/converter/verify.go internal/converter/verify_test.go
git commit -m "feat(converter): implement Verify with diff position"
```

---

## Task 5: Implement `Batch` with 200-item ceiling

**Files:**
- Create: `internal/converter/batch.go`
- Create: `internal/converter/batch_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/converter/batch_test.go`:

```go
package converter

import (
	"strings"
	"testing"
)

func TestBatch_Empty(t *testing.T) {
	out, err := Batch(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected 0 items, got %d", len(out))
	}
}

func TestBatch_Mixed(t *testing.T) {
	out, err := Batch([]string{"1234.56", "abc", "1000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 items, got %d", len(out))
	}
	if out[0].Chinese != "壹仟贰佰叁拾肆圆伍角陆分" || out[0].Error != "" {
		t.Errorf("item 0: %+v", out[0])
	}
	if out[1].Error != string(ErrInvalidFormat) {
		t.Errorf("item 1: expected invalid_format, got %+v", out[1])
	}
	if out[2].Chinese != "壹仟圆整" {
		t.Errorf("item 2: %+v", out[2])
	}
}

func TestBatch_Ceiling(t *testing.T) {
	exactly := make([]string, MaxBatchSize)
	for i := range exactly {
		exactly[i] = "1"
	}
	if _, err := Batch(exactly); err != nil {
		t.Fatalf("at ceiling should succeed: %v", err)
	}

	overLimit := make([]string, MaxBatchSize+1)
	for i := range overLimit {
		overLimit[i] = "1"
	}
	_, err := Batch(overLimit)
	if err == nil {
		t.Fatalf("expected error for over-limit batch")
	}
	ce, ok := err.(*ConverterError)
	if !ok || ce.Code != ErrBatchTooLarge {
		t.Errorf("got %v, want batch_too_large", err)
	}
	if !strings.Contains(ce.Message, "200") {
		t.Errorf("message should mention limit: %q", ce.Message)
	}
}

func TestBatch_OrderPreserved(t *testing.T) {
	in := []string{"3", "1", "2"}
	out, _ := Batch(in)
	for i, item := range out {
		if item.Amount != in[i] {
			t.Errorf("position %d: got %q, want %q", i, item.Amount, in[i])
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/converter/... -run Batch`
Expected: build error.

- [ ] **Step 3: Implement `batch.go`**

Create `internal/converter/batch.go`:

```go
package converter

const MaxBatchSize = 200

type BatchItem struct {
	Amount  string `json:"amount"`
	Chinese string `json:"chinese,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func Batch(amounts []string) ([]BatchItem, error) {
	if len(amounts) > MaxBatchSize {
		return nil, &ConverterError{Code: ErrBatchTooLarge, Message: "batch size exceeds 200"}
	}
	out := make([]BatchItem, len(amounts))
	for i, a := range amounts {
		item := BatchItem{Amount: a}
		chinese, err := Forward(a)
		if err != nil {
			if ce, ok := err.(*ConverterError); ok {
				item.Error = string(ce.Code)
				item.Message = ce.Message
			} else {
				item.Error = "internal"
				item.Message = err.Error()
			}
		} else {
			item.Chinese = chinese
		}
		out[i] = item
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/converter/... -v`
Expected: every test across all 4 source files PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/converter/batch.go internal/converter/batch_test.go
git commit -m "feat(converter): implement Batch with 200-item ceiling"
```

---

## Task 6: Refactor `main.go` — thin HTTP layer + 4 routes + `go:embed`

**Files:**
- Modify: `main.go` (full rewrite)
- Create: `main_test.go`

The existing `main.go` still holds the old `ConvertToChinese` and only registers two routes. This task replaces it entirely: routes call into `internal/converter`, static assets are embedded.

- [ ] **Step 1: Write the failing integration test**

Create `main_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return buildRouter()
}

func TestRoute_Convert_Compat(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]string{"amount": "1234.56"})
	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["chinese"] != "壹仟贰佰叁拾肆圆伍角陆分" {
		t.Errorf("got %q", resp["chinese"])
	}
}

func TestRoute_Reverse(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]string{"chinese": "壹仟贰佰叁拾肆圆伍角陆分"})
	req := httptest.NewRequest("POST", "/api/convert/reverse", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["amount"] != "1234.56" {
		t.Errorf("got %q", resp["amount"])
	}
}

func TestRoute_Verify_Match(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]string{
		"amount":  "1234.56",
		"chinese": "壹仟贰佰叁拾肆圆伍角陆分",
	})
	req := httptest.NewRequest("POST", "/api/convert/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["match"] != true {
		t.Errorf("expected match=true, got %+v", resp)
	}
}

func TestRoute_Batch_OverLimit(t *testing.T) {
	r := newTestRouter()
	amounts := make([]string, 201)
	for i := range amounts {
		amounts[i] = "1"
	}
	body, _ := json.Marshal(map[string]any{"amounts": amounts})
	req := httptest.NewRequest("POST", "/api/convert/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 413 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "batch_too_large" {
		t.Errorf("got error %v", resp["error"])
	}
}

func TestRoute_InvalidFormat(t *testing.T) {
	r := newTestRouter()
	body := strings.NewReader(`{"amount":"abc"}`)
	req := httptest.NewRequest("POST", "/api/convert", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("status %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "invalid_format" {
		t.Errorf("got error %v", resp["error"])
	}
}

func TestRoute_OpenAPI_Served(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest("GET", "/openapi.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "json") {
		t.Errorf("expected JSON content-type, got %q", w.Header().Get("Content-Type"))
	}
}
```

- [ ] **Step 2: Create a placeholder `static/openapi.json` so the embed compiles**

Run:
```bash
echo '{"openapi":"3.1.0","info":{"title":"placeholder","version":"0.0.0"},"paths":{}}' > static/openapi.json
```

(The real OpenAPI spec is written in Task 12.)

- [ ] **Step 3: Rewrite `main.go`**

Replace the entire contents of `main.go` with:

```go
package main

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/12ain/rmb-uppercase-converter/internal/converter"
	"github.com/gin-gonic/gin"
)

//go:embed static
var staticFS embed.FS

func buildRouter() *gin.Engine {
	r := gin.Default()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	r.StaticFS("/static", http.FS(sub))

	r.GET("/", func(c *gin.Context) {
		c.FileFromFS("index.html", http.FS(sub))
	})
	r.GET("/docs", func(c *gin.Context) {
		c.FileFromFS("docs.html", http.FS(sub))
	})
	r.GET("/docs/spec", func(c *gin.Context) {
		c.FileFromFS("swagger.html", http.FS(sub))
	})
	r.GET("/openapi.json", func(c *gin.Context) {
		c.FileFromFS("openapi.json", http.FS(sub))
	})

	r.POST("/api/convert", handleConvert)
	r.POST("/api/convert/reverse", handleReverse)
	r.POST("/api/convert/verify", handleVerify)
	r.POST("/api/convert/batch", handleBatch)

	return r
}

func main() {
	buildRouter().Run(":8080")
}

func handleConvert(c *gin.Context) {
	var req struct {
		Amount string `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": string(converter.ErrInvalidFormat), "message": "invalid JSON"})
		return
	}
	chinese, err := converter.Forward(req.Amount)
	if err != nil {
		writeConverterError(c, err)
		return
	}
	c.JSON(200, gin.H{"chinese": chinese})
}

func handleReverse(c *gin.Context) {
	var req struct {
		Chinese string `json:"chinese"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": string(converter.ErrUnparsableChinese), "message": "invalid JSON"})
		return
	}
	amount, err := converter.Reverse(req.Chinese)
	if err != nil {
		writeConverterError(c, err)
		return
	}
	c.JSON(200, gin.H{"amount": amount})
}

func handleVerify(c *gin.Context) {
	var req struct {
		Amount  string `json:"amount"`
		Chinese string `json:"chinese"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": string(converter.ErrInvalidFormat), "message": "invalid JSON"})
		return
	}
	result, err := converter.Verify(req.Amount, req.Chinese)
	if err != nil {
		writeConverterError(c, err)
		return
	}
	c.JSON(200, result)
}

func handleBatch(c *gin.Context) {
	var req struct {
		Amounts []string `json:"amounts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": string(converter.ErrInvalidFormat), "message": "invalid JSON"})
		return
	}
	results, err := converter.Batch(req.Amounts)
	if err != nil {
		writeConverterError(c, err)
		return
	}
	c.JSON(200, gin.H{"results": results})
}

func writeConverterError(c *gin.Context, err error) {
	if ce, ok := err.(*converter.ConverterError); ok {
		status := 400
		if ce.Code == converter.ErrBatchTooLarge {
			status = 413
		}
		resp := gin.H{"error": string(ce.Code), "message": ce.Message}
		if ce.At > 0 {
			resp["at"] = ce.At
		}
		c.JSON(status, resp)
		return
	}
	c.JSON(500, gin.H{"error": "internal", "message": err.Error()})
}
```

- [ ] **Step 4: Run all tests**

Run: `go test ./...`
Expected: every test PASS (converter package + main package).

- [ ] **Step 5: Smoke-run the server manually**

Run: `go run . &`
Then:
```bash
sleep 1
curl -s -X POST http://localhost:8080/api/convert -d '{"amount":"1234.56"}' -H 'Content-Type: application/json'
curl -s -X POST http://localhost:8080/api/convert/reverse -d '{"chinese":"壹仟圆整"}' -H 'Content-Type: application/json'
kill %1
```
Expected: first returns `{"chinese":"壹仟贰佰叁拾肆圆伍角陆分"}`, second returns `{"amount":"1000.00"}`.

- [ ] **Step 6: Commit**

```bash
git add main.go main_test.go static/openapi.json
git commit -m "refactor: thin main.go using internal/converter + go:embed static"
```

---

## Task 7: Extract `static/shared/theme.css`

**Files:**
- Create: `static/shared/theme.css`
- Modify: `static/index.html` (replace inline `:root`/`[data-theme]` blocks with `<link>`)

The current `static/index.html` has both light and dark theme CSS variables inline in its `<style>` block. Extract them into a shared file so `docs.html` (Task 14) can reuse them without duplication.

- [ ] **Step 1: Open `static/index.html` and locate the existing theme variable blocks**

The blocks live inside `<style>` and start with `/* ── DARK (default) ─────...` through the end of the `[data-theme="light"]` block. Copy these two blocks verbatim into a new file.

- [ ] **Step 2: Create `static/shared/theme.css`**

Create `static/shared/theme.css` with the two blocks (paste exactly as they appear in `index.html`):

```css
/* ── DARK (default) ─────────────────────────────────── */
[data-theme="dark"] {
  --bg: #0a0a0c;
  --bg-elev: #131316;
  --surface: rgba(255, 255, 255, 0.035);
  --surface-2: rgba(255, 255, 255, 0.06);
  --surface-hover: rgba(255, 255, 255, 0.09);
  --hairline: rgba(255, 255, 255, 0.08);
  --hairline-strong: rgba(255, 255, 255, 0.16);
  --fg: #f5f3ee;
  --fg-2: #c8c5bd;
  --muted: #6b6862;
  --muted-2: #3d3b37;
  --accent: #d4ff5e;
  --accent-fg: #0a0a0c;
  --accent-glow: rgba(212, 255, 94, 0.22);
  --warn: #ff7a5c;
  --ok: #6fe39e;
  --mesh-1: rgba(212, 255, 94, 0.10);
  --mesh-2: rgba(255, 122, 92, 0.08);
  --noise-opacity: 0.35;
  --noise-blend: overlay;
  --shadow-card: 0 8px 32px rgba(0, 0, 0, 0.4);
}

/* ── LIGHT ──────────────────────────────────────────── */
[data-theme="light"] {
  --bg: #f4f1ea;
  --bg-elev: #ffffff;
  --surface: rgba(255, 255, 255, 0.6);
  --surface-2: rgba(255, 255, 255, 0.85);
  --surface-hover: rgba(255, 255, 255, 1);
  --hairline: rgba(20, 16, 10, 0.10);
  --hairline-strong: rgba(20, 16, 10, 0.22);
  --fg: #14110a;
  --fg-2: #3d3a32;
  --muted: #8a8479;
  --muted-2: #c4beae;
  --accent: #2e7d32;
  --accent-fg: #ffffff;
  --accent-glow: rgba(46, 125, 50, 0.18);
  --warn: #c2410c;
  --ok: #2e7d32;
  --mesh-1: rgba(46, 125, 50, 0.08);
  --mesh-2: rgba(194, 65, 12, 0.06);
  --noise-opacity: 0.25;
  --noise-blend: multiply;
  --shadow-card: 0 4px 20px rgba(20, 16, 10, 0.06);
}
```

- [ ] **Step 3: Remove the two blocks from `static/index.html` and add a `<link>`**

In `static/index.html`'s `<head>`, immediately before the `<style>` tag, add:

```html
<link rel="stylesheet" href="/static/shared/theme.css">
```

Then delete both `[data-theme="dark"]` and `[data-theme="light"]` blocks from inside `<style>` (the rest of the style block stays intact).

- [ ] **Step 4: Manual smoke check**

Run: `go run .`
Open `http://localhost:8080` in a browser. Verify:
- Dark mode displays with lime accent (default)
- Theme toggle button still flips to light mode (warm cream + deep green)
- No FOUC at page load (theme bootstrap script in `<head>` still runs before paint)

Stop the server.

- [ ] **Step 5: Commit**

```bash
git add static/shared/theme.css static/index.html
git commit -m "refactor(ui): extract theme variables to shared/theme.css"
```

---

## Task 8: Add Tab scaffolding + state isolation to `index.html`

**Files:**
- Modify: `static/index.html`

Add a 4-tab strip below the hero. Refactor Vue `data()` so each tab has its own state slice. Default tab = `forward` (renders the existing UI); the other three tabs render a placeholder panel that says "this tab will be wired up next". Subsequent tasks (9/10/11) replace each placeholder with real UI.

- [ ] **Step 1: Insert the Tab strip markup**

In `static/index.html`, locate the `<!-- WORKSPACE -->` comment (right before `<section class="workspace">`). Immediately before it, insert:

```html
<!-- MODE TABS -->
<nav class="mode-tabs reveal r3" role="tablist" aria-label="模式切换">
  <span class="mode-indicator" :style="{ transform: 'translateX(' + tabIndex * 100 + '%)' }"></span>
  <button
    v-for="(t, i) in tabs"
    :key="t.key"
    role="tab"
    :class="['mode-tab', { active: activeTab === t.key }]"
    :aria-selected="activeTab === t.key"
    @click="activeTab = t.key"
  >
    <span class="mode-zh">{{ t.zh }}</span>
    <span class="mode-en">{{ t.en }}</span>
  </button>
</nav>
```

- [ ] **Step 2: Wrap the existing workspace in a `v-if="activeTab === 'forward'"`**

Find `<section class="workspace">` and change it to:

```html
<section v-if="activeTab === 'forward'" class="workspace">
```

Then after the closing `</section>` of the existing workspace, add three placeholder sections:

```html
<section v-else-if="activeTab === 'reverse'" class="workspace placeholder-section">
  <div class="panel"><div class="placeholder-msg">还原 Tab 将在下一步接入</div></div>
</section>
<section v-else-if="activeTab === 'verify'" class="workspace placeholder-section">
  <div class="panel"><div class="placeholder-msg">校验 Tab 将在下一步接入</div></div>
</section>
<section v-else-if="activeTab === 'batch'" class="workspace placeholder-section">
  <div class="panel"><div class="placeholder-msg">批量 Tab 将在下一步接入</div></div>
</section>
```

- [ ] **Step 3: Add CSS for the Tab strip**

Inside the existing `<style>` block, append (just before the `/* Responsive */` block):

```css
/* ── MODE TABS ─────────────────────────────────────── */
.mode-tabs {
  margin-top: 48px;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  background: var(--surface);
  border: 1px solid var(--hairline);
  border-radius: 16px;
  padding: 4px;
  position: relative;
  backdrop-filter: blur(12px);
}
.mode-tab {
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 14px 16px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  color: var(--muted);
  transition: color 0.3s;
  position: relative;
  z-index: 2;
}
.mode-tab.active {
  color: var(--accent-fg);
}
.mode-tab:not(.active):hover { color: var(--fg-2); }
.mode-zh {
  font-family: 'Bricolage Grotesque', sans-serif;
  font-weight: 600;
  font-size: 15px;
  letter-spacing: 0.18em;
}
.mode-en {
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  letter-spacing: 0.24em;
  text-transform: uppercase;
  opacity: 0.75;
}
.mode-indicator {
  position: absolute;
  top: 4px;
  left: 4px;
  width: calc(25% - 2px);
  height: calc(100% - 8px);
  background: var(--accent);
  border-radius: 12px;
  transition: transform 0.4s cubic-bezier(0.2, 0.8, 0.2, 1);
  z-index: 1;
}
.placeholder-section .placeholder-msg {
  padding: 64px 32px;
  text-align: center;
  font-family: 'Bricolage Grotesque', sans-serif;
  color: var(--muted);
  font-size: 18px;
  letter-spacing: -0.01em;
}
```

- [ ] **Step 4: Refactor Vue data for state isolation**

Locate the `data()` function in the existing Vue app. Replace its returned object with:

```js
return {
  tabs: [
    { key: 'forward', zh: '转 写', en: 'convert' },
    { key: 'reverse', zh: '还 原', en: 'reverse' },
    { key: 'verify',  zh: '校 验', en: 'verify'  },
    { key: 'batch',   zh: '批 量', en: 'batch'   },
  ],
  activeTab: 'forward',
  // forward state (existing)
  amount: '',
  result: '',
  resultKey: 0,
  resultStamp: '',
  errorMessage: '',
  isValid: false,
  isConverting: false,
  copied: false,
  // reverse state
  reverseInput: '',
  reverseResult: '',
  reverseStamp: '',
  reverseError: '',
  reverseConverting: false,
  reverseCopied: false,
  // verify state
  verifyAmount: '',
  verifyChinese: '',
  verifyResult: null,    // { match, expected, diffAt, message } | null
  verifyConverting: false,
  verifyError: '',
  // batch state
  batchInput: '',
  batchResults: [],      // [{amount, chinese?, error?, message?}]
  batchConverting: false,
  batchError: '',
  // shared
  theme: document.documentElement.getAttribute('data-theme') || 'dark',
  examples: [
    { num: '1234.56',      zh: '壹仟贰佰叁拾肆圆伍角陆分' },
    { num: '1000.00',      zh: '壹仟圆整' },
    { num: '100000000.00', zh: '壹亿圆整' },
    { num: '10010010.01',  zh: '壹仟零壹万零壹拾圆零壹分' },
  ],
  todayStr: '',
};
```

- [ ] **Step 5: Add `tabIndex` computed property**

Inside the Vue config (next to `data()` and `methods`), add:

```js
computed: {
  tabIndex() {
    return this.tabs.findIndex(t => t.key === this.activeTab);
  },
},
```

- [ ] **Step 6: Manual smoke check**

Run: `go run .` and open `http://localhost:8080`.
- Tab strip shows below hero with 4 tabs; indicator under "转 写" (lime in dark mode).
- Clicking each tab slides the indicator and shows the placeholder panel for non-forward tabs.
- Switching back to "转 写" restores the existing UI with no loss of any input you typed.

- [ ] **Step 7: Commit**

```bash
git add static/index.html
git commit -m "feat(ui): add 4-mode tab scaffolding with isolated Vue state"
```

---

## Task 9: Implement Reverse Tab — UI + wiring

**Files:**
- Modify: `static/index.html`

Replace the `activeTab === 'reverse'` placeholder with a real workspace. Left panel = numeric result. Right panel = textarea for the Chinese uppercase string + a "还 原" button calling `/api/convert/reverse`.

- [ ] **Step 1: Replace the reverse placeholder section**

Find the `<section v-else-if="activeTab === 'reverse'" class="workspace placeholder-section">` block and replace the entire `<section>` with:

```html
<section v-else-if="activeTab === 'reverse'" class="workspace">
  <div class="panel result-panel">
    <div class="panel-head">
      <div class="label">OUTPUT · <b>阿拉伯数字</b></div>
      <span class="chip" :class="{ ok: reverseResult }">
        <span class="d"></span>{{ reverseResult ? 'PARSED' : 'AWAITING' }}
      </span>
    </div>
    <div class="result-canvas">
      <div v-if="reverseResult" class="reverse-number">¥ {{ reverseResult }}</div>
      <div v-else class="result-empty">
        <div class="phrase">Paste Chinese capitals. <b>Get the figure back.</b></div>
        <div class="hint">↳ 接受 圆/元 · 整/正 任一写法</div>
      </div>
    </div>
    <div class="result-foot">
      <div class="result-stats">
        <div class="stat"><span class="k">Stamp</span><span class="v">{{ reverseStamp || '— · —' }}</span></div>
      </div>
      <button class="copy-btn" :class="{ copied: reverseCopied }" :disabled="!reverseResult" @click="copyReverse">
        <span>{{ reverseCopied ? 'Copied to clipboard' : 'Copy number' }}</span>
        <span class="arrow">{{ reverseCopied ? '✓' : '→' }}</span>
      </button>
    </div>
  </div>

  <div class="right-col">
    <div class="panel">
      <div class="panel-head">
        <div class="label">INPUT · <b>中文大写</b></div>
        <span class="chip" :class="{ live: reverseInput.length > 0 }">
          <span class="d"></span>{{ reverseInput.length > 0 ? 'READY' : 'IDLE' }}
        </span>
      </div>
      <textarea
        v-model="reverseInput"
        class="reverse-textarea"
        rows="4"
        placeholder="例：壹仟贰佰叁拾肆圆伍角陆分"
      ></textarea>
      <div class="input-meta">
        <span v-if="reverseError" class="err">⚠ {{ reverseError }}</span>
        <span v-else>支持 圆 / 元 · 整 / 正 · 负数前缀</span>
        <span>F.0</span>
      </div>
      <button class="convert-btn" @click="reverseConvert" :disabled="reverseConverting || !reverseInput.trim()">
        <div style="display:flex; flex-direction:column; align-items:flex-start; gap:2px;">
          <span class="label-en">Parse Now</span>
          <span>{{ reverseConverting ? '还原中…' : '还 原' }}</span>
        </div>
        <span class="icon">
          <span v-if="reverseConverting" class="spinner"></span>
          <span v-else>↩</span>
        </span>
      </button>
    </div>
  </div>
</section>
```

- [ ] **Step 2: Add reverse-specific CSS**

Append to the `<style>` block:

```css
.reverse-number {
  font-family: 'JetBrains Mono', monospace;
  font-weight: 300;
  font-size: clamp(48px, 7vw, 96px);
  color: var(--fg);
  letter-spacing: -0.03em;
  line-height: 1;
  padding: 16px 0;
}
.reverse-textarea {
  width: 100%;
  background: transparent;
  border: 1px solid var(--hairline-strong);
  border-radius: 12px;
  color: var(--fg);
  font-family: 'Noto Serif SC', serif;
  font-size: 18px;
  letter-spacing: 0.04em;
  line-height: 1.6;
  padding: 16px;
  resize: vertical;
  outline: none;
  transition: border-color 0.3s;
  margin-bottom: 14px;
}
.reverse-textarea:focus { border-color: var(--accent); }
.reverse-textarea::placeholder { color: var(--muted-2); }
```

- [ ] **Step 3: Add `reverseConvert` and `copyReverse` methods**

Inside Vue `methods: {}`, add:

```js
async reverseConvert() {
  const v = this.reverseInput.trim();
  if (!v || this.reverseConverting) return;
  this.reverseConverting = true;
  this.reverseError = '';
  try {
    const res = await axios.post('/api/convert/reverse', { chinese: v });
    this.reverseResult = res.data.amount;
    this.reverseStamp = this.stampNow();
    this.reverseCopied = false;
  } catch (e) {
    this.reverseError = e.response?.data?.message || e.response?.data?.error || '解析失败';
    this.reverseResult = '';
  } finally {
    this.reverseConverting = false;
  }
},
copyReverse() {
  if (!this.reverseResult) return;
  navigator.clipboard.writeText(this.reverseResult).then(() => {
    this.reverseCopied = true;
    setTimeout(() => { this.reverseCopied = false; }, 2200);
  }).catch(() => { this.reverseError = '复制失败，请手动选择'; });
},
```

- [ ] **Step 4: Manual smoke check**

Run: `go run .`
- Switch to "还 原" tab
- Paste `壹仟贰佰叁拾肆圆伍角陆分` → click 还 原 → result shows `¥ 1234.56`
- Paste `壹仟元正` → result shows `¥ 1000.00` (圆/元 兼容)
- Paste `garbage` → error message displayed in input-meta
- Copy button works

- [ ] **Step 5: Commit**

```bash
git add static/index.html
git commit -m "feat(ui): wire reverse tab to /api/convert/reverse"
```

---

## Task 10: Implement Verify Tab — UI + diff visualization

**Files:**
- Modify: `static/index.html`

Replace the `verify` placeholder. Right side: number input + Chinese textarea + 校 验 button. Left side: ✓/✗ verdict, plus a two-row diff display on mismatch (your input vs system-computed, highlighting the first differing character).

- [ ] **Step 1: Replace the verify placeholder section**

Find `<section v-else-if="activeTab === 'verify'" ...>` and replace with:

```html
<section v-else-if="activeTab === 'verify'" class="workspace">
  <div class="panel result-panel">
    <div class="panel-head">
      <div class="label">VERDICT · <b>校验结论</b></div>
      <span class="chip" :class="{ ok: verifyResult && verifyResult.match }">
        <span class="d"></span>
        {{ verifyResult ? (verifyResult.match ? 'MATCH' : 'MISMATCH') : 'AWAITING' }}
      </span>
    </div>
    <div class="result-canvas">
      <div v-if="!verifyResult" class="result-empty">
        <div class="phrase">Submit both. <b>See if they agree.</b></div>
        <div class="hint">↳ 系统会计算并对比，定位首个差异字</div>
      </div>
      <div v-else-if="verifyResult.match" class="verify-match">
        <div class="verify-icon">✓</div>
        <div class="verify-label">M A T C H</div>
        <div class="verify-canonical">{{ verifyResult.expected }}</div>
      </div>
      <div v-else class="verify-mismatch">
        <div class="verify-icon mis">✗</div>
        <div class="verify-label">M I S M A T C H</div>
        <div v-if="verifyResult.message" class="verify-msg">{{ verifyResult.message }}</div>
        <div class="verify-diff-row">
          <span class="verify-diff-label">你 输 入 的</span>
          <div class="verify-diff-text">
            <span
              v-for="(ch, i) in verifyChinese"
              :key="'a' + i"
              :class="{ 'diff-char': i === verifyResult.diffAt }"
            >{{ ch }}</span>
          </div>
        </div>
        <div class="verify-diff-row">
          <span class="verify-diff-label">系 统 计 算</span>
          <div class="verify-diff-text">
            <span
              v-for="(ch, i) in verifyResult.expected"
              :key="'b' + i"
              :class="{ 'diff-char': i === verifyResult.diffAt }"
            >{{ ch }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div class="right-col">
    <div class="panel">
      <div class="panel-head">
        <div class="label">INPUT · <b>双向输入</b></div>
        <span class="chip" :class="{ live: verifyAmount && verifyChinese }">
          <span class="d"></span>{{ (verifyAmount && verifyChinese) ? 'READY' : 'IDLE' }}
        </span>
      </div>
      <label class="sublabel">阿拉伯数字</label>
      <div class="input-wrap">
        <span class="currency">¥</span>
        <input
          v-model="verifyAmount"
          type="text"
          inputmode="decimal"
          placeholder="0.00"
          autocomplete="off"
          class="verify-num-input"
        >
      </div>
      <label class="sublabel">中文大写</label>
      <textarea
        v-model="verifyChinese"
        class="reverse-textarea"
        rows="3"
        placeholder="例：壹仟贰佰叁拾肆圆伍角陆分"
      ></textarea>
      <div class="input-meta">
        <span v-if="verifyError" class="err">⚠ {{ verifyError }}</span>
        <span v-else>两栏均需填写</span>
        <span>F.2</span>
      </div>
      <button class="convert-btn" @click="verifyRun" :disabled="verifyConverting || !verifyAmount || !verifyChinese">
        <div style="display:flex; flex-direction:column; align-items:flex-start; gap:2px;">
          <span class="label-en">Cross-check Now</span>
          <span>{{ verifyConverting ? '校验中…' : '校 验' }}</span>
        </div>
        <span class="icon">
          <span v-if="verifyConverting" class="spinner"></span>
          <span v-else>⇄</span>
        </span>
      </button>
    </div>
  </div>
</section>
```

- [ ] **Step 2: Add verify-specific CSS**

Append to the `<style>` block:

```css
.verify-match, .verify-mismatch {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 12px 0;
}
.verify-icon {
  width: 80px; height: 80px;
  border-radius: 50%;
  background: var(--accent);
  color: var(--accent-fg);
  display: grid;
  place-items: center;
  font-size: 44px;
  font-family: 'Bricolage Grotesque', sans-serif;
  font-weight: 700;
  box-shadow: 0 0 48px var(--accent-glow);
}
.verify-icon.mis {
  background: var(--warn);
  color: #fff;
  box-shadow: 0 0 48px rgba(255, 122, 92, 0.32);
}
.verify-label {
  font-family: 'Bricolage Grotesque', sans-serif;
  font-weight: 700;
  font-size: 32px;
  letter-spacing: 0.3em;
  color: var(--fg);
}
.verify-canonical {
  font-family: 'Noto Serif SC', serif;
  font-weight: 500;
  font-size: 22px;
  color: var(--fg-2);
  line-height: 1.6;
  letter-spacing: 0.04em;
  word-break: break-all;
}
.verify-msg {
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  color: var(--warn);
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.verify-diff-row {
  display: grid;
  grid-template-columns: 100px 1fr;
  gap: 16px;
  align-items: baseline;
}
.verify-diff-label {
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  color: var(--muted);
  letter-spacing: 0.22em;
  text-transform: uppercase;
}
.verify-diff-text {
  font-family: 'Noto Serif SC', serif;
  font-size: 22px;
  line-height: 1.5;
  color: var(--fg-2);
  letter-spacing: 0.04em;
  word-break: break-all;
}
.verify-diff-text .diff-char {
  color: var(--accent);
  font-weight: 700;
  border-bottom: 2px solid var(--accent);
  padding: 0 2px;
}
.sublabel {
  display: block;
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  color: var(--muted);
  letter-spacing: 0.2em;
  text-transform: uppercase;
  margin-top: 16px;
  margin-bottom: 6px;
}
.verify-num-input { font-size: 28px !important; }
```

- [ ] **Step 3: Add `verifyRun` method**

Inside Vue `methods: {}`, add:

```js
async verifyRun() {
  if (!this.verifyAmount || !this.verifyChinese || this.verifyConverting) return;
  this.verifyConverting = true;
  this.verifyError = '';
  this.verifyResult = null;
  try {
    const res = await axios.post('/api/convert/verify', {
      amount: this.verifyAmount,
      chinese: this.verifyChinese,
    });
    this.verifyResult = res.data;
  } catch (e) {
    this.verifyError = e.response?.data?.message || e.response?.data?.error || '校验失败';
  } finally {
    this.verifyConverting = false;
  }
},
```

- [ ] **Step 4: Manual smoke check**

Run: `go run .`
- Switch to "校 验"
- Match case: amount `1234.56`, chinese `壹仟贰佰叁拾肆圆伍角陆分` → green ✓ MATCH
- Mismatch case: amount `1234.56`, chinese `壹仟贰佰叁拾肆圆伍角柒分` → red ✗ MISMATCH, first differing char (陆 vs 柒) highlighted
- Unparsable case: amount `1234.56`, chinese `garbage` → ✗ MISMATCH with message line
- Invalid amount: amount `abc`, chinese anything → error in meta line

- [ ] **Step 5: Commit**

```bash
git add static/index.html
git commit -m "feat(ui): wire verify tab with diff visualization"
```

---

## Task 11: Implement Batch Tab — UI + client-side line counting

**Files:**
- Modify: `static/index.html`

Replace the `batch` placeholder. Right side: multi-line textarea + live `N / 200` counter that turns warn when exceeded. Left side: result rows aligned to source lines with per-row copy + "复制全部 TSV" + "导出 CSV" buttons.

- [ ] **Step 1: Replace the batch placeholder section**

Find `<section v-else-if="activeTab === 'batch'" ...>` and replace with:

```html
<section v-else-if="activeTab === 'batch'" class="workspace">
  <div class="panel result-panel">
    <div class="panel-head">
      <div class="label">OUTPUT · <b>批量结果</b></div>
      <span class="chip" :class="{ ok: batchResults.length > 0 }">
        <span class="d"></span>
        {{ batchValidCount }} valid · {{ batchErrorCount }} errors
      </span>
    </div>
    <div class="batch-list" v-if="batchResults.length">
      <div v-for="(row, i) in batchResults" :key="i" class="batch-row" :class="{ failed: row.error }">
        <span class="batch-idx">#{{ String(i+1).padStart(3,'0') }}</span>
        <span class="batch-src">¥ {{ row.amount }}</span>
        <span class="batch-arrow">→</span>
        <span class="batch-dst">{{ row.chinese || row.message || row.error }}</span>
        <button class="batch-copy" @click="copyOneBatch(row)" :disabled="!row.chinese" title="复制本行">⧉</button>
      </div>
    </div>
    <div v-else class="result-empty" style="padding: 24px 0;">
      <div class="phrase">Paste many. <b>Get them all back.</b></div>
      <div class="hint">↳ 每行一个金额，最多 200 条</div>
    </div>
    <div class="result-foot" v-if="batchResults.length">
      <div class="result-stats">
        <div class="stat"><span class="k">Total</span><span class="v">{{ batchResults.length }}</span></div>
      </div>
      <div style="display: flex; gap: 12px;">
        <button class="copy-btn" @click="copyAllBatchTSV" :disabled="!batchValidCount">
          <span>Copy TSV</span><span class="arrow">⧉</span>
        </button>
        <button class="copy-btn" @click="exportBatchCSV" :disabled="!batchValidCount">
          <span>Export CSV</span><span class="arrow">↓</span>
        </button>
      </div>
    </div>
  </div>

  <div class="right-col">
    <div class="panel">
      <div class="panel-head">
        <div class="label">INPUT · <b>多行金额</b></div>
        <span class="chip" :class="{ live: batchLineCount > 0 && batchLineCount <= 200, ok: batchLineCount > 200 ? false : batchLineCount > 0 }">
          <span class="d"></span>
          <span :class="{ 'batch-count-over': batchLineCount > 200 }">{{ batchLineCount }} / 200</span>
        </span>
      </div>
      <textarea
        v-model="batchInput"
        class="reverse-textarea batch-textarea"
        :class="{ 'over-limit': batchLineCount > 200 }"
        rows="10"
        placeholder="每行一个金额&#10;例：&#10;1234.56&#10;1000&#10;-50.25"
      ></textarea>
      <div class="input-meta">
        <span v-if="batchLineCount > 200" class="err">⚠ 已粘贴 {{ batchLineCount }} 条，请删除 {{ batchLineCount - 200 }} 条后再继续</span>
        <span v-else-if="batchError" class="err">⚠ {{ batchError }}</span>
        <span v-else>每行解析为一条独立金额</span>
        <span>F.M</span>
      </div>
      <button class="convert-btn" @click="batchRun" :disabled="batchConverting || batchLineCount === 0 || batchLineCount > 200">
        <div style="display:flex; flex-direction:column; align-items:flex-start; gap:2px;">
          <span class="label-en">Transcribe {{ batchLineCount }} entries</span>
          <span>{{ batchConverting ? '批量转写中…' : '批 量 转 写' }}</span>
        </div>
        <span class="icon">
          <span v-if="batchConverting" class="spinner"></span>
          <span v-else>↗</span>
        </span>
      </button>
    </div>
  </div>
</section>
```

- [ ] **Step 2: Add batch-specific CSS**

Append to the `<style>` block:

```css
.batch-textarea.over-limit { border-color: var(--warn); }
.batch-count-over { color: var(--warn); }
.batch-list {
  max-height: 480px;
  overflow-y: auto;
  margin: 16px 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.batch-row {
  display: grid;
  grid-template-columns: 60px 110px 20px 1fr 36px;
  gap: 10px;
  align-items: center;
  padding: 12px;
  border-radius: 10px;
  background: var(--surface);
  border: 1px solid var(--hairline);
  font-size: 14px;
}
.batch-row.failed {
  border-color: var(--warn);
  background: color-mix(in srgb, var(--warn) 8%, var(--surface));
}
.batch-row.failed .batch-dst { color: var(--warn); }
.batch-idx {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  color: var(--muted);
  letter-spacing: 0.12em;
}
.batch-src {
  font-family: 'JetBrains Mono', monospace;
  font-size: 14px;
  color: var(--fg-2);
}
.batch-arrow {
  font-family: 'JetBrains Mono', monospace;
  color: var(--muted);
}
.batch-dst {
  font-family: 'Noto Serif SC', serif;
  font-size: 15px;
  color: var(--fg);
  letter-spacing: 0.04em;
  word-break: break-all;
}
.batch-copy {
  background: transparent;
  border: 1px solid var(--hairline-strong);
  border-radius: 6px;
  color: var(--fg-2);
  cursor: pointer;
  font-size: 14px;
  width: 32px; height: 28px;
  transition: all 0.2s;
}
.batch-copy:hover:not(:disabled) {
  background: var(--accent);
  color: var(--accent-fg);
  border-color: var(--accent);
}
.batch-copy:disabled { opacity: 0.3; cursor: not-allowed; }
```

- [ ] **Step 3: Add batch computed + methods**

Inside `computed`, add:

```js
batchLineCount() {
  if (!this.batchInput) return 0;
  return this.batchInput.split('\n').filter(l => l.trim().length > 0).length;
},
batchValidCount() {
  return this.batchResults.filter(r => !r.error).length;
},
batchErrorCount() {
  return this.batchResults.filter(r => r.error).length;
},
```

Inside `methods`, add:

```js
async batchRun() {
  const amounts = this.batchInput.split('\n').map(l => l.trim()).filter(l => l.length > 0);
  if (amounts.length === 0 || amounts.length > 200 || this.batchConverting) return;
  this.batchConverting = true;
  this.batchError = '';
  try {
    const res = await axios.post('/api/convert/batch', { amounts });
    this.batchResults = res.data.results;
  } catch (e) {
    this.batchError = e.response?.data?.message || e.response?.data?.error || '批量转写失败';
    this.batchResults = [];
  } finally {
    this.batchConverting = false;
  }
},
copyOneBatch(row) {
  if (!row.chinese) return;
  navigator.clipboard.writeText(row.chinese).catch(() => {});
},
copyAllBatchTSV() {
  const tsv = this.batchResults
    .filter(r => r.chinese)
    .map(r => r.amount + '\t' + r.chinese)
    .join('\n');
  navigator.clipboard.writeText(tsv).catch(() => {});
},
exportBatchCSV() {
  const header = 'amount,chinese\n';
  const rows = this.batchResults
    .filter(r => r.chinese)
    .map(r => '"' + r.amount + '","' + r.chinese.replace(/"/g, '""') + '"')
    .join('\n');
  const blob = new Blob([header + rows], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'rmb-batch-' + Date.now() + '.csv';
  a.click();
  URL.revokeObjectURL(url);
},
```

- [ ] **Step 4: Manual smoke check**

Run: `go run .`
- Switch to "批 量"
- Paste 3 lines: `1234.56` / `1000` / `abc` → click 批量转写 → 3 result rows, third in warn red with `invalid_format` message
- Paste 205 lines (e.g. paste any short line repeated): counter shows `205 / 200` in warn red, button disabled, warning shown
- TSV copy and CSV export work

- [ ] **Step 5: Commit**

```bash
git add static/index.html
git commit -m "feat(ui): wire batch tab with client-side line counting"
```

---

## Task 12: Author `static/openapi.json` (full OpenAPI 3.1 spec)

**Files:**
- Modify: `static/openapi.json` (replace placeholder with full spec)

- [ ] **Step 1: Write the full spec**

Replace the contents of `static/openapi.json` with:

```json
{
  "openapi": "3.1.0",
  "info": {
    "title": "RMB Capital · Numeral Transcriber API",
    "version": "1.0.0",
    "description": "Convert between Arabic numerals and Chinese capitalised numerals (人民币大写) for invoices, contracts, and bank instruments."
  },
  "servers": [
    { "url": "http://localhost:8080", "description": "Local dev" }
  ],
  "paths": {
    "/api/convert": {
      "post": {
        "summary": "Convert number → Chinese capitalised form",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/ConvertRequest" },
              "example": { "amount": "1234.56" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/ConvertResponse" },
                "example": { "chinese": "壹仟贰佰叁拾肆圆伍角陆分" }
              }
            }
          },
          "400": {
            "description": "Bad Request",
            "content": {
              "application/json": { "schema": { "$ref": "#/components/schemas/Error" } }
            }
          }
        }
      }
    },
    "/api/convert/reverse": {
      "post": {
        "summary": "Convert Chinese capitalised form → number",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/ReverseRequest" },
              "example": { "chinese": "壹仟贰佰叁拾肆圆伍角陆分" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/ReverseResponse" },
                "example": { "amount": "1234.56" }
              }
            }
          },
          "400": {
            "description": "Bad Request",
            "content": {
              "application/json": { "schema": { "$ref": "#/components/schemas/Error" } }
            }
          }
        }
      }
    },
    "/api/convert/verify": {
      "post": {
        "summary": "Bidirectional consistency check",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/VerifyRequest" },
              "example": { "amount": "1234.56", "chinese": "壹仟贰佰叁拾肆圆伍角陆分" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/VerifyResponse" }
              }
            }
          },
          "400": {
            "description": "Bad Request (invalid amount)",
            "content": {
              "application/json": { "schema": { "$ref": "#/components/schemas/Error" } }
            }
          }
        }
      }
    },
    "/api/convert/batch": {
      "post": {
        "summary": "Batch convert up to 200 amounts",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/BatchRequest" },
              "example": { "amounts": ["1234.56", "1000", "abc"] }
            }
          }
        },
        "responses": {
          "200": {
            "description": "OK (per-item errors reported in results array)",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/BatchResponse" }
              }
            }
          },
          "413": {
            "description": "Batch exceeds 200 items",
            "content": {
              "application/json": { "schema": { "$ref": "#/components/schemas/Error" } }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "ConvertRequest": {
        "type": "object", "required": ["amount"],
        "properties": {
          "amount": { "type": "string", "pattern": "^-?\\d+(\\.\\d{1,2})?$", "description": "0 to 999,999,999,999.99; optional leading minus" }
        }
      },
      "ConvertResponse": {
        "type": "object", "required": ["chinese"],
        "properties": { "chinese": { "type": "string" } }
      },
      "ReverseRequest": {
        "type": "object", "required": ["chinese"],
        "properties": { "chinese": { "type": "string", "description": "Accepts 圆/元 and 整/正 interchangeably" } }
      },
      "ReverseResponse": {
        "type": "object", "required": ["amount"],
        "properties": { "amount": { "type": "string" } }
      },
      "VerifyRequest": {
        "type": "object", "required": ["amount", "chinese"],
        "properties": {
          "amount": { "type": "string" },
          "chinese": { "type": "string" }
        }
      },
      "VerifyResponse": {
        "type": "object", "required": ["match", "expected"],
        "properties": {
          "match": { "type": "boolean" },
          "expected": { "type": "string", "description": "Canonical form computed by server" },
          "diffAt": { "type": "integer", "description": "First differing rune index (mismatch only)" },
          "message": { "type": "string", "description": "Parse failure detail (mismatch only)" }
        }
      },
      "BatchRequest": {
        "type": "object", "required": ["amounts"],
        "properties": {
          "amounts": { "type": "array", "items": { "type": "string" }, "maxItems": 200 }
        }
      },
      "BatchItem": {
        "type": "object", "required": ["amount"],
        "properties": {
          "amount": { "type": "string" },
          "chinese": { "type": "string" },
          "error": { "type": "string", "enum": ["invalid_format", "out_of_range"] },
          "message": { "type": "string" }
        }
      },
      "BatchResponse": {
        "type": "object", "required": ["results"],
        "properties": {
          "results": { "type": "array", "items": { "$ref": "#/components/schemas/BatchItem" } }
        }
      },
      "Error": {
        "type": "object", "required": ["error", "message"],
        "properties": {
          "error": { "type": "string", "enum": ["invalid_format", "out_of_range", "unparsable_chinese", "batch_too_large"] },
          "message": { "type": "string" },
          "at": { "type": "integer" }
        }
      }
    }
  }
}
```

- [ ] **Step 2: Validate JSON**

Run: `python3 -m json.tool static/openapi.json > /dev/null && echo OK`
Expected: `OK`.

- [ ] **Step 3: Verify served at `/openapi.json`**

Run: `go run . &`
Run: `curl -s http://localhost:8080/openapi.json | python3 -m json.tool | head -10`
Expected: starts with `{"openapi": "3.1.0", ...}`.
Stop server: `kill %1`.

- [ ] **Step 4: Commit**

```bash
git add static/openapi.json
git commit -m "feat(docs): author OpenAPI 3.1 spec for all 4 endpoints"
```

---

## Task 13: Create `static/swagger.html` (CDN-loaded Swagger UI)

**Files:**
- Create: `static/swagger.html`

- [ ] **Step 1: Create the file**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>RMB Capital · OpenAPI Reference</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #fafafa; }
    .topbar-back {
      position: sticky;
      top: 0;
      z-index: 100;
      background: #1b1b1b;
      color: #fff;
      padding: 12px 24px;
      font-family: -apple-system, BlinkMacSystemFont, sans-serif;
      font-size: 13px;
      letter-spacing: 0.12em;
      text-transform: uppercase;
    }
    .topbar-back a { color: #d4ff5e; text-decoration: none; margin-right: 16px; }
    .topbar-back a:hover { text-decoration: underline; }
  </style>
</head>
<body>
  <div class="topbar-back">
    <a href="/docs">← Docs</a>
    <a href="/">← Home</a>
    <span>Swagger UI · /openapi.json</span>
  </div>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      SwaggerUIBundle({
        url: '/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
      });
    };
  </script>
</body>
</html>
```

- [ ] **Step 2: Smoke check**

Run: `go run .`
Open `http://localhost:8080/docs/spec` in a browser. Verify Swagger UI loads all 4 endpoints, "Try it out" works for `/api/convert`.

- [ ] **Step 3: Commit**

```bash
git add static/swagger.html
git commit -m "feat(docs): add Swagger UI wrapper at /docs/spec"
```

---

## Task 14: Author `static/docs.html` (hand-written API reference)

**Files:**
- Create: `static/docs.html`

This is the largest single static file. Structure: NAV + sidebar nav + sections for overview, error codes, each endpoint (with curl/Python/Node/Go samples and a Try It debugger).

- [ ] **Step 1: Create skeleton with NAV + sidebar + theme bootstrap**

Create `static/docs.html` with the framing structure:

```html
<!DOCTYPE html>
<html lang="zh-CN" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>RMB Capital · API Reference</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Bricolage+Grotesque:opsz,wght@12..96,400;12..96,500;12..96,600;12..96,700&family=JetBrains+Mono:wght@300;400;500&family=Noto+Serif+SC:wght@500&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="/static/shared/theme.css">
  <script>
    (function () {
      const saved = localStorage.getItem('theme');
      const sys = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
      document.documentElement.setAttribute('data-theme', saved || sys);
    })();
  </script>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg);
      color: var(--fg);
      font-family: 'Bricolage Grotesque', system-ui, sans-serif;
      -webkit-font-smoothing: antialiased;
      letter-spacing: -0.005em;
      transition: background 0.4s, color 0.4s;
    }
    .layout { display: grid; grid-template-columns: 240px 1fr; min-height: 100vh; }
    .sidebar {
      position: sticky; top: 0; height: 100vh;
      border-right: 1px solid var(--hairline);
      padding: 32px 24px;
      background: var(--bg);
      overflow-y: auto;
    }
    .side-logo {
      display: flex; align-items: center; gap: 10px; margin-bottom: 32px;
    }
    .side-logo .mark {
      width: 28px; height: 28px; border-radius: 8px;
      background: var(--accent); color: var(--accent-fg);
      display: grid; place-items: center;
      font-weight: 700; font-size: 14px;
    }
    .side-logo b { font-size: 14px; letter-spacing: -0.02em; }
    .side-section {
      font-family: 'JetBrains Mono', monospace;
      font-size: 10px; letter-spacing: 0.24em; text-transform: uppercase;
      color: var(--muted);
      margin: 18px 0 8px;
    }
    .side-link {
      display: block; padding: 6px 10px; border-radius: 6px;
      color: var(--fg-2); text-decoration: none;
      font-size: 13px;
      transition: background 0.2s, color 0.2s;
    }
    .side-link:hover { background: var(--surface); color: var(--fg); }
    .side-link.api { font-family: 'JetBrains Mono', monospace; font-size: 11px; }
    .side-spacer { flex-grow: 1; }
    .main { padding: 32px 56px 80px; max-width: 980px; }
    h1 {
      font-size: 48px; font-weight: 600; letter-spacing: -0.04em;
      margin-bottom: 12px;
    }
    h1 em { color: var(--accent); font-style: normal; font-weight: 700; }
    .lead { color: var(--fg-2); font-size: 17px; line-height: 1.55; margin-bottom: 48px; max-width: 640px; }
    h2 {
      font-size: 28px; font-weight: 600; letter-spacing: -0.03em;
      margin: 56px 0 16px;
      padding-top: 16px;
      border-top: 1px solid var(--hairline);
    }
    h3 {
      font-family: 'JetBrains Mono', monospace;
      font-size: 14px; font-weight: 500; letter-spacing: 0.04em;
      margin: 24px 0 10px;
      color: var(--fg);
    }
    p { color: var(--fg-2); line-height: 1.65; margin-bottom: 12px; max-width: 720px; }
    code, pre {
      font-family: 'JetBrains Mono', monospace;
      font-size: 13px;
    }
    code { color: var(--accent); background: var(--surface); padding: 2px 6px; border-radius: 4px; }
    pre {
      background: var(--surface); border: 1px solid var(--hairline);
      border-radius: 12px; padding: 18px; overflow-x: auto;
      line-height: 1.55; color: var(--fg);
    }
    pre code { background: none; color: inherit; padding: 0; }
    table {
      width: 100%; border-collapse: collapse; margin: 16px 0;
      font-size: 13px;
    }
    th, td {
      text-align: left; padding: 10px 12px;
      border-bottom: 1px solid var(--hairline);
    }
    th {
      font-family: 'JetBrains Mono', monospace;
      font-weight: 500; font-size: 11px; letter-spacing: 0.2em;
      text-transform: uppercase; color: var(--muted);
    }
    td code { font-size: 12px; }
    .endpoint-head {
      display: flex; align-items: center; gap: 12px;
      margin: 56px 0 8px; padding-top: 16px;
      border-top: 1px solid var(--hairline);
    }
    .method {
      display: inline-block; padding: 4px 8px; border-radius: 6px;
      background: var(--accent); color: var(--accent-fg);
      font-family: 'JetBrains Mono', monospace;
      font-size: 11px; font-weight: 700; letter-spacing: 0.1em;
    }
    .endpoint-path {
      font-family: 'JetBrains Mono', monospace; font-size: 18px;
      color: var(--fg);
    }
    .lang-tabs { display: flex; gap: 4px; margin: 12px 0 8px; }
    .lang-tab {
      background: var(--surface); border: 1px solid var(--hairline);
      color: var(--fg-2); padding: 6px 14px; border-radius: 8px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 11px; letter-spacing: 0.1em; cursor: pointer;
      transition: all 0.2s;
    }
    .lang-tab.active { background: var(--accent); color: var(--accent-fg); border-color: var(--accent); }
    .try-it {
      margin-top: 16px; padding: 18px;
      background: var(--surface); border: 1px solid var(--hairline);
      border-radius: 12px;
    }
    .try-it h4 {
      font-family: 'JetBrains Mono', monospace;
      font-size: 11px; letter-spacing: 0.22em; text-transform: uppercase;
      color: var(--muted); margin-bottom: 12px;
    }
    .try-it textarea {
      width: 100%; min-height: 90px;
      background: var(--bg); border: 1px solid var(--hairline);
      border-radius: 8px; padding: 12px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 12px; color: var(--fg);
      resize: vertical; outline: none;
    }
    .try-it textarea:focus { border-color: var(--accent); }
    .try-it-controls { display: flex; gap: 10px; align-items: center; margin-top: 10px; }
    .try-send {
      background: var(--accent); color: var(--accent-fg);
      border: none; padding: 8px 18px; border-radius: 8px;
      font-family: 'Bricolage Grotesque', sans-serif;
      font-weight: 600; cursor: pointer;
    }
    .try-status {
      font-family: 'JetBrains Mono', monospace;
      font-size: 11px; color: var(--muted); letter-spacing: 0.1em;
    }
    .try-response {
      margin-top: 12px;
      background: var(--bg); border: 1px solid var(--hairline);
      border-radius: 8px; padding: 12px;
      font-family: 'JetBrains Mono', monospace; font-size: 12px;
      max-height: 240px; overflow-y: auto;
      white-space: pre-wrap; word-break: break-all;
    }
    .top-actions {
      display: flex; gap: 12px; margin-bottom: 32px;
    }
    .top-action {
      padding: 8px 14px; border: 1px solid var(--hairline-strong);
      border-radius: 999px; font-family: 'JetBrains Mono', monospace;
      font-size: 11px; letter-spacing: 0.2em; text-transform: uppercase;
      color: var(--fg-2); text-decoration: none;
      transition: all 0.2s;
    }
    .top-action:hover { background: var(--fg); color: var(--bg); border-color: var(--fg); }
    @media (max-width: 880px) {
      .layout { grid-template-columns: 1fr; }
      .sidebar { position: relative; height: auto; }
      .main { padding: 24px 20px 56px; }
    }
  </style>
</head>
<body>
  <div class="layout">
    <aside class="sidebar">
      <div class="side-logo"><span class="mark">¥</span><b>RMB Capital · Docs</b></div>
      <div class="side-section">Guide</div>
      <a class="side-link" href="#overview">Overview</a>
      <a class="side-link" href="#errors">Error codes</a>
      <a class="side-link" href="#reverse-notes">Reverse parsing</a>
      <div class="side-section">Endpoints</div>
      <a class="side-link api" href="#ep-convert">POST /api/convert</a>
      <a class="side-link api" href="#ep-reverse">POST /api/convert/reverse</a>
      <a class="side-link api" href="#ep-verify">POST /api/convert/verify</a>
      <a class="side-link api" href="#ep-batch">POST /api/convert/batch</a>
      <div class="side-section">Spec</div>
      <a class="side-link" href="/openapi.json" target="_blank">openapi.json ↗</a>
      <a class="side-link" href="/docs/spec">Swagger UI ↗</a>
    </aside>
    <main class="main">
      <div class="top-actions">
        <a class="top-action" href="/">← Home</a>
        <a class="top-action" href="/docs/spec">Swagger UI ↗</a>
      </div>
      <h1>API <em>Reference</em></h1>
      <p class="lead">Convert between Arabic numerals and Chinese capitalised numerals (人民币大写). Four endpoints, no auth, JSON only.</p>

      <!-- (sections inserted in Steps 2-5) -->
    </main>
  </div>
  <!-- (Try It script inserted in Step 6) -->
</body>
</html>
```

- [ ] **Step 2: Insert Overview + Error codes sections**

Replace the `<!-- (sections inserted in Steps 2-5) -->` comment with the following, then keep that comment marker at the new tail position for next steps:

```html
<section id="overview">
  <h2>Overview</h2>
  <p>Base URL: <code>http://localhost:8080</code> (development) — replace with your deployed origin.</p>
  <p>Authentication: none. All endpoints are open. Rate limiting and API keys are not implemented in v1.</p>
  <p>Content-Type: <code>application/json</code> for all requests and responses. UTF-8.</p>
</section>

<section id="errors">
  <h2>Error codes</h2>
  <p>All errors share one envelope: <code>{ "error": "&lt;code&gt;", "message": "&lt;human readable&gt;", "at": &lt;optional offset&gt; }</code>.</p>
  <table>
    <thead><tr><th>HTTP</th><th>code</th><th>Meaning</th></tr></thead>
    <tbody>
      <tr><td><code>400</code></td><td><code>invalid_format</code></td><td>Amount does not match <code>^-?\d+(\.\d{1,2})?$</code>, or JSON body malformed.</td></tr>
      <tr><td><code>400</code></td><td><code>out_of_range</code></td><td>Absolute value exceeds 999,999,999,999.99.</td></tr>
      <tr><td><code>400</code></td><td><code>unparsable_chinese</code></td><td>Chinese capitalised string could not be parsed; <code>at</code> field marks the offset where parsing failed.</td></tr>
      <tr><td><code>413</code></td><td><code>batch_too_large</code></td><td>Batch contains more than 200 entries.</td></tr>
    </tbody>
  </table>
</section>

<section id="reverse-notes">
  <h2>Reverse parsing behaviour</h2>
  <p>The reverse and verify endpoints accept either currency unit (圆 or 元) and either terminator (整 or 正) interchangeably. The forward and batch endpoints always emit canonical <code>圆 + 整</code> form (PBOC ledger convention).</p>
  <p>Negative amounts are supported across all endpoints: a leading <code>-</code> in the numeric form, a leading <code>负</code> in the Chinese form.</p>
</section>

<!-- (endpoint sections inserted in Steps 3-5) -->
```

- [ ] **Step 3: Insert the `/api/convert` endpoint section**

Replace the `<!-- (endpoint sections inserted in Steps 3-5) -->` comment with the following, keeping the marker at the new tail:

```html
<section id="ep-convert">
  <div class="endpoint-head">
    <span class="method">POST</span>
    <span class="endpoint-path">/api/convert</span>
  </div>
  <p>Convert an Arabic numeral amount to Chinese capitalised form.</p>

  <h3>Request</h3>
  <pre><code>{
  "amount": "1234.56"
}</code></pre>

  <h3>Response 200</h3>
  <pre><code>{
  "chinese": "壹仟贰佰叁拾肆圆伍角陆分"
}</code></pre>

  <h3>Code samples</h3>
  <div class="lang-tabs" data-target="sample-convert">
    <button class="lang-tab active" data-lang="curl">curl</button>
    <button class="lang-tab" data-lang="python">Python</button>
    <button class="lang-tab" data-lang="node">Node</button>
    <button class="lang-tab" data-lang="go">Go</button>
  </div>
  <pre id="sample-convert"><code class="sample" data-lang="curl">curl -X POST http://localhost:8080/api/convert \
  -H 'Content-Type: application/json' \
  -d '{"amount":"1234.56"}'</code></pre>
  <pre style="display:none" data-for="sample-convert"><code class="sample" data-lang="python">import requests

r = requests.post(
    "http://localhost:8080/api/convert",
    json={"amount": "1234.56"},
)
print(r.json()["chinese"])  # 壹仟贰佰叁拾肆圆伍角陆分</code></pre>
  <pre style="display:none" data-for="sample-convert"><code class="sample" data-lang="node">const res = await fetch("http://localhost:8080/api/convert", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ amount: "1234.56" }),
});
const data = await res.json();
console.log(data.chinese); // 壹仟贰佰叁拾肆圆伍角陆分</code></pre>
  <pre style="display:none" data-for="sample-convert"><code class="sample" data-lang="go">package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

func main() {
    body, _ := json.Marshal(map[string]string{"amount": "1234.56"})
    resp, _ := http.Post("http://localhost:8080/api/convert",
        "application/json", bytes.NewReader(body))
    defer resp.Body.Close()
    var out struct{ Chinese string `json:"chinese"` }
    json.NewDecoder(resp.Body).Decode(&out)
    fmt.Println(out.Chinese) // 壹仟贰佰叁拾肆圆伍角陆分
}</code></pre>

  <div class="try-it">
    <h4>Try it</h4>
    <textarea class="try-body" data-path="/api/convert">{"amount":"1234.56"}</textarea>
    <div class="try-it-controls">
      <button class="try-send" data-path="/api/convert">Send</button>
      <span class="try-status"></span>
    </div>
    <div class="try-response" hidden></div>
  </div>
</section>

<!-- (more endpoint sections inserted in Steps 4-5) -->
```

- [ ] **Step 4: Insert `/api/convert/reverse` and `/api/convert/verify` sections**

Replace the comment marker with two more sections (curl-only sample by default to keep the file size manageable — the full 4-language samples mirror the pattern in Step 3 so only show curl here):

```html
<section id="ep-reverse">
  <div class="endpoint-head">
    <span class="method">POST</span>
    <span class="endpoint-path">/api/convert/reverse</span>
  </div>
  <p>Parse a Chinese capitalised amount back into Arabic numerals. Accepts 圆/元 and 整/正 interchangeably.</p>
  <h3>Request</h3>
  <pre><code>{ "chinese": "壹仟贰佰叁拾肆圆伍角陆分" }</code></pre>
  <h3>Response 200</h3>
  <pre><code>{ "amount": "1234.56" }</code></pre>
  <h3>Error 400 (unparsable)</h3>
  <pre><code>{
  "error": "unparsable_chinese",
  "message": "input does not match expected pattern",
  "at": 0
}</code></pre>
  <h3>Sample (curl)</h3>
  <pre><code>curl -X POST http://localhost:8080/api/convert/reverse \
  -H 'Content-Type: application/json' \
  -d '{"chinese":"壹仟元正"}'</code></pre>
  <div class="try-it">
    <h4>Try it</h4>
    <textarea class="try-body" data-path="/api/convert/reverse">{"chinese":"壹仟元正"}</textarea>
    <div class="try-it-controls">
      <button class="try-send" data-path="/api/convert/reverse">Send</button>
      <span class="try-status"></span>
    </div>
    <div class="try-response" hidden></div>
  </div>
</section>

<section id="ep-verify">
  <div class="endpoint-head">
    <span class="method">POST</span>
    <span class="endpoint-path">/api/convert/verify</span>
  </div>
  <p>Compare a numeric amount and a Chinese capitalised string. Always returns 200 unless the numeric form itself is invalid.</p>
  <h3>Request</h3>
  <pre><code>{
  "amount": "1234.56",
  "chinese": "壹仟贰佰叁拾肆圆伍角陆分"
}</code></pre>
  <h3>Response 200 (match)</h3>
  <pre><code>{ "match": true, "expected": "壹仟贰佰叁拾肆圆伍角陆分" }</code></pre>
  <h3>Response 200 (mismatch)</h3>
  <pre><code>{
  "match": false,
  "expected": "壹仟贰佰叁拾肆圆伍角陆分",
  "diffAt": 9,
  "message": ""
}</code></pre>
  <div class="try-it">
    <h4>Try it</h4>
    <textarea class="try-body" data-path="/api/convert/verify">{"amount":"1234.56","chinese":"壹仟贰佰叁拾肆圆伍角陆分"}</textarea>
    <div class="try-it-controls">
      <button class="try-send" data-path="/api/convert/verify">Send</button>
      <span class="try-status"></span>
    </div>
    <div class="try-response" hidden></div>
  </div>
</section>

<!-- (batch section inserted in Step 5) -->
```

- [ ] **Step 5: Insert `/api/convert/batch` section**

Replace the marker with:

```html
<section id="ep-batch">
  <div class="endpoint-head">
    <span class="method">POST</span>
    <span class="endpoint-path">/api/convert/batch</span>
  </div>
  <p>Convert up to 200 amounts in one call. Per-item errors are reported inside the results array; the overall request still returns 200. Exceeding 200 entries returns 413.</p>
  <h3>Request</h3>
  <pre><code>{
  "amounts": ["1234.56", "1000", "abc"]
}</code></pre>
  <h3>Response 200</h3>
  <pre><code>{
  "results": [
    { "amount": "1234.56", "chinese": "壹仟贰佰叁拾肆圆伍角陆分" },
    { "amount": "1000",    "chinese": "壹仟圆整" },
    { "amount": "abc",     "error": "invalid_format", "message": "amount must match ^-?\\d+(\\.\\d{1,2})?$" }
  ]
}</code></pre>
  <h3>Error 413</h3>
  <pre><code>{
  "error": "batch_too_large",
  "message": "batch size exceeds 200"
}</code></pre>
  <div class="try-it">
    <h4>Try it</h4>
    <textarea class="try-body" data-path="/api/convert/batch">{"amounts":["1234.56","1000","abc"]}</textarea>
    <div class="try-it-controls">
      <button class="try-send" data-path="/api/convert/batch">Send</button>
      <span class="try-status"></span>
    </div>
    <div class="try-response" hidden></div>
  </div>
</section>
```

- [ ] **Step 6: Insert the Try It + language Tab script**

Replace the `<!-- (Try It script inserted in Step 6) -->` comment with:

```html
<script>
  // Language tab switching
  document.querySelectorAll('.lang-tabs').forEach(group => {
    const target = group.dataset.target;
    const tabs = group.querySelectorAll('.lang-tab');
    tabs.forEach(tab => {
      tab.addEventListener('click', () => {
        tabs.forEach(t => t.classList.remove('active'));
        tab.classList.add('active');
        const lang = tab.dataset.lang;
        const mainBlock = document.getElementById(target);
        const allBlocks = [mainBlock, ...document.querySelectorAll('[data-for="' + target + '"]')];
        allBlocks.forEach(b => {
          const code = b.querySelector('code.sample');
          if (!code) return;
          b.style.display = (code.dataset.lang === lang) ? '' : 'none';
        });
      });
    });
  });

  // Try It
  document.querySelectorAll('.try-send').forEach(btn => {
    btn.addEventListener('click', async () => {
      const path = btn.dataset.path;
      const card = btn.closest('.try-it');
      const body = card.querySelector('.try-body').value;
      const status = card.querySelector('.try-status');
      const out = card.querySelector('.try-response');
      out.hidden = false;
      status.textContent = 'sending…';
      const t0 = performance.now();
      try {
        const res = await fetch(path, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: body,
        });
        const dt = Math.round(performance.now() - t0);
        const text = await res.text();
        let pretty = text;
        try { pretty = JSON.stringify(JSON.parse(text), null, 2); } catch (_) {}
        status.textContent = res.status + ' · ' + dt + 'ms';
        out.textContent = pretty;
      } catch (e) {
        status.textContent = 'failed';
        out.textContent = String(e);
      }
    });
  });
</script>
```

- [ ] **Step 7: Smoke check**

Run: `go run .`
Open `http://localhost:8080/docs`. Verify:
- Sidebar nav, all 4 endpoint sections render
- Light/dark mode picks up from main app's localStorage
- Language tabs on the `/api/convert` section switch the visible code block
- "Try it" sends a real request and shows `200 · 12ms` + the pretty-printed JSON
- Sidebar links to `/openapi.json` and `/docs/spec` work

- [ ] **Step 8: Commit**

```bash
git add static/docs.html
git commit -m "feat(docs): hand-written API reference with Try It at /docs"
```

---

## Task 15: Update Dockerfile, README, NAV link, and run final test sweep

**Files:**
- Modify: `Dockerfile`
- Modify: `README.md`
- Modify: `static/index.html` (add DOCS link in NAV)

- [ ] **Step 1: Update `Dockerfile`**

Open `Dockerfile`. Verify the build stage compiles `go build` (which now embeds `static/`). The runtime stage previously copied `static/` separately — remove that step.

After editing, the Dockerfile should look like (representative, adjust to match your existing base images):

```dockerfile
FROM golang:1.21-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/rmb-server .

FROM alpine:3.19
WORKDIR /app
COPY --from=build /out/rmb-server /app/rmb-server
EXPOSE 8080
ENTRYPOINT ["/app/rmb-server"]
```

Key point: there is **no** `COPY static/ ./static/` line in the runtime stage — the static files live inside the binary via `go:embed`.

- [ ] **Step 2: Add the DOCS link to the main NAV**

In `static/index.html`, locate the `<div class="nav-right">` block. Inside, before the `<div class="theme-toggle"`...`>` element, insert:

```html
<a href="/docs" class="docs-link">DOCS ↗</a>
```

Then append to the `<style>` block:

```css
.docs-link {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: var(--fg-2);
  text-decoration: none;
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid var(--hairline-strong);
  transition: all 0.2s;
}
.docs-link:hover {
  background: var(--fg);
  color: var(--bg);
  border-color: var(--fg);
}
```

- [ ] **Step 3: Update `README.md`**

Replace the existing endpoint section with a new one listing all 4. Find the existing `## API文档` section and replace it through to the next top-level heading with:

```markdown
## API 文档

完整接口说明、代码示例与在线试调请见 `http://localhost:8080/docs`。
OpenAPI 规范：`GET /openapi.json` · Swagger UI：`/docs/spec`。

### 端点速览

| 方法 | 路径 | 用途 |
|------|------|------|
| POST | `/api/convert` | 数字 → 中文大写 |
| POST | `/api/convert/reverse` | 中文大写 → 数字 |
| POST | `/api/convert/verify` | 双向一致性校验 |
| POST | `/api/convert/batch` | 批量转换（最多 200 条） |

### 错误码

| HTTP | code | 说明 |
|------|------|------|
| 400  | `invalid_format` | 金额格式不符 |
| 400  | `out_of_range`   | 超出 999,999,999,999.99 上限 |
| 400  | `unparsable_chinese` | 中文大写无法解析（含 `at` 偏移） |
| 413  | `batch_too_large` | 批量超过 200 条 |

### 功能特点

- 4 种模式 Tab：转写 / 还原 / 校验 / 批量
- 深色 / 浅色主题切换（跟随系统，可手动覆盖）
- 反向解析宽容输入：圆 / 元、整 / 正 任一写法均可
- 负数支持（合同冲销 / 退款场景）
- 客户端预校验批量上限（粘贴超过 200 条立即提示）
- API 文档页内嵌 Try It 调试器，不持久化调试历史
```

- [ ] **Step 4: Run the full test sweep**

Run:
```bash
go test ./... -v
```
Expected: every test PASS (converter package: forward, reverse, verify, batch + symmetry; main package: 6 integration tests).

- [ ] **Step 5: End-to-end manual smoke check**

Run: `go run .`
Walk through:
1. `http://localhost:8080` — switch through all 4 Tabs, run a sample in each
2. Switch theme dark↔light; verify Tab UI follows
3. Click "DOCS ↗" → docs page loads in same theme; click "Try it" on convert endpoint; verify it works
4. `/docs/spec` → Swagger UI loads, "Try it out" on `/api/convert` works
5. `/openapi.json` returns full spec

- [ ] **Step 6: Docker build smoke check**

Run:
```bash
docker build -t rmb-test:v1 .
docker run --rm -p 8081:8080 rmb-test:v1 &
sleep 2
curl -s -X POST http://localhost:8081/api/convert -d '{"amount":"1234.56"}' -H 'Content-Type: application/json'
docker stop $(docker ps -q --filter ancestor=rmb-test:v1)
```
Expected: returns `{"chinese":"壹仟贰佰叁拾肆圆伍角陆分"}`. Confirms `go:embed` carried static into binary.

- [ ] **Step 7: Commit**

```bash
git add Dockerfile README.md static/index.html
git commit -m "chore: drop COPY static in Dockerfile (embedded), add /docs nav link, update README"
```

---

## Done

After Task 15, you should have:
- 4 working API endpoints with full test coverage
- 4-tab UI with dark/light theme
- Hand-written `/docs` page with code samples + Try It
- Embedded Swagger UI at `/docs/spec`
- Single-binary Docker deploy via `go:embed`

Final verification: run `go test ./...` (all green) and `git log --oneline -15` (15 atomic commits, one per task).
