package btredis

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var redactedKeyPattern = regexp.MustCompile(`^redis-key:[0-9a-f]{24}$`)

// Key Redis key, TTL, lease, owner token, Lua script primitive에서 사용하는 구조체다.
type Key struct {
	Value      string
	RedactedID string
}

// String Redis key, TTL, lease, owner token, Lua script primitive의 식별 정보를 반환한다.
func (k Key) String() string {
	return k.RedactedID
}

// GoString Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
func (k Key) GoString() string {
	return k.RedactedID
}

// KeyBuilder Redis key, TTL, lease, owner token, Lua script primitive에서 사용하는 구조체다.
type KeyBuilder struct {
	prefix     []string
	structural []string
	hashTag    string
}

// NewKeyBuilder Redis key, TTL, lease, owner token, Lua script primitive에 사용할 값을 생성한다.
//
// 매개변수:
//   - prefix: NewKeyBuilder에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
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

// Structural Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
//
// 매개변수:
//   - parts: Structural에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
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

// WithHashTag Redis key, TTL, lease, owner token, Lua script primitive 옵션을 설정한다.
//
// 매개변수:
//   - tag: WithHashTag에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
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

// StructuralKey Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
//
// 매개변수:
//   - parts: StructuralKey에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
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

// LogicalKey Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
//
// 매개변수:
//   - logicalKey: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
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

// RedactedKeyID Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
//
// 매개변수:
//   - key: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
func RedactedKeyID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "redis-key:" + hex.EncodeToString(sum[:12])
}

// ValidateRedactedKeyID Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
//
// 매개변수:
//   - id: ValidateRedactedKeyID에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
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
