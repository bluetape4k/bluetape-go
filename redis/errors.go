package btredis

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

var (
	// ErrInvalidKey Redis key 또는 key 관련 label이 invalid일 때 반환된다.
	ErrInvalidKey = errors.New("redis: invalid key")
	// ErrInvalidHashTag Redis Cluster hash tag가 invalid일 때 반환된다.
	ErrInvalidHashTag = errors.New("redis: invalid hash tag")
	// ErrInvalidTTL Redis TTL이 invalid일 때 반환된다.
	ErrInvalidTTL = errors.New("redis: invalid ttl")
	// ErrCommitUnknown dispatch된 Redis mutation이 commit됐을 수 있을 때 반환된다.
	ErrCommitUnknown = errors.New("redis: commit unknown")
)

// OpLabels Redis key, TTL, lease, owner token, Lua script primitive에서 사용하는 구조체다.
type OpLabels struct {
	Family    string
	Operation string
}

// OpError Redis key, TTL, lease, owner token, Lua script primitive에서 사용하는 구조체다.
type OpError struct {
	family    string
	operation string
	keyID     string
	err       error
}

// NewOpError Redis key, TTL, lease, owner token, Lua script primitive에 사용할 값을 생성한다.
//
// 매개변수:
//   - labels: NewOpError에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - rawKey: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//   - err: NewOpError에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func NewOpError(labels OpLabels, rawKey string, err error) error {
	if err := labels.validate(); err != nil {
		return err
	}
	return newOpErrorWithKeyID(labels, RedactedKeyID(rawKey), err)
}

// NewOpErrorWithRedactedKey Redis key, TTL, lease, owner token, Lua script primitive에 사용할 값을 생성한다.
//
// 매개변수:
//   - labels: NewOpErrorWithRedactedKey에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - redactedKeyID: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//   - err: NewOpErrorWithRedactedKey에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func NewOpErrorWithRedactedKey(labels OpLabels, redactedKeyID string, err error) error {
	if err := labels.validate(); err != nil {
		return err
	}
	if err := ValidateRedactedKeyID(redactedKeyID); err != nil {
		return err
	}
	return newOpErrorWithKeyID(labels, redactedKeyID, err)
}

// Error 오류 메시지를 반환한다.
func (e *OpError) Error() string {
	if e == nil {
		return "redis operation failed"
	}
	cause := "<nil>"
	if e.err != nil {
		cause = reflect.TypeOf(e.err).String()
	}
	return fmt.Sprintf("%s %s failed for %s: %s", e.Family(), e.Operation(), e.KeyID(), cause)
}

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: Is에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (e *OpError) Is(target error) bool {
	if e == nil {
		return false
	}
	return errors.Is(e.err, target)
}

// Family Redis key, TTL, lease, owner token, Lua script primitive의 식별 정보를 반환한다.
func (e *OpError) Family() string {
	if e == nil || !validLabel(e.family) {
		return "redis"
	}
	return e.family
}

// Operation Redis key, TTL, lease, owner token, Lua script primitive의 식별 정보를 반환한다.
func (e *OpError) Operation() string {
	if e == nil || !validLabel(e.operation) {
		return "operation"
	}
	return e.operation
}

// KeyID Redis key, TTL, lease, owner token, Lua script primitive의 식별 정보를 반환한다.
func (e *OpError) KeyID() string {
	if e == nil || ValidateRedactedKeyID(e.keyID) != nil {
		return "redis-key:<missing>"
	}
	return e.keyID
}

func newOpErrorWithKeyID(labels OpLabels, keyID string, err error) error {
	return &OpError{
		family:    labels.Family,
		operation: labels.Operation,
		keyID:     keyID,
		err:       err,
	}
}

func (l OpLabels) validate() error {
	if !validLabel(l.Family) || !validLabel(l.Operation) {
		return invalidKey("operation labels")
	}
	return nil
}

func validLabel(label string) bool {
	if label != strings.TrimSpace(label) || label == "" || len(label) > 64 || strings.ContainsAny(label, "{}:") {
		return false
	}
	for _, r := range label {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
