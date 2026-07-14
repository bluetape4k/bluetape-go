package sqlcheckpoint

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestOpErrorSupportsNestedInspection(t *testing.T) {
	cause := errors.New("hostile-cause-marker")
	err := newOperationError("load", []byte("namespace"), []byte("key"), cause)
	nested := fmt.Errorf("nested: %w", err)

	var opErr *OpError
	if !errors.As(nested, &opErr) {
		t.Fatalf("errors.As(%T) = false", nested)
	}
	if !errors.Is(nested, cause) {
		t.Fatal("nested OpError did not preserve its cause")
	}
	if got := opErr.Family(); got != "sql checkpoint" {
		t.Fatalf("Family() = %q, want %q", got, "sql checkpoint")
	}
	if got := opErr.Operation(); got != "load" {
		t.Fatalf("Operation() = %q, want %q", got, "load")
	}
	if got := opErr.KeyID(); !strings.HasPrefix(got, "sql-checkpoint-key:") {
		t.Fatalf("KeyID() = %q", got)
	}
}

func TestCodecErrorSupportsNestedInspection(t *testing.T) {
	cause := errors.New("hostile-codec-cause-marker")
	err := newCodecError("decode", cause)
	nested := fmt.Errorf("nested: %w", err)

	var codecErr *CodecError
	if !errors.As(nested, &codecErr) {
		t.Fatalf("errors.As(%T) = false", nested)
	}
	if !errors.Is(nested, cause) {
		t.Fatal("nested CodecError did not preserve its cause")
	}
	if got := codecErr.Family(); got != "checkpoint codec" {
		t.Fatalf("Family() = %q, want %q", got, "checkpoint codec")
	}
	if got := codecErr.Operation(); got != "decode" {
		t.Fatalf("Operation() = %q, want %q", got, "decode")
	}
}

func TestOpErrorAndCodecErrorZeroValuesAreSafe(t *testing.T) {
	var nilOp *OpError
	for _, opErr := range []*OpError{nilOp, {}} {
		if opErr.Error() == "" || opErr.Family() == "" || opErr.Operation() == "" || opErr.KeyID() == "" {
			t.Fatalf("unsafe zero OpError: error=%q family=%q operation=%q keyID=%q", opErr.Error(), opErr.Family(), opErr.Operation(), opErr.KeyID())
		}
		if opErr.Unwrap() != nil {
			t.Fatal("zero OpError unwrap was non-nil")
		}
	}

	var nilCodec *CodecError
	for _, codecErr := range []*CodecError{nilCodec, {}} {
		if codecErr.Error() == "" || codecErr.Family() == "" || codecErr.Operation() == "" {
			t.Fatalf("unsafe zero CodecError: error=%q family=%q operation=%q", codecErr.Error(), codecErr.Family(), codecErr.Operation())
		}
		if codecErr.Unwrap() != nil {
			t.Fatal("zero CodecError unwrap was non-nil")
		}
	}
}

func TestErrorsRedactHostileValuesFromAllDefaultFormats(t *testing.T) {
	markers := []string{
		"hostile-namespace-marker",
		"hostile-key-marker",
		"hostile-payload-marker",
		"postgres://user:hostile-password-marker@hostile-endpoint-marker/database",
		"hostile-panic-marker",
		"hostile-cause-marker",
		"select hostile-sql-marker",
	}
	cause := errors.New(strings.Join(markers[2:], " "))
	errs := []error{
		newOperationError("load", []byte(markers[0]), []byte(markers[1]), cause),
		newCodecError("decode", cause),
	}

	for _, err := range errs {
		for _, format := range []string{"%s", "%v", "%+v"} {
			rendered := fmt.Sprintf(format, err)
			for _, marker := range markers {
				if strings.Contains(rendered, marker) {
					t.Fatalf("%T format %q leaked %q: %s", err, format, marker, rendered)
				}
			}
		}
	}
}

func TestKeyIDIsDeterministicAndBoundarySafe(t *testing.T) {
	const want = "sql-checkpoint-key:2b63864eb225cda63378"
	if got := redactedKeyID([]byte("ns"), []byte("checkpoint-key")); got != want {
		t.Fatalf("redactedKeyID() = %q, want %q", got, want)
	}
	if got := redactedKeyID([]byte("ns"), []byte("checkpoint-key")); got != want {
		t.Fatalf("second redactedKeyID() = %q, want %q", got, want)
	}
	if left, right := redactedKeyID([]byte("a"), []byte("bc")), redactedKeyID([]byte("ab"), []byte("c")); left == right {
		t.Fatalf("length-prefixed namespace boundary collided: %q", left)
	}
}
