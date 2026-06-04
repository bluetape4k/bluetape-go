package redisnear

import (
	"strings"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	payload, err := encodeMessage(invalidationMessage{
		Namespace: "catalog",
		OriginID:  "origin-1",
		Operation: operationSet,
		Key:       "item:1",
	})
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	message, err := decodeMessage(string(payload))
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}
	if message.Version != messageVersion {
		t.Fatalf("version should be set, got %d", message.Version)
	}
	if message.Namespace != "catalog" || message.OriginID != "origin-1" ||
		message.Operation != operationSet || message.Key != "item:1" {
		t.Fatalf("decoded message mismatch: %+v", message)
	}
}

func TestDecodeMessageRejectsMalformedPayload(t *testing.T) {
	if _, err := decodeMessage("{"); err == nil {
		t.Fatal("malformed payload should fail")
	}

	if _, err := decodeMessage(`{"version":2,"namespace":"n","originID":"o","operation":"set"}`); err == nil {
		t.Fatal("unsupported version should fail")
	}

	if _, err := decodeMessage(`{"version":1,"namespace":"n","originID":"o","operation":"copy"}`); err == nil {
		t.Fatal("unsupported operation should fail")
	}

	if _, err := decodeMessage(`{"version":1,"originID":"o","operation":"set"}`); err == nil ||
		!strings.Contains(err.Error(), "namespace") {
		t.Fatalf("missing namespace should fail with namespace error, got %v", err)
	}
}
