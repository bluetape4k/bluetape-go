package mongo

import jwt "github.com/bluetape4k/bluetape-go/jwt"

// Options는 JWT key provider repository에서 설정값과 기본값 적용 방식을 설명한다.
type Options = jwt.MongoRepositoryOptions

// Repository는 JWT key provider repository에서 caller-visible 상태와 의미를 설명한다.
type Repository = jwt.MongoRepository

// New는 JWT key provider repository에서 생성과 초기화 계약을 설명한다.
func New(options Options) (*Repository, error) {
	return jwt.NewMongoRepository(options)
}
