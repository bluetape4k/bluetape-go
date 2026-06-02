package serialization_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/serialization"
)

type account struct {
	ID     string `json:"id"`
	Active bool   `json:"active"`
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
