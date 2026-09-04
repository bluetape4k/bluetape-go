package redismap

import (
	"errors"
	"fmt"
	"reflect"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

var (
	// ErrInvalidClient nil 또는 typed-nil Redis client가 전달됐을 때 반환된다.
	ErrInvalidClient = errors.New("redis mapcache: invalid client")
	// ErrInvalidOptions namespace, hash tag, serializer 또는 option이 유효하지 않을 때 반환된다.
	ErrInvalidOptions = errors.New("redis mapcache: invalid options")
	// ErrInvalidContext operation에 nil context가 전달됐을 때 반환된다.
	ErrInvalidContext = errors.New("redis mapcache: invalid context")
	// ErrSerialization serializer marshal이 실패했을 때 반환된다.
	ErrSerialization = errors.New("redis mapcache: serialization failed")
	// ErrInvalidPayload serializer unmarshal이 실패했을 때 반환된다.
	ErrInvalidPayload = errors.New("redis mapcache: invalid payload")
	// ErrMalformedResult Redis command 또는 Lua result 형식이 예상과 다를 때 반환된다.
	ErrMalformedResult = errors.New("redis mapcache: malformed result")
	// ErrUninitialized zero-value MapCache를 사용했을 때 반환된다.
	ErrUninitialized = errors.New("redis mapcache: uninitialized")
	// ErrCommitUnknown 공용 Redis mutation ambiguity sentinel의 별칭이다.
	ErrCommitUnknown = btredis.ErrCommitUnknown
)

// Error 원인을 보존하면서 MapCache operation 정보와 key를 정제하는 오류다.
type Error struct {
	operation string
	keyID     string
	cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return "redis mapcache operation failed"
	}
	return fmt.Sprintf("redis mapcache %s failed for %s: %s", e.operation, e.keyID, causeType(e.cause))
}

// Unwrap typed Redis operation error 또는 package sentinel chain을 노출한다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is 보존된 원인 chain을 통한 errors.Is 비교를 지원한다.
func (e *Error) Is(target error) bool {
	return e != nil && errors.Is(e.cause, target)
}

// Operation low-cardinality operation label을 반환한다.
func (e *Error) Operation() string {
	if e == nil || e.operation == "" {
		return "operation"
	}
	return e.operation
}

// KeyID redacted Redis key identifier를 반환한다.
func (e *Error) KeyID() string {
	if e == nil || e.keyID == "" {
		return "redis-key:<missing>"
	}
	return e.keyID
}

func causeType(err error) string {
	if err == nil {
		return "<nil>"
	}
	return reflect.TypeOf(err).String()
}

func newError(operation, keyID string, cause error) error {
	return &Error{operation: operation, keyID: keyID, cause: cause}
}

func providerError(operation, keyID string, cause error, unknown bool) error {
	if unknown {
		cause = errors.Join(cause, btredis.ErrCommitUnknown)
	}
	wrapped := btredis.NewOpErrorWithRedactedKey(
		btredis.OpLabels{Family: "redis-mapcache", Operation: operation},
		keyID,
		cause,
	)
	return newError(operation, keyID, wrapped)
}

func codecError(operation, keyID string, sentinel, cause error) error {
	return newError(operation, keyID, errors.Join(sentinel, cause))
}

func malformedMutation(operation, keyID string) error {
	return providerError(operation, keyID, ErrMalformedResult, true)
}
