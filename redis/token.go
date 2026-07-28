package btredis

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

var tokenPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ErrInvalidOwnerToken 변수 공개 값이며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
// 호출자는 이 식별자를 Redis 오류, stream 상태, lease/token, 옵션, 또는 기본값 계약을 비교할 때 사용한다.
var ErrInvalidOwnerToken = errors.New("redis: invalid owner token")

// OwnerToken struct 공개 타입이며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type OwnerToken struct {
	value string
}

// NewOwnerToken NewOwnerToken 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func NewOwnerToken() (OwnerToken, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return OwnerToken{}, fmt.Errorf("redis owner token: %w", err)
	}
	return OwnerToken{value: hex.EncodeToString(data[:])}, nil
}

// ParseOwnerToken ParseOwnerToken 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 매개변수:
//   - value: ParseOwnerToken에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func ParseOwnerToken(value string) (OwnerToken, error) {
	token := OwnerToken{value: value}
	if err := token.Validate(); err != nil {
		return OwnerToken{}, err
	}
	return token, nil
}

// String String 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (t OwnerToken) String() string {
	if t.value == "" {
		return "redis-owner-token:<empty>"
	}
	return "redis-owner-token:<redacted>"
}

// GoString GoString 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (t OwnerToken) GoString() string {
	return t.String()
}

// LogValue LogValue 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (t OwnerToken) LogValue() slog.Value {
	return slog.StringValue(t.String())
}

// RedisValue RedisValue 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (t OwnerToken) RedisValue() string {
	return t.value
}

// Validate Validate 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func (t OwnerToken) Validate() error {
	if strings.TrimSpace(t.value) == "" || !tokenPattern.MatchString(t.value) {
		return fmt.Errorf("%w: expected 64 lowercase hex characters", ErrInvalidOwnerToken)
	}
	return nil
}
