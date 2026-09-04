package sqsextended

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEnvelopeRoundTripPreservesMetadata(t *testing.T) {
	original := Envelope{
		Version:     EnvelopeVersion,
		Bucket:      "payloads",
		Key:         "orders/42/payload.json",
		ContentSize: 11,
		Checksum:    strings.Repeat("a", 64),
		ContentType: "application/json",
		EncryptionMetadata: map[string]string{
			"key_id":    "alias/orders",
			"algorithm": "aws:kms",
		},
	}

	encoded, err := EncodeEnvelope(original)
	if err != nil {
		t.Fatalf("EncodeEnvelope: %v", err)
	}
	want := `{"version":1,"bucket":"payloads","key":"orders/42/payload.json","content_size":11,"checksum":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","content_type":"application/json","encryption_metadata":{"algorithm":"aws:kms","key_id":"alias/orders"}}`
	if string(encoded) != want {
		t.Fatalf("encoded = %s, want canonical %s", encoded, want)
	}

	decoded, err := DecodeEnvelope(encoded)
	if err != nil {
		t.Fatalf("DecodeEnvelope: %v", err)
	}
	if decoded.Version != original.Version || decoded.Bucket != original.Bucket || decoded.Key != original.Key || decoded.ContentSize != original.ContentSize || decoded.Checksum != original.Checksum || decoded.ContentType != original.ContentType {
		t.Fatalf("decoded = %#v, want %#v", decoded, original)
	}
	if len(decoded.EncryptionMetadata) != 2 || decoded.EncryptionMetadata["algorithm"] != "aws:kms" || decoded.EncryptionMetadata["key_id"] != "alias/orders" {
		t.Fatalf("decoded encryption metadata = %#v", decoded.EncryptionMetadata)
	}
	original.EncryptionMetadata["algorithm"] = "mutated"
	if decoded.EncryptionMetadata["algorithm"] != "aws:kms" {
		t.Fatal("DecodeEnvelope shared caller metadata map")
	}
}

func TestEncodeEnvelopeValidation(t *testing.T) {
	valid := Envelope{
		Version:     EnvelopeVersion,
		Bucket:      "bucket",
		Key:         "key",
		ContentSize: 1,
		Checksum:    strings.Repeat("0", 64),
	}
	tests := []struct {
		name   string
		mutate func(*Envelope)
	}{
		{name: "unsupported version", mutate: func(value *Envelope) { value.Version = EnvelopeVersion + 1 }},
		{name: "blank bucket", mutate: func(value *Envelope) { value.Bucket = " " }},
		{name: "blank key", mutate: func(value *Envelope) { value.Key = "\t" }},
		{name: "negative size", mutate: func(value *Envelope) { value.ContentSize = -1 }},
		{name: "invalid checksum", mutate: func(value *Envelope) { value.Checksum = "not-a-checksum" }},
		{name: "uppercase checksum", mutate: func(value *Envelope) { value.Checksum = strings.Repeat("A", 64) }},
		{name: "invalid bucket utf8", mutate: func(value *Envelope) { value.Bucket = string([]byte{0xff}) }},
		{name: "invalid metadata key", mutate: func(value *Envelope) { value.EncryptionMetadata = map[string]string{"": "value"} }},
		{name: "invalid metadata value utf8", mutate: func(value *Envelope) { value.EncryptionMetadata = map[string]string{"key": string([]byte{0xff})} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := valid
			tt.mutate(&candidate)
			want := ErrInvalidEnvelope
			switch tt.name {
			case "unsupported version":
				want = ErrUnsupportedVersion
			case "negative size":
				want = ErrPayloadTooLarge
			}
			if _, err := EncodeEnvelope(candidate); !errors.Is(err, want) {
				t.Fatalf("EncodeEnvelope error = %v, want %v", err, want)
			}
		})
	}
}

func TestDecodeEnvelopeRejectsAmbiguousWireValues(t *testing.T) {
	valid := `{"version":1,"bucket":"bucket","key":"key","content_size":1,"checksum":"0000000000000000000000000000000000000000000000000000000000000000"}`
	tests := []struct {
		name string
		data string
		want error
	}{
		{name: "leading whitespace", data: " " + valid, want: ErrInvalidEnvelope},
		{name: "trailing bytes", data: valid + "x", want: ErrInvalidEnvelope},
		{name: "unknown field", data: strings.TrimSuffix(valid, "}") + `,"unknown":true}`, want: ErrInvalidEnvelope},
		{name: "duplicate field", data: strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1), want: ErrInvalidEnvelope},
		{name: "unsupported version", data: strings.Replace(valid, `"version":1`, `"version":2`, 1), want: ErrUnsupportedVersion},
		{name: "null", data: "null", want: ErrInvalidEnvelope},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeEnvelope([]byte(tt.data)); !errors.Is(err, tt.want) {
				t.Fatalf("DecodeEnvelope error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDecodeEnvelopeRejectsNonCanonicalOrderAndCopiesInput(t *testing.T) {
	data := []byte(`{"bucket":"bucket","version":1,"key":"key","content_size":1,"checksum":"0000000000000000000000000000000000000000000000000000000000000000"}`)
	if _, err := DecodeEnvelope(data); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("DecodeEnvelope error = %v, want ErrInvalidEnvelope", err)
	}

	canonical, err := EncodeEnvelope(Envelope{
		Version:     EnvelopeVersion,
		Bucket:      "bucket",
		Key:         "key",
		ContentSize: 1,
		Checksum:    strings.Repeat("0", 64),
	})
	if err != nil {
		t.Fatalf("EncodeEnvelope: %v", err)
	}
	got, err := DecodeEnvelope(canonical)
	if err != nil {
		t.Fatalf("DecodeEnvelope: %v", err)
	}
	canonical[0] = 'x'
	if got.Bucket != "bucket" {
		t.Fatalf("decoded envelope changed after input mutation: %#v", got)
	}
}

func TestEnvelopeJSONUsesNoUnexpectedFields(t *testing.T) {
	encoded, err := json.Marshal(Envelope{
		Version:     EnvelopeVersion,
		Bucket:      "bucket",
		Key:         "key",
		ContentSize: 0,
		Checksum:    strings.Repeat("0", 64),
	})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !bytes.Contains(encoded, []byte(`"content_size":0`)) {
		t.Fatalf("encoded zero content size = %s", encoded)
	}
}
