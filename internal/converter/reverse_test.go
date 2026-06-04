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
