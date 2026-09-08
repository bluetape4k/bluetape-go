package postgis

import "errors"

var (
	// ErrInvalidPoint 좌표, EWKB 또는 WKT Point 값이 유효하지 않을 때 반환된다.
	ErrInvalidPoint = errors.New("postgis: invalid point")

	// ErrInvalidSRID SRID가 음수이거나 지원 범위를 벗어날 때 반환된다.
	ErrInvalidSRID = errors.New("postgis: invalid SRID")

	// ErrInvalidArgument SQL helper 인자가 비어 있거나 안전하지 않을 때 반환된다.
	ErrInvalidArgument = errors.New("postgis: invalid argument")

	// ErrInvalidDistance 거리 값이 유한하지 않거나 음수일 때 반환된다.
	ErrInvalidDistance = errors.New("postgis: invalid distance")
)
