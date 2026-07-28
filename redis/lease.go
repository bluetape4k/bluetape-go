package btredis

import (
	"fmt"
	"log/slog"
	"strings"
)

// Lease Redis key, TTL, lease, owner token, Lua script primitive에서 사용하는 구조체다.
type Lease struct {
	key   string
	token OwnerToken
}

// NewLease Redis key, TTL, lease, owner token, Lua script primitive에 사용할 값을 생성한다.
//
// 매개변수:
//   - key: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//   - token: 소유자 식별 또는 compare-and-delete 안전성을 위한 token이다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func NewLease(key string, token OwnerToken) (Lease, error) {
	lease := Lease{key: key, token: token}
	if err := lease.Validate(); err != nil {
		return Lease{}, err
	}
	return lease, nil
}

// Key Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
func (l Lease) Key() string {
	return l.key
}

// RedactedKeyID Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
func (l Lease) RedactedKeyID() string {
	return RedactedKeyID(l.key)
}

// Token Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
func (l Lease) Token() OwnerToken {
	return l.token
}

// String Redis key, TTL, lease, owner token, Lua script primitive의 식별 정보를 반환한다.
func (l Lease) String() string {
	return "redis-lease:" + l.RedactedKeyID()
}

// GoString Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
func (l Lease) GoString() string {
	return l.String()
}

// LogValue Redis key, TTL, lease, owner token, Lua script primitive 동작을 수행한다.
func (l Lease) LogValue() slog.Value {
	return slog.StringValue(l.String())
}

// Validate 값이 Redis key, TTL, lease, owner token, Lua script primitive 규칙을 만족하는지 검사한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func (l Lease) Validate() error {
	if strings.TrimSpace(l.key) == "" {
		return fmt.Errorf("%w: invalid lease key", ErrInvalidKey)
	}
	if err := l.token.Validate(); err != nil {
		return err
	}
	return nil
}
