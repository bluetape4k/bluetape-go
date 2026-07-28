package btredis

import (
	"fmt"
	"log/slog"
	"strings"
)

// Lease struct 공개 타입이며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Lease struct {
	key   string
	token OwnerToken
}

// NewLease NewLease 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 매개변수:
//   - key: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//   - token: 소유자 식별 또는 compare-and-delete 안전성을 위한 token이다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func NewLease(key string, token OwnerToken) (Lease, error) {
	lease := Lease{key: key, token: token}
	if err := lease.Validate(); err != nil {
		return Lease{}, err
	}
	return lease, nil
}

// Key Key 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (l Lease) Key() string {
	return l.key
}

// RedactedKeyID RedactedKeyID 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (l Lease) RedactedKeyID() string {
	return RedactedKeyID(l.key)
}

// Token Token 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (l Lease) Token() OwnerToken {
	return l.token
}

// String String 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (l Lease) String() string {
	return "redis-lease:" + l.RedactedKeyID()
}

// GoString GoString 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (l Lease) GoString() string {
	return l.String()
}

// LogValue LogValue 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
func (l Lease) LogValue() slog.Value {
	return slog.StringValue(l.String())
}

// Validate Validate 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func (l Lease) Validate() error {
	if strings.TrimSpace(l.key) == "" {
		return fmt.Errorf("%w: invalid lease key", ErrInvalidKey)
	}
	if err := l.token.Validate(); err != nil {
		return err
	}
	return nil
}
