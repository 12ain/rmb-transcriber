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
