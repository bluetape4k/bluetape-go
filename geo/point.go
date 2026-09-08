package geo

import "math"

// Point 값은 degree 단위의 유효한 WGS 84 latitude/longitude 좌표다.
type Point struct {
	latitude  float64
	longitude float64
}

// NewPoint 함수는 latitude, longitude 순서로 Point를 생성한다.
func NewPoint(latitude, longitude float64) (Point, error) {
	point := Point{latitude: latitude, longitude: longitude}
	if err := point.Validate(); err != nil {
		return Point{}, err
	}
	return point, nil
}

// Latitude 메서드는 degree 단위 latitude를 반환한다.
func (p Point) Latitude() float64 { return p.latitude }

// Longitude 메서드는 degree 단위 longitude를 반환한다.
func (p Point) Longitude() float64 { return p.longitude }

// Validate 메서드는 Point가 유한하고 WGS 84 범위 안에 있는지 확인한다.
func (p Point) Validate() error {
	if !finiteInRange(p.latitude, -90, 90) {
		return fieldError(ErrInvalidPoint, "latitude")
	}
	if !finiteInRange(p.longitude, -180, 180) {
		return fieldError(ErrInvalidPoint, "longitude")
	}
	return nil
}

func finiteInRange(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}
