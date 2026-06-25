package mongo

import jwt "github.com/bluetape4k/bluetape-go/jwt"

// Options configures a MongoDB-backed JWT KeyChain repository.
type Options = jwt.MongoRepositoryOptions

// Repository is a MongoDB-backed JWT KeyChain repository.
type Repository = jwt.MongoRepository

// New creates a MongoDB-backed JWT KeyChain repository.
func New(options Options) (*Repository, error) {
	return jwt.NewMongoRepository(options)
}
