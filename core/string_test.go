package core_test

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestStringDefaults(t *testing.T) {
	if !core.HasLength(" ") {
		t.Fatal("HasLength should accept whitespace because length is non-zero")
	}
	if core.HasLength("") {
		t.Fatal("HasLength should reject empty text")
	}
	if !core.NoLength("") {
		t.Fatal("NoLength should accept empty text")
	}
	if core.NoLength(" ") {
		t.Fatal("NoLength should reject whitespace because length is non-zero")
	}
	if !core.HasText(" blue ") {
		t.Fatal("HasText should accept visible text")
	}
	if core.HasText(" \t ") {
		t.Fatal("HasText should reject whitespace-only text")
	}
	if !core.NoText(" \t ") {
		t.Fatal("NoText should accept whitespace-only text")
	}
	if core.NoText("blue") {
		t.Fatal("NoText should reject visible text")
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

func TestStringNilConversions(t *testing.T) {
	if got := core.EmptyToNil(""); got != nil {
		t.Fatalf("EmptyToNil(empty) = %v, want nil", *got)
	}
	if got := core.EmptyToNil(" "); got == nil || *got != " " {
		t.Fatalf("EmptyToNil(space) = %v, want pointer to space", got)
	}
	if got := core.BlankToNil(" \t "); got != nil {
		t.Fatalf("BlankToNil(blank) = %v, want nil", *got)
	}
	if got := core.BlankToNil("blue"); got == nil || *got != "blue" {
		t.Fatalf("BlankToNil(text) = %v, want pointer to text", got)
	}
}

func TestStringMaskAndCommonAffixes(t *testing.T) {
	if got := core.Mask("secret", '#'); got != "######" {
		t.Fatalf("Mask returned %q", got)
	}
	if got := core.Mask("비밀", '*'); got != "**" {
		t.Fatalf("Mask should be rune-aware, got %q", got)
	}
	if got := core.Mask("", '*'); got != "" {
		t.Fatalf("Mask(empty) = %q, want empty", got)
	}
	if got := core.CommonPrefix("안녕-blue", "안녕-red"); got != "안녕-" {
		t.Fatalf("CommonPrefix returned %q", got)
	}
	if got := core.CommonPrefix("abc", "xyz"); got != "" {
		t.Fatalf("CommonPrefix mismatch returned %q", got)
	}
	if got := core.CommonSuffix("blue-세계", "red-세계"); got != "-세계" {
		t.Fatalf("CommonSuffix returned %q", got)
	}
	if got := core.CommonSuffix("abc", "xyz"); got != "" {
		t.Fatalf("CommonSuffix mismatch returned %q", got)
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
