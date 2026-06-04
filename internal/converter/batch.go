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
