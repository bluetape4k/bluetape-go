package postgis

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	ewkbPointType    = uint32(1)
	ewkbSRIDFlag     = uint32(0x20000000)
	ewkbBaseTypeMask = uint32(0x0fffffff)
)

// Point 는 WGS84 경도(X)·위도(Y)와 SRID를 보관하는 유효한 공간 값이다.
//
// X는 -180..180, Y는 -90..90의 degree 값이다. zero value는 SQL NULL을
// 나타내며, Valid가 true일 때만 Value가 non-NULL EWKB를 반환한다.
type Point struct {
	X     float64
	Y     float64
	SRID  int
	Valid bool
}

var _ driver.Valuer = Point{}
var _ interface{ Scan(any) error } = (*Point)(nil)

// NewPoint 는 경도(X), 위도(Y), SRID를 검증한 유효한 Point를 생성한다.
func NewPoint(x, y float64, srid int) (Point, error) {
	if err := validateSRID(srid); err != nil {
		return Point{}, err
	}
	if err := validateCoordinates(x, y); err != nil {
		return Point{}, err
	}
	return Point{X: x, Y: y, SRID: srid, Valid: true}, nil
}

// NewWGS84Point 는 위도와 경도를 받아 SRID 4326 Point를 생성한다.
func NewWGS84Point(latitude, longitude float64) (Point, error) {
	return NewPoint(longitude, latitude, 4326)
}

// Value 는 유효한 Point를 SRID가 포함된 little-endian EWKB로 반환한다.
func (p Point) Value() (driver.Value, error) {
	if !p.Valid {
		return nil, nil
	}
	raw, err := p.MarshalEWKB()
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// MarshalEWKB 는 Point를 호출자가 소유하는 새 EWKB byte slice로 인코딩한다.
func (p Point) MarshalEWKB() ([]byte, error) {
	if !p.Valid {
		return nil, nil
	}
	if err := validateSRID(p.SRID); err != nil {
		return nil, err
	}
	if err := validateCoordinates(p.X, p.Y); err != nil {
		return nil, err
	}

	raw := make([]byte, 25)
	raw[0] = 1 // WKB little endian marker.
	binary.LittleEndian.PutUint32(raw[1:5], ewkbPointType|ewkbSRIDFlag)
	binary.LittleEndian.PutUint32(raw[5:9], uint32(p.SRID))
	binary.LittleEndian.PutUint64(raw[9:17], math.Float64bits(p.X))
	binary.LittleEndian.PutUint64(raw[17:25], math.Float64bits(p.Y))
	return raw, nil
}

// Scan 은 nil, EWKB/WKB byte slice 또는 POINT WKT를 Point로 decode한다.
//
// Scan은 decode 성공 후에만 값을 공개하며, 실패하면 receiver를 zero value로
// 되돌린다. driver가 소유한 byte slice는 내부에 보관하지 않는다.
func (p *Point) Scan(src any) error {
	if p == nil {
		return fmt.Errorf("%w: nil receiver", ErrInvalidPoint)
	}
	*p = Point{}
	if src == nil {
		return nil
	}

	var point Point
	switch value := src.(type) {
	case []byte:
		copied := bytes.Clone(value)
		var err error
		if len(copied) > 0 && (copied[0] == 0 || copied[0] == 1) {
			point, err = ParseEWKB(copied)
		} else {
			point, err = parseWKT(string(copied))
		}
		if err != nil {
			return err
		}
	case string:
		var err error
		point, err = parseWKT(value)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: unsupported source", ErrInvalidPoint)
	}
	*p = point
	return nil
}

// ParseEWKB 는 EWKB 또는 SRID 없는 WKB Point를 검증해 반환한다.
func ParseEWKB(raw []byte) (Point, error) {
	if len(raw) < 21 {
		return Point{}, fmt.Errorf("%w: truncated value", ErrInvalidPoint)
	}
	var order binary.ByteOrder
	switch raw[0] {
	case 0:
		order = binary.BigEndian
	case 1:
		order = binary.LittleEndian
	default:
		return Point{}, fmt.Errorf("%w: invalid byte order", ErrInvalidPoint)
	}

	typeWord := order.Uint32(raw[1:5])
	if typeWord&0x80000000 != 0 || typeWord&0x40000000 != 0 || typeWord&0x10000000 != 0 {
		return Point{}, fmt.Errorf("%w: dimensional point is unsupported", ErrInvalidPoint)
	}
	hasSRID := typeWord&ewkbSRIDFlag != 0
	baseType := typeWord & ewkbBaseTypeMask
	if baseType != ewkbPointType {
		return Point{}, fmt.Errorf("%w: non-point geometry", ErrInvalidPoint)
	}
	offset := 5
	srid := 0
	if hasSRID {
		if len(raw) < 25 {
			return Point{}, fmt.Errorf("%w: truncated SRID point", ErrInvalidPoint)
		}
		srid = int(order.Uint32(raw[5:9]))
		offset = 9
	}
	if len(raw) != offset+16 {
		return Point{}, fmt.Errorf("%w: unexpected point length", ErrInvalidPoint)
	}
	x := math.Float64frombits(order.Uint64(raw[offset : offset+8]))
	y := math.Float64frombits(order.Uint64(raw[offset+8 : offset+16]))
	if err := validateSRID(srid); err != nil {
		return Point{}, err
	}
	if err := validateCoordinates(x, y); err != nil {
		return Point{}, err
	}
	return Point{X: x, Y: y, SRID: srid, Valid: true}, nil
}

func parseWKT(value string) (Point, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return Point{}, fmt.Errorf("%w: empty WKT", ErrInvalidPoint)
	}
	srid := 0
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "SRID=") {
		separator := strings.IndexByte(trimmed, ';')
		if separator < 0 {
			return Point{}, fmt.Errorf("%w: malformed SRID", ErrInvalidPoint)
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(trimmed[len("SRID="):separator]))
		if err != nil {
			return Point{}, fmt.Errorf("%w: malformed SRID", ErrInvalidPoint)
		}
		srid = parsed
		trimmed = strings.TrimSpace(trimmed[separator+1:])
		upper = strings.ToUpper(trimmed)
	}
	if !strings.HasPrefix(upper, "POINT") {
		return Point{}, fmt.Errorf("%w: unsupported WKT", ErrInvalidPoint)
	}
	body := strings.TrimSpace(trimmed[len("POINT"):])
	if len(body) < 5 || body[0] != '(' || body[len(body)-1] != ')' {
		return Point{}, fmt.Errorf("%w: malformed WKT", ErrInvalidPoint)
	}
	parts := strings.Fields(strings.TrimSpace(body[1 : len(body)-1]))
	if len(parts) != 2 {
		return Point{}, fmt.Errorf("%w: point needs two coordinates", ErrInvalidPoint)
	}
	x, errX := strconv.ParseFloat(parts[0], 64)
	y, errY := strconv.ParseFloat(parts[1], 64)
	if errX != nil || errY != nil {
		return Point{}, fmt.Errorf("%w: malformed coordinate", ErrInvalidPoint)
	}
	if err := validateSRID(srid); err != nil {
		return Point{}, err
	}
	if err := validateCoordinates(x, y); err != nil {
		return Point{}, err
	}
	return Point{X: x, Y: y, SRID: srid, Valid: true}, nil
}

func validateSRID(srid int) error {
	if srid < 0 || uint64(srid) > math.MaxUint32 {
		return fmt.Errorf("%w", ErrInvalidSRID)
	}
	return nil
}

func validateCoordinates(x, y float64) error {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return fmt.Errorf("%w: coordinates must be finite", ErrInvalidPoint)
	}
	if x < -180 || x > 180 || y < -90 || y > 90 {
		return fmt.Errorf("%w: coordinates out of range", ErrInvalidPoint)
	}
	return nil
}
