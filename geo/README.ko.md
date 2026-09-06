# geo

[English](README.md) | [한국어](README.ko.md)

`geo`는 외부 dependency 없이 WGS 84 degree 값, inclusive bounds, Haversine
거리와 canonical lowercase Geohash encode/decode를 제공합니다.

## 가져오기

```go
import (
    "errors"

    "github.com/bluetape4k/bluetape-go/geo"
)
```

## 사용법

```go
func cellFromGeoJSON(position [2]float64) (geo.Cell, error) {
    geoJSONLongitude, geoJSONLatitude := position[0], position[1]
    point, err := geo.NewPoint(geoJSONLatitude, geoJSONLongitude)
    if err != nil {
        return geo.Cell{}, err
    }
    hash, err := geo.Encode(point, 11)
    if err != nil {
        return geo.Cell{}, err
    }
    return geo.Decode(hash)
}
```

`NewPoint`는 degree 단위 `(latitude, longitude)` 순서입니다. GeoJSON position은
`(longitude, latitude)` 순서이므로 변환 코드에서는 변수 이름을 명시하십시오.
Radian 입력, 암묵적 clamp, wrap, normalize는 지원하지 않습니다.

## 경계와 거리

`NewBounds(west, south, east, north)`는 inclusive bounds를 만듭니다.
`east < west`는 antimeridian을 가로지르는 경계를 뜻합니다. 포함 판정에서는
longitude `-180`과 `180`을 같은 meridian으로 취급하지만 constructor 입력값은
그대로 보존합니다.

`DistanceMeters`는 Haversine 공식과 평균 지구 반지름 `6,371,008.8m`를 쓰는
구면 근사입니다. 측량, 과금 또는 법적 경계 판단 계산이 아닙니다.

## Geohash

`Encode`는 precision 1..12와 표준 alphabet
`0123456789bcdefghjkmnpqrstuvwxyz`를 사용합니다. `Decode`는 canonical lowercase
입력만 받고 중심점과 inclusive bounds를 반환합니다. `Cell` zero value는 accessor가
안전하지만 유효하지 않으므로 항상 decode error를 먼저 확인하십시오.

## 오류

`ErrInvalidPoint`, `ErrInvalidBounds`, `ErrInvalidCell`, `ErrInvalidPrecision`,
`ErrInvalidGeohash`는 안정적인 sentinel입니다. 감싼 오류는 `errors.Is`로 판별하고,
error가 nil일 때만 반환값을 사용합니다.

```go
func decodeUserHash(hash string) (geo.Cell, error) {
    cell, err := geo.Decode(hash)
    if errors.Is(err, geo.ErrInvalidGeohash) {
        return geo.Cell{}, err
    }
    return cell, err
}
```

## 비목표

Datum 변환, projection, polygon, routing, tile, geocoding, spatial SQL,
Geohash neighbor/radius cover/index는 제공하지 않습니다.
