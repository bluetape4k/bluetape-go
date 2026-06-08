package id

import (
	"errors"
	"testing"
)

func TestErrorsSupportIsAndAs(t *testing.T) {
	cause := errors.New("cause")

	optionErr := OptionError{Option: "machineID", Err: cause}
	if !errors.Is(optionErr, ErrInvalidOptions) || !errors.Is(optionErr, cause) {
		t.Fatalf("OptionError should match sentinel and cause: %v", optionErr)
	}
	var asOption OptionError
	if !errors.As(optionErr, &asOption) {
		t.Fatalf("OptionError should support errors.As")
	}

	parseErr := ParseError{Kind: "uuid", Value: "bad", Err: cause}
	if !errors.Is(parseErr, ErrInvalidID) || !errors.Is(parseErr, cause) {
		t.Fatalf("ParseError should match sentinel and cause: %v", parseErr)
	}

	entropyErr := EntropyError{Kind: "uuid", Err: cause}
	if !errors.Is(entropyErr, ErrEntropy) || !errors.Is(entropyErr, cause) {
		t.Fatalf("EntropyError should match sentinel and cause: %v", entropyErr)
	}

	rollbackErr := ClockRollbackError{Last: 10, Now: 9}
	if !errors.Is(rollbackErr, ErrClockRollback) {
		t.Fatalf("ClockRollbackError should match sentinel: %v", rollbackErr)
	}

	exhaustedErr := SequenceExhaustedError{Millis: 10}
	if !errors.Is(exhaustedErr, ErrSequenceExhausted) {
		t.Fatalf("SequenceExhaustedError should match sentinel: %v", exhaustedErr)
	}
}
