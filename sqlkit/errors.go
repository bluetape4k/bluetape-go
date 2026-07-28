package sqlkit

import "errors"

var (
	// ErrInvalidArgument 호출자가 전달한 SQL helper 입력이 유효하지 않을 때 반환된다.
	ErrInvalidArgument = errors.New("sqlkit: invalid argument")

	// ErrNoRows 단일 row query가 row를 반환하지 않았을 때 반환된다.
	ErrNoRows = errors.New("sqlkit: no rows")

	// ErrTooManyRows 단일 row query가 둘 이상의 row를 반환했을 때 반환된다.
	ErrTooManyRows = errors.New("sqlkit: too many rows")
)
