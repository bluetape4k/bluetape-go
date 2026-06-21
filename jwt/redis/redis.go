package redis

import jwt "github.com/bluetape4k/bluetape-go/jwt"

// Options configures a Redis-backed JWT KeyChain repository.
type Options = jwt.RedisRepositoryOptions

// Repository is a Redis-backed JWT KeyChain repository.
type Repository = jwt.RedisRepository

// New creates a Redis-backed JWT KeyChain repository.
func New(options Options) (*Repository, error) {
	return jwt.NewRedisRepository(options)
}
