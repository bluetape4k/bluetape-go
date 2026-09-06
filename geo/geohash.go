package geo

import "strings"

const (
	geohashAlphabet  = "0123456789bcdefghjkmnpqrstuvwxyz"
	minimumPrecision = 1
	maximumPrecision = 12
)

var geohashBits = [...]byte{16, 8, 4, 2, 1}

// Cell은 decode된 Geohash의 중심점과 inclusive 경계를 보존한다.
type Cell struct {
	center    Point
	bounds    Bounds
	precision int
}

// Center는 cell 중심점을 반환한다. zero Cell에서는 유효한 zero Point를 반환한다.
func (c Cell) Center() Point { return c.center }

// Bounds는 cell의 inclusive 경계를 반환한다. zero Cell에서는 유효한 zero Bounds를 반환한다.
func (c Cell) Bounds() Bounds { return c.bounds }

// Validate는 Cell이 성공한 decode 결과인지 확인한다.
func (c Cell) Validate() error {
	if c.precision < minimumPrecision || c.precision > maximumPrecision {
		return fieldError(ErrInvalidCell, "precision")
	}
	if c.center.Validate() != nil {
		return fieldError(ErrInvalidCell, "center")
	}
	if c.bounds.Validate() != nil || !c.bounds.containsValidPoint(c.center) {
		return fieldError(ErrInvalidCell, "bounds")
	}
	return nil
}

// Encode는 point를 지정 precision의 canonical lowercase Geohash로 변환한다.
func Encode(point Point, precision int) (string, error) {
	if err := point.Validate(); err != nil {
		return "", err
	}
	if precision < minimumPrecision || precision > maximumPrecision {
		return "", fieldError(ErrInvalidPrecision, "precision")
	}

	longitudeMinimum, longitudeMaximum := -180.0, 180.0
	latitudeMinimum, latitudeMaximum := -90.0, 90.0
	encoded := make([]byte, 0, precision)
	evenBit, bitIndex, character := true, 0, byte(0)
	for len(encoded) < precision {
		if evenBit {
			midpoint := (longitudeMinimum + longitudeMaximum) / 2
			if point.longitude >= midpoint {
				character |= geohashBits[bitIndex]
				longitudeMinimum = midpoint
			} else {
				longitudeMaximum = midpoint
			}
		} else {
			midpoint := (latitudeMinimum + latitudeMaximum) / 2
			if point.latitude >= midpoint {
				character |= geohashBits[bitIndex]
				latitudeMinimum = midpoint
			} else {
				latitudeMaximum = midpoint
			}
		}
		evenBit = !evenBit
		bitIndex++
		if bitIndex == len(geohashBits) {
			encoded = append(encoded, geohashAlphabet[character])
			bitIndex, character = 0, 0
		}
	}
	return string(encoded), nil
}

// Decode는 canonical lowercase Geohash를 중심점과 inclusive 경계를 가진 Cell로 변환한다.
func Decode(hash string) (Cell, error) {
	if len(hash) < minimumPrecision || len(hash) > maximumPrecision {
		return Cell{}, fieldError(ErrInvalidGeohash, "length")
	}
	longitudeMinimum, longitudeMaximum := -180.0, 180.0
	latitudeMinimum, latitudeMaximum := -90.0, 90.0
	evenBit := true
	for index := 0; index < len(hash); index++ {
		value := strings.IndexByte(geohashAlphabet, hash[index])
		if value < 0 {
			return Cell{}, fieldError(ErrInvalidGeohash, "character")
		}
		for _, mask := range geohashBits {
			upper := byte(value)&mask != 0
			if evenBit {
				longitudeMinimum, longitudeMaximum = selectHalf(longitudeMinimum, longitudeMaximum, upper)
			} else {
				latitudeMinimum, latitudeMaximum = selectHalf(latitudeMinimum, latitudeMaximum, upper)
			}
			evenBit = !evenBit
		}
	}

	center := Point{
		latitude:  (latitudeMinimum + latitudeMaximum) / 2,
		longitude: (longitudeMinimum + longitudeMaximum) / 2,
	}
	bounds := Bounds{
		west: longitudeMinimum, south: latitudeMinimum,
		east: longitudeMaximum, north: latitudeMaximum,
	}
	cell := Cell{center: center, bounds: bounds, precision: len(hash)}
	if err := cell.Validate(); err != nil {
		return Cell{}, err
	}
	return cell, nil
}

func selectHalf(minimum, maximum float64, upper bool) (float64, float64) {
	midpoint := (minimum + maximum) / 2
	if upper {
		return midpoint, maximum
	}
	return minimum, midpoint
}
