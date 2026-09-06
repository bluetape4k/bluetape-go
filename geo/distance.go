package geo

import "math"

const meanEarthRadiusMeters = 6_371_008.8

// DistanceMeters는 평균 지구 반지름 Haversine 구면 근사 거리를 meter로 반환한다.
func DistanceMeters(left, right Point) (float64, error) {
	if err := left.Validate(); err != nil {
		return 0, err
	}
	if err := right.Validate(); err != nil {
		return 0, err
	}
	latitude1 := degreesToRadians(left.latitude)
	latitude2 := degreesToRadians(right.latitude)
	deltaLatitude := latitude2 - latitude1
	deltaLongitude := degreesToRadians(longitudeDelta(left.longitude, right.longitude))

	sinLatitude := math.Sin(deltaLatitude / 2)
	sinLongitude := math.Sin(deltaLongitude / 2)
	a := sinLatitude*sinLatitude + math.Cos(latitude1)*math.Cos(latitude2)*sinLongitude*sinLongitude
	a = math.Max(0, math.Min(1, a))
	return 2 * meanEarthRadiusMeters * math.Asin(math.Sqrt(a)), nil
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

func longitudeDelta(left, right float64) float64 {
	delta := math.Mod(right-left+180, 360)
	if delta < 0 {
		delta += 360
	}
	return delta - 180
}
