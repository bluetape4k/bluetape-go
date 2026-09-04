package dynamodbleader

import (
	"errors"

	"github.com/bluetape4k/bluetape-go/leader"
)

var (
	// ErrInvalidClient nil 또는 typed-nil DynamoDB client가 전달됐을 때 반환된다.
	ErrInvalidClient = errors.New("dynamodb leader: invalid client")
	// ErrInvalidOptions table, schema 또는 provider option이 유효하지 않을 때 반환된다.
	ErrInvalidOptions = errors.New("dynamodb leader: invalid options")
	// ErrMalformedItem 필수 item attribute가 없거나 잘못된 형식일 때 반환된다.
	ErrMalformedItem = errors.New("dynamodb leader: malformed item")
)

func providerError(operation string, cause error, unknown bool) error {
	if unknown {
		cause = errors.Join(cause, leader.ErrCommitUnknown)
	}
	return leader.NewOperationError("dynamodb", operation, cause)
}
