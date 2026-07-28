package redis

import jwt "github.com/bluetape4k/bluetape-go/jwt"

// Options Redis-backed JWT KeyChain repository 설정이다.
type Options = jwt.RedisRepositoryOptions

// Repository Redis-backed JWT KeyChain repository 구현이다.
type Repository = jwt.RedisRepository

// New New 공개 API의 동작을 수행하며 JWT key repository의 Redis key, TTL, serialization 계약을 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func New(options Options) (*Repository, error) {
	return jwt.NewRedisRepository(options)
}
