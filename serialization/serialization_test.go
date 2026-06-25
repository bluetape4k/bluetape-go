package serialization_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
	"github.com/bluetape4k/bluetape-go/serialization"
)

type account struct {
	ID     string `json:"id"`
	Active bool   `json:"active"`
}

type formatSerializer[T any] struct {
	format string
}

func (s formatSerializer[T]) Format() string { return s.format }
func (s formatSerializer[T]) Marshal(_ T) ([]byte, error) {
	return nil, nil
}
func (s formatSerializer[T]) Unmarshal(_ []byte) (T, error) {
	var zero T
	return zero, nil
}

func TestJSONSerializerRoundTrip(t *testing.T) {
	serializer := serialization.NewJSONSerializer[account]()
	expected := account{ID: "acct-1", Active: true}

	data, err := serializer.Marshal(expected)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	actual, err := serializer.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if actual != expected {
		t.Fatalf("got %+v, want %+v", actual, expected)
	}
}

func TestJSONSerializerRejectsCorruptInput(t *testing.T) {
	serializer := serialization.NewJSONSerializer[account]()

	if _, err := serializer.Unmarshal([]byte("{not-json")); err == nil {
		t.Fatal("expected corrupt JSON to fail")
	}
	if _, err := serializer.Unmarshal(nil); err == nil {
		t.Fatal("expected empty input to fail")
	}
}

func TestJSONSerializerRejectsTrailingJSONValue(t *testing.T) {
	serializer := serialization.NewJSONSerializer[account]()

	if _, err := serializer.Unmarshal([]byte(`{"id":"acct-1","active":true}{"id":"acct-2","active":false}`)); err == nil {
		t.Fatal("expected trailing JSON value to fail")
	}
}

func TestJSONSerializerRejectsUnknownFieldsWhenConfigured(t *testing.T) {
	serializer := serialization.NewJSONSerializer[account](serialization.WithDisallowUnknownFields())

	if _, err := serializer.Unmarshal([]byte(`{"id":"acct-1","active":true,"role":"admin"}`)); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}

func TestBytesSerializerCopiesData(t *testing.T) {
	serializer := serialization.BytesSerializer{}
	source := []byte{1, 2, 3}

	data, err := serializer.Marshal(source)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	source[0] = 9
	if data[0] != 1 {
		t.Fatalf("Marshal should return a copy, got %v", data)
	}

	decoded, err := serializer.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	data[1] = 9
	if decoded[1] != 2 {
		t.Fatalf("Unmarshal should return a copy, got %v", decoded)
	}
}

func TestStringSerializerRoundTrip(t *testing.T) {
	serializer := serialization.StringSerializer{}
	expected := "안녕, bluetape-go"

	data, err := serializer.Marshal(expected)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	actual, err := serializer.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if actual != expected {
		t.Fatalf("got %q, want %q", actual, expected)
	}
}

func TestStringSerializerRejectsInvalidUTF8(t *testing.T) {
	serializer := serialization.StringSerializer{}
	invalid := string([]byte{0xff})

	if _, err := serializer.Marshal(invalid); !errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("Marshal invalid UTF-8 error = %v, want ErrInvalidUTF8", err)
	}
	if _, err := serializer.Unmarshal([]byte{0xff}); !errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("Unmarshal invalid UTF-8 error = %v, want ErrInvalidUTF8", err)
	}
}

func TestBytesSerializerAcceptsArbitraryBinary(t *testing.T) {
	serializer := serialization.BytesSerializer{}
	input := []byte{0xff, 0xfe, 0x00}

	data, err := serializer.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal binary failed: %v", err)
	}
	got, err := serializer.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal binary failed: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Fatalf("BytesSerializer binary = %v, want %v", got, input)
	}
}

func TestRawSerializersAcceptEmptyNonNilInput(t *testing.T) {
	emptyBytes := []byte{}
	bytesValue, err := (serialization.BytesSerializer{}).Unmarshal(emptyBytes)
	if err != nil {
		t.Fatalf("BytesSerializer empty input failed: %v", err)
	}
	if bytesValue == nil || len(bytesValue) != 0 {
		t.Fatalf("BytesSerializer empty input = %#v, want empty non-nil slice", bytesValue)
	}

	stringValue, err := (serialization.StringSerializer{}).Unmarshal([]byte{})
	if err != nil {
		t.Fatalf("StringSerializer empty input failed: %v", err)
	}
	if stringValue != "" {
		t.Fatalf("StringSerializer empty input = %q, want empty string", stringValue)
	}

	data, err := (serialization.StringSerializer{}).Marshal("")
	if err != nil {
		t.Fatalf("StringSerializer empty string marshal failed: %v", err)
	}
	if data == nil || len(data) != 0 {
		t.Fatalf("StringSerializer empty string marshal = %#v, want empty non-nil bytes", data)
	}
}

func TestRawSerializersRejectNilUnmarshalInput(t *testing.T) {
	if _, err := (serialization.BytesSerializer{}).Unmarshal(nil); err == nil || errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("BytesSerializer nil input error = %v, want non-UTF8 error", err)
	}
	if _, err := (serialization.StringSerializer{}).Unmarshal(nil); err == nil || errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("StringSerializer nil input error = %v, want non-UTF8 error", err)
	}
}

func TestVersionedSerializerRoundTrip(t *testing.T) {
	jsonSerializer := serialization.NewJSONSerializer[account]()
	serializer, err := serialization.NewVersionedSerializer[account](jsonSerializer, 1)
	if err != nil {
		t.Fatalf("NewVersionedSerializer failed: %v", err)
	}

	data, err := serializer.Marshal(account{ID: "acct-1", Active: true})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("BTGS")) {
		t.Fatalf("missing envelope magic: %q", data[:4])
	}

	actual, err := serializer.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if actual.ID != "acct-1" || !actual.Active {
		t.Fatalf("unexpected value: %+v", actual)
	}
}

func TestVersionedSerializerRejectsCorruptEnvelope(t *testing.T) {
	jsonSerializer := serialization.NewJSONSerializer[account]()
	serializer, err := serialization.NewVersionedSerializer[account](jsonSerializer, 1)
	if err != nil {
		t.Fatalf("NewVersionedSerializer failed: %v", err)
	}

	_, err = serializer.Unmarshal([]byte("bad"))
	if !errors.Is(err, serialization.ErrInvalidEnvelope) {
		t.Fatalf("expected ErrInvalidEnvelope, got %v", err)
	}

	data, err := serializer.Marshal(account{ID: "acct-1"})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	data[4], data[5] = 0, 2
	_, err = serializer.Unmarshal(data)
	if !errors.Is(err, serialization.ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestVersionedSerializerRejectsFormatMismatch(t *testing.T) {
	jsonSerializer := serialization.NewJSONSerializer[account]()
	jsonVersioned, err := serialization.NewVersionedSerializer[account](jsonSerializer, 1)
	if err != nil {
		t.Fatalf("NewVersionedSerializer failed: %v", err)
	}

	stringVersioned, err := serialization.NewVersionedSerializer[string](serialization.StringSerializer{}, 1)
	if err != nil {
		t.Fatalf("NewVersionedSerializer failed: %v", err)
	}
	data, err := stringVersioned.Marshal("hello")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	_, err = jsonVersioned.Unmarshal(data)
	if !errors.Is(err, serialization.ErrFormatMismatch) {
		t.Fatalf("expected ErrFormatMismatch, got %v", err)
	}
}

func TestVersionedSerializerRequiresValidMetadata(t *testing.T) {
	if _, err := serialization.NewVersionedSerializer[account](nil, 1); err == nil {
		t.Fatal("expected nil serializer to fail")
	}
	if _, err := serialization.NewVersionedSerializer[account](serialization.NewJSONSerializer[account](), 0); err == nil {
		t.Fatal("expected zero version to fail")
	}
}

func TestVersionedSerializerRejectsInvalidFormatMetadata(t *testing.T) {
	if _, err := serialization.NewVersionedSerializer[string](formatSerializer[string]{format: ""}, 1); err == nil {
		t.Fatal("expected empty format to fail")
	}

	tooLong := strings.Repeat("x", 256)
	if _, err := serialization.NewVersionedSerializer[string](formatSerializer[string]{format: tooLong}, 1); err == nil {
		t.Fatal("expected overlong format to fail")
	}
}
