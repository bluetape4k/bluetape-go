package geo

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidPoint 오류는 유한하지 않거나 WGS 84 범위를 벗어난 좌표를 나타낸다.
	ErrInvalidPoint = errors.New("geo: invalid point")
	// ErrInvalidBounds 오류는 유한하지 않거나 범위/남북 순서가 잘못된 경계를 나타낸다.
	ErrInvalidBounds = errors.New("geo: invalid bounds")
	// ErrInvalidCell 오류는 유효한 decode 결과가 아닌 Cell을 나타낸다.
	ErrInvalidCell = errors.New("geo: invalid cell")
	// ErrInvalidPrecision 오류는 지원 범위 밖의 Geohash precision을 나타낸다.
	ErrInvalidPrecision = errors.New("geo: invalid precision")
	// ErrInvalidGeohash 오류는 canonical lowercase Geohash가 아닌 입력을 나타낸다.
	ErrInvalidGeohash = errors.New("geo: invalid geohash")
)

func fieldError(kind error, field string) error {
	return fmt.Errorf("%w: %s", kind, field)
}
