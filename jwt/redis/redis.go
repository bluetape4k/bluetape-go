package redis

import jwt "github.com/bluetape4k/bluetape-go/jwt"

// Options Redis-backed JWT KeyChain repository 설정이다.
type Options = jwt.RedisRepositoryOptions

// Repository Redis-backed JWT KeyChain repository 구현이다.
type Repository = jwt.RedisRepository

// New New 공개 API의 동작을 수행하며 JWT key repository의 Redis key, TTL, serialization 계약을 보존한다.
//
// 매개변수:
//   - options: New 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
func New(options Options) (*Repository, error) {
	return jwt.NewRedisRepository(options)
}
