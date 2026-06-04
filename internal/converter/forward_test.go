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
