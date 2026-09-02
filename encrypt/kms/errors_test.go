package kms

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorRedactsExternallyConstructedKindAndOperation(t *testing.T) {
	err := &Error{
		Kind:      fmt.Errorf("secret SDK detail: %w", ErrKMSOperation),
		Operation: "secret key id and payload",
		Cause:     errors.New("secret cause"),
	}
	message := err.Error()
	if strings.Contains(message, "secret") || strings.Contains(message, "SDK") {
		t.Fatalf("Error() leaked caller-provided detail: %q", message)
	}
	if !strings.Contains(message, ErrKMSOperation.Error()) || !strings.Contains(message, "operation") {
		t.Fatalf("Error() = %q, want safe sentinel and operation label", message)
	}
	if !errors.Is(err, ErrKMSOperation) {
		t.Fatalf("errors.Is(Error, ErrKMSOperation) = false")
	}
}
