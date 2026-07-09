package btredis

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var redactedKeyPattern = regexp.MustCompile(`^redis-key:[0-9a-f]{24}$`)

// Key is a Redis key value plus its stable redacted diagnostic identifier.
type Key struct {
	Value      string
	RedactedID string
}

// String returns the redacted key identifier.
func (k Key) String() string {
	return k.RedactedID
}

// GoString returns the redacted key identifier for debug formatting.
func (k Key) GoString() string {
	return k.RedactedID
}

// KeyBuilder builds Redis keys from package-owned structural parts and one
// caller-owned logical key segment.
type KeyBuilder struct {
	prefix     []string
	structural []string
	hashTag    string
}

// NewKeyBuilder returns a builder for a colon-delimited package key prefix.
func NewKeyBuilder(prefix string) (KeyBuilder, error) {
	parts := strings.Split(prefix, ":")
	if len(parts) == 0 {
		return KeyBuilder{}, invalidKey("prefix")
	}
	for _, part := range parts {
		if err := validateStructuralSegment(part); err != nil {
			return KeyBuilder{}, invalidKey("prefix")
		}
	}
	return KeyBuilder{prefix: append([]string(nil), parts...)}, nil
}

// Structural appends package-owned structural key parts.
func (b KeyBuilder) Structural(parts ...string) (KeyBuilder, error) {
	if err := b.validate(); err != nil {
		return KeyBuilder{}, err
	}
	if err := validateStructuralSegments(parts); err != nil {
		return KeyBuilder{}, err
	}
	next := b.clone()
	next.structural = append(next.structural, parts...)
	return next, nil
}

// WithHashTag adds a Redis Cluster hash tag. Colons are preserved.
func (b KeyBuilder) WithHashTag(tag string) (KeyBuilder, error) {
	if err := b.validate(); err != nil {
		return KeyBuilder{}, err
	}
	if strings.TrimSpace(tag) == "" || strings.ContainsAny(tag, "{}") {
		return KeyBuilder{}, fmt.Errorf("%w: invalid hash tag", ErrInvalidHashTag)
	}
	next := b.clone()
	next.hashTag = tag
	return next, nil
}

// StructuralKey returns a key made only from package-owned structural parts.
func (b KeyBuilder) StructuralKey(parts ...string) (Key, error) {
	if err := b.validate(); err != nil {
		return Key{}, err
	}
	if err := validateStructuralSegments(parts); err != nil {
		return Key{}, err
	}
	value := b.join(parts...)
	return Key{Value: value, RedactedID: RedactedKeyID(value)}, nil
}

// LogicalKey returns a key with one caller-owned logical key segment preserved verbatim.
func (b KeyBuilder) LogicalKey(logicalKey string) (Key, error) {
	if err := b.validate(); err != nil {
		return Key{}, err
	}
	if strings.TrimSpace(logicalKey) == "" {
		return Key{}, invalidKey("logical key")
	}
	value := b.join(logicalKey)
	return Key{Value: value, RedactedID: RedactedKeyID(value)}, nil
}

// RedactedKeyID returns a stable, deterministic key correlation identifier.
func RedactedKeyID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "redis-key:" + hex.EncodeToString(sum[:12])
}

// ValidateRedactedKeyID verifies that id has the canonical redacted key shape.
func ValidateRedactedKeyID(id string) error {
	if !redactedKeyPattern.MatchString(id) {
		return invalidKey("redacted key id")
	}
	return nil
}

func (b KeyBuilder) validate() error {
	if len(b.prefix) == 0 {
		return invalidKey("builder")
	}
	return nil
}

func (b KeyBuilder) clone() KeyBuilder {
	next := b
	next.prefix = append([]string(nil), b.prefix...)
	next.structural = append([]string(nil), b.structural...)
	return next
}

func (b KeyBuilder) join(suffix ...string) string {
	parts := make([]string, 0, len(b.prefix)+len(b.structural)+1+len(suffix))
	parts = append(parts, b.prefix...)
	parts = append(parts, b.structural...)
	if b.hashTag != "" {
		parts = append(parts, "{"+b.hashTag+"}")
	}
	parts = append(parts, suffix...)
	return strings.Join(parts, ":")
}

func validateStructuralSegments(parts []string) error {
	for _, part := range parts {
		if err := validateStructuralSegment(part); err != nil {
			return err
		}
	}
	return nil
}

func validateStructuralSegment(part string) error {
	if strings.TrimSpace(part) == "" || strings.TrimSpace(part) != part || strings.ContainsAny(part, "{}:") {
		return invalidKey("structural segment")
	}
	return nil
}

func invalidKey(name string) error {
	return fmt.Errorf("%w: invalid %s", ErrInvalidKey, name)
}
