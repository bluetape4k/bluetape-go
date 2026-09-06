// Package geo는 WGS 84 degree 좌표를 검증하고 경계 포함, Haversine 거리,
// canonical lowercase Geohash encode/decode를 제공한다.
//
// NewPoint는 latitude, longitude 순서다. GeoJSON의 좌표 순서인 longitude,
// latitude와 다르며 radian 입력은 지원하지 않는다. DistanceMeters는 평균 지구
// 반지름을 쓰는 구면 근사이므로 측량, 과금 또는 법적 경계 판단 용도가 아니다.
package geo
