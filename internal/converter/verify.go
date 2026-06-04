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
