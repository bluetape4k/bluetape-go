package mariadbgis

import "errors"

var (
	// ErrInvalidPoint는 좌표 또는 WKB Point가 유효하지 않을 때 반환된다.
	ErrInvalidPoint = errors.New("mariadbgis: invalid point")
	// ErrInvalidSRID는 SRID가 음수이거나 uint32 범위를 벗어날 때 반환된다.
	ErrInvalidSRID = errors.New("mariadbgis: invalid SRID")
	// ErrInvalidArgument는 SQL helper 인자가 비어 있거나 안전하지 않을 때 반환된다.
	ErrInvalidArgument = errors.New("mariadbgis: invalid argument")
	// ErrInvalidDistance는 거리 값이 유한하지 않거나 음수일 때 반환된다.
	ErrInvalidDistance = errors.New("mariadbgis: invalid distance")
)
