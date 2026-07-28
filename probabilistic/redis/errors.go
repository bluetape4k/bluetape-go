package redisbloom

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidOptions는 Redis probabilistic option이 invalid일 때 반환된다.
	ErrInvalidOptions = errors.New("redis probabilistic: invalid options")
	// ErrConfigMismatch는 저장된 metadata가 호출자 config와 호환되지 않을 때 반환된다.
	ErrConfigMismatch = errors.New("redis bloom: config mismatch")
	// ErrConfigCorrupt는 저장된 metadata가 없거나 불완전할 때 반환된다.
	ErrConfigCorrupt = errors.New("redis bloom: config corrupt")
)

// RedisError는 struct 공개 타입이며 Redis Bloom/HyperLogLog key, TTL, script, backend compatibility 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type RedisError struct {
	Family    string
	Operation string
	KeyID     string
	Err       error
}

func (e RedisError) Error() string {
	family := e.Family
	if family == "" {
		family = "redis probabilistic"
	}
	if e.KeyID == "" {
		return fmt.Sprintf("%s %s: %v", family, e.Operation, e.Err)
	}
	return fmt.Sprintf("%s %s %s: %v", family, e.Operation, e.KeyID, e.Err)
}

func (e RedisError) Unwrap() error {
	return e.Err
}

func mapScriptError(operation string, keyID string, err error) error {
	if err == nil {
		return nil
	}
	switch scriptErrorMarker(err) {
	case "config_mismatch":
		return fmt.Errorf("%w: %s", ErrConfigMismatch, keyID)
	case "config_corrupt":
		return fmt.Errorf("%w: %s", ErrConfigCorrupt, keyID)
	default:
		return mapRedisError("redis bloom", operation, keyID, err)
	}
}

func mapRedisError(family string, operation string, keyID string, err error) error {
	if err == nil {
		return nil
	}
	return RedisError{Family: family, Operation: operation, KeyID: keyID, Err: err}
}

func scriptErrorMarker(err error) string {
	message := strings.TrimSpace(err.Error())
	message = strings.TrimPrefix(message, "ERR ")
	switch message {
	case "config_mismatch", "config_corrupt":
		return message
	default:
		return ""
	}
}
