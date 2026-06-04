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
