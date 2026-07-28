package btredis

import (
	"fmt"
	"time"
)

// ValidateTTL ValidateTTL 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 매개변수:
//   - name: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//   - ttl: lease 또는 entry 유효 시간이다. zero/negative/expiry 의미는 TTL 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func ValidateTTL(name string, ttl time.Duration) error {
	if ttl < time.Millisecond {
		return fmt.Errorf("%w: invalid %s duration", ErrInvalidTTL, ttlName(name))
	}
	return nil
}

// TTLMillis TTLMillis 공개 API의 동작을 수행하며 Redis key, TTL, lease, owner token, Lua script primitive 계약을 보존한다.
//
// 매개변수:
//   - name: Redis key 또는 key 구성 요소다. namespace, slot, normalization 의미는 primitive 계약을 따른다.
//   - ttl: lease 또는 entry 유효 시간이다. zero/negative/expiry 의미는 TTL 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func TTLMillis(name string, ttl time.Duration) (int64, error) {
	if err := ValidateTTL(name, ttl); err != nil {
		return 0, err
	}
	return ttl.Milliseconds(), nil
}

func ttlName(name string) string {
	if validLabel(name) {
		return name
	}
	return "ttl"
}
