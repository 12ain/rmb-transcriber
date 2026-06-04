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
