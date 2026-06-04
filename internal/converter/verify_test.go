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
	if r.DiffAt != 10 {
		t.Errorf("got diffAt %d, want 10", r.DiffAt)
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
