package btredis

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
)

func TestKeyBuilderBuildsStructuralAndLogicalKeys(t *testing.T) {
	builder, err := NewKeyBuilder("bluetape:probabilistic:bloom:v1")
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}

	logical := "tenant:user:{caller}:a:b"
	key, err := builder.Structural("locks")
	if err != nil {
		t.Fatalf("Structural() error = %v", err)
	}
	logicalKey, err := key.LogicalKey(logical)
	if err != nil {
		t.Fatalf("LogicalKey() error = %v", err)
	}
	if want := "bluetape:probabilistic:bloom:v1:locks:" + logical; logicalKey.Value != want {
		t.Fatalf("LogicalKey() = %q, want %q", logicalKey.Value, want)
	}

	spaced, err := builder.LogicalKey(" lock key ")
	if err != nil {
		t.Fatalf("LogicalKey(spaced) error = %v", err)
	}
	if want := "bluetape:probabilistic:bloom:v1: lock key "; spaced.Value != want {
		t.Fatalf("LogicalKey(spaced) = %q, want exact byte preservation", spaced.Value)
	}

	newline, err := builder.LogicalKey("line\nkey")
	if err != nil {
		t.Fatalf("LogicalKey(newline) error = %v", err)
	}
	if want := "bluetape:probabilistic:bloom:v1:line\nkey"; newline.Value != want {
		t.Fatalf("LogicalKey(newline) = %q, want exact byte preservation", newline.Value)
	}
}

func TestKeyBuilderHashTagPreservesColonNamespace(t *testing.T) {
	builder, err := NewKeyBuilder("bluetape:probabilistic:bloom:v1")
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}
	builder, err = builder.WithHashTag("test:package:case")
	if err != nil {
		t.Fatalf("WithHashTag() error = %v", err)
	}

	bits, err := builder.StructuralKey("bits")
	if err != nil {
		t.Fatalf("StructuralKey(bits) error = %v", err)
	}
	config, err := builder.StructuralKey("config")
	if err != nil {
		t.Fatalf("StructuralKey(config) error = %v", err)
	}

	if want := "bluetape:probabilistic:bloom:v1:{test:package:case}:bits"; bits.Value != want {
		t.Fatalf("bits key = %q, want %q", bits.Value, want)
	}
	if want := "bluetape:probabilistic:bloom:v1:{test:package:case}:config"; config.Value != want {
		t.Fatalf("config key = %q, want %q", config.Value, want)
	}
}

func TestKeyBuilderRejectsInvalidStructuralSegments(t *testing.T) {
	builder, err := NewKeyBuilder("bluetape:test:v1")
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}

	for _, value := range []string{"", " ", "a:b", "{bad}", " bad"} {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			if _, err := builder.Structural(value); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Structural(%q) error = %v, want ErrInvalidKey", value, err)
			}
			if _, err := builder.StructuralKey(value); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("StructuralKey(%q) error = %v, want ErrInvalidKey", value, err)
			}
		})
	}
}

func TestKeyBuilderRejectsInvalidPrefixesAndHashTags(t *testing.T) {
	for _, prefix := range []string{"", " ", "bad::prefix", "bad:{tag}", "bad: part"} {
		t.Run("prefix", func(t *testing.T) {
			if _, err := NewKeyBuilder(prefix); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("NewKeyBuilder(%q) error = %v, want ErrInvalidKey", prefix, err)
			}
		})
	}

	builder, err := NewKeyBuilder("bluetape:test:v1")
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}
	for _, tag := range []string{"", " ", "{bad}", "bad{tag}"} {
		t.Run("hash-tag", func(t *testing.T) {
			if _, err := builder.WithHashTag(tag); !errors.Is(err, ErrInvalidHashTag) {
				t.Fatalf("WithHashTag(%q) error = %v, want ErrInvalidHashTag", tag, err)
			}
		})
	}
}

func TestZeroValueKeyBuilderDoesNotPanic(t *testing.T) {
	var builder KeyBuilder
	if _, err := builder.Structural("part"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("zero Structural() error = %v, want ErrInvalidKey", err)
	}
	if _, err := builder.WithHashTag("tag"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("zero WithHashTag() error = %v, want ErrInvalidKey", err)
	}
	if _, err := builder.StructuralKey("part"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("zero StructuralKey() error = %v, want ErrInvalidKey", err)
	}
	if _, err := builder.LogicalKey("logical"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("zero LogicalKey() error = %v, want ErrInvalidKey", err)
	}
}

func TestRedactedKeyIDAndKeyFormatting(t *testing.T) {
	key := "tenant:secret"
	id := RedactedKeyID(key)
	if !regexp.MustCompile(`^redis-key:[0-9a-f]{24}$`).MatchString(id) {
		t.Fatalf("RedactedKeyID() = %q, want redis-key plus 24 hex chars", id)
	}
	if id != RedactedKeyID(key) {
		t.Fatal("RedactedKeyID() is not deterministic")
	}
	if contains(id, key) {
		t.Fatal("RedactedKeyID() leaked raw key")
	}

	built := Key{Value: key, RedactedID: id}
	if built.String() != id || built.GoString() != id {
		t.Fatalf("Key formatting = %q/%q, want redacted id", built.String(), built.GoString())
	}
	if contains(fmt.Sprintf("%#v", built), key) {
		t.Fatal("debug formatting leaked raw key")
	}
}
