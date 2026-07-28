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
	// ErrInvalidTTL은 Redis TTL이 invalid일 때 반환된다.
	ErrInvalidTTL = errors.New("redis: invalid ttl")
	// ErrCommitUnknown은 dispatch된 Redis mutation이 commit됐을 수 있을 때 반환된다.
	ErrCommitUnknown = errors.New("redis: commit unknown")
)

// OpLabels struct 공개 타입이며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type OpLabels struct {
	Family    string
	Operation string
}

// OpError struct 공개 타입이며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type OpError struct {
	family    string
	operation string
	keyID     string
	err       error
}

// NewOpError NewOpError 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 매개변수:
//   - labels: NewOpError 동작에 필요한 labels 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - rawKey: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//   - err: NewOpError 동작에 필요한 err 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func NewOpError(labels OpLabels, rawKey string, err error) error {
	if err := labels.validate(); err != nil {
		return err
	}
	return newOpErrorWithKeyID(labels, RedactedKeyID(rawKey), err)
}

// NewOpErrorWithRedactedKey NewOpErrorWithRedactedKey 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 매개변수:
//   - labels: NewOpErrorWithRedactedKey 동작에 필요한 labels 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - redactedKeyID: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//   - err: NewOpErrorWithRedactedKey 동작에 필요한 err 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func NewOpErrorWithRedactedKey(labels OpLabels, redactedKeyID string, err error) error {
	if err := labels.validate(); err != nil {
		return err
	}
	if err := ValidateRedactedKeyID(redactedKeyID); err != nil {
		return err
	}
	return newOpErrorWithKeyID(labels, redactedKeyID, err)
}

// Error Error 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
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

// Unwrap Unwrap 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Is Is 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 매개변수:
//   - target: Is 동작에 필요한 target 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (e *OpError) Is(target error) bool {
	if e == nil {
		return false
	}
	return errors.Is(e.err, target)
}

// Family Family 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (e *OpError) Family() string {
	if e == nil || !validLabel(e.family) {
		return "redis"
	}
	return e.family
}

// Operation Operation 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (e *OpError) Operation() string {
	if e == nil || !validLabel(e.operation) {
		return "operation"
	}
	return e.operation
}

// KeyID KeyID 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
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
