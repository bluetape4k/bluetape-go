# geo

[English](README.md) | [한국어](README.ko.md)

`geo` provides dependency-free WGS 84 degree values, inclusive bounds,
Haversine distance, and canonical lowercase Geohash encode/decode.

## Import

```go
import (
    "errors"

    "github.com/bluetape4k/bluetape-go/geo"
)
```

## Usage

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

`NewPoint` accepts `(latitude, longitude)` in degrees. GeoJSON positions use
`(longitude, latitude)`; name the variables when converting between them.
Radian input, implicit clamp, wrapping, and normalization are not supported.

## Bounds and distance

`NewBounds(west, south, east, north)` creates inclusive bounds. `east < west`
means that the bounds cross the antimeridian. Longitudes `-180` and `180` are
equivalent for containment, while the constructor preserves the input value.

`DistanceMeters` uses the Haversine formula and the mean Earth radius
`6,371,008.8m`. It is a spherical approximation, not a surveying, billing, or
legal-boundary calculation.

## Geohash

`Encode` accepts precision 1 through 12 and returns the standard alphabet
`0123456789bcdefghjkmnpqrstuvwxyz`. `Decode` accepts only canonical lowercase
input and returns a center and inclusive bounds. Check the decode error before
using `Cell`; its zero value is safe to access but invalid.

## Errors

`ErrInvalidPoint`, `ErrInvalidBounds`, `ErrInvalidCell`, `ErrInvalidPrecision`,
and `ErrInvalidGeohash` are stable sentinels. Handle wrapped failures with
`errors.Is`; inspect a returned value only when the error is nil.

```go
func decodeUserHash(hash string) (geo.Cell, error) {
    cell, err := geo.Decode(hash)
    if errors.Is(err, geo.ErrInvalidGeohash) {
        return geo.Cell{}, err
    }
    return cell, err
}
```

## Non-goals

This package does not provide datum conversion, projection, polygons, routing,
tiles, geocoding, spatial SQL, Geohash neighbors, radius covers, or indexes.
Use the separate [`geocoding`](../geocoding/README.md) and
[`sqlkit/postgis`](../sqlkit/postgis/README.md),
[`sqlkit/mysqlgis`](../sqlkit/mysqlgis/README.md), or
[`sqlkit/mariadbgis`](../sqlkit/mariadbgis/README.md) packages when those
boundaries are required.
