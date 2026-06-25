package core_test

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestStringDefaults(t *testing.T) {
	if !core.HasText(" blue ") {
		t.Fatal("HasText should accept visible text")
	}
	if core.HasText(" \t ") {
		t.Fatal("HasText should reject whitespace-only text")
	}
	if got := core.EmptyToDefault("", "fallback"); got != "fallback" {
		t.Fatalf("EmptyToDefault returned %q", got)
	}
	if got := core.EmptyToDefault(" ", "fallback"); got != " " {
		t.Fatalf("EmptyToDefault should keep whitespace, got %q", got)
	}
	if got := core.BlankToDefault(" ", "fallback"); got != "fallback" {
		t.Fatalf("BlankToDefault returned %q", got)
	}
}

func TestTruncateUTF8Bytes(t *testing.T) {
	got, err := core.TruncateUTF8Bytes("Hello, 세계", 9)
	if err != nil {
		t.Fatalf("TruncateUTF8Bytes returned error: %v", err)
	}
	if got != "Hello, " {
		t.Fatalf("TruncateUTF8Bytes returned %q", got)
	}

	got, err = core.TruncateUTF8Bytes("abc", 10)
	if err != nil {
		t.Fatalf("TruncateUTF8Bytes returned error: %v", err)
	}
	if got != "abc" {
		t.Fatalf("TruncateUTF8Bytes should keep short input, got %q", got)
	}

	if _, err := core.TruncateUTF8Bytes("abc", -1); err == nil {
		t.Fatal("TruncateUTF8Bytes should reject negative maxBytes")
	}
}

func TestTruncateUTF8BytesRejectsInvalidUTF8(t *testing.T) {
	invalidShort := string([]byte{0xff})
	if _, err := core.TruncateUTF8Bytes(invalidShort, len(invalidShort)); !errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("TruncateUTF8Bytes invalid short error = %v, want ErrInvalidUTF8", err)
	}

	invalidAroundBoundary := "ok" + string([]byte{0xff}) + "tail"
	if _, err := core.TruncateUTF8Bytes(invalidAroundBoundary, 3); !errors.Is(err, core.ErrInvalidUTF8) {
		t.Fatalf("TruncateUTF8Bytes invalid boundary error = %v, want ErrInvalidUTF8", err)
	}
}

func TestTruncateUTF8BytesBoundaries(t *testing.T) {
	got, err := core.TruncateUTF8Bytes("세계", 0)
	if err != nil {
		t.Fatalf("TruncateUTF8Bytes zero limit returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("TruncateUTF8Bytes zero limit = %q, want empty", got)
	}

	got, err = core.TruncateUTF8Bytes("세계", len("세"))
	if err != nil {
		t.Fatalf("TruncateUTF8Bytes exact rune boundary returned error: %v", err)
	}
	if got != "세" {
		t.Fatalf("TruncateUTF8Bytes exact rune boundary = %q, want %q", got, "세")
	}
}
