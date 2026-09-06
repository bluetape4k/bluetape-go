package mysqlgis

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const pointWKBLength = 21

// Point는 MySQL geometry(Point)에 저장할 WGS84 경도(X)·위도(Y)와 SRID를 보관한다.
//
// MySQL의 ST_AsBinary 결과에는 SRID가 없을 수 있으므로 Scan은 SRID 0을
// 사용한다. 조회 시 ST_SRID를 함께 읽었다면 ScanWithSRID로 명시적으로 복원한다.
type Point struct {
	X     float64
	Y     float64
	SRID  int
	Valid bool
}

// ScannedPoint는 ST_AsBinary와 ST_SRID를 함께 읽은 database/sql 결과다.
type ScannedPoint struct {
	WKB  []byte
	SRID int
}

var _ driver.Valuer = Point{}
var _ interface{ Scan(any) error } = (*Point)(nil)

// NewPoint는 경도(X), 위도(Y), SRID를 검증한 유효한 Point를 생성한다.
func NewPoint(x, y float64, srid int) (Point, error) {
	if err := validateSRID(srid); err != nil {
		return Point{}, err
	}
	if err := validateCoordinates(x, y); err != nil {
		return Point{}, err
	}
	return Point{X: x, Y: y, SRID: srid, Valid: true}, nil
}

// NewWGS84Point는 위도와 경도를 받아 SRID 4326 Point를 생성한다.
func NewWGS84Point(latitude, longitude float64) (Point, error) {
	return NewPoint(longitude, latitude, 4326)
}

// Value는 유효한 Point를 MySQL WKB로 반환하며 SRID는 query helper가 전달한다.
func (p Point) Value() (driver.Value, error) {
	if !p.Valid {
		return nil, nil
	}
	raw, err := p.MarshalWKB()
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// MarshalWKB는 SRID 없는 little-endian WKB byte slice를 새로 반환한다.
func (p Point) MarshalWKB() ([]byte, error) {
	if !p.Valid {
		return nil, nil
	}
	if err := validateSRID(p.SRID); err != nil {
		return nil, err
	}
	if err := validateCoordinates(p.X, p.Y); err != nil {
		return nil, err
	}
	raw := make([]byte, pointWKBLength)
	raw[0] = 1
	binary.LittleEndian.PutUint32(raw[1:5], 1)
	binary.LittleEndian.PutUint64(raw[5:13], math.Float64bits(p.X))
	binary.LittleEndian.PutUint64(raw[13:21], math.Float64bits(p.Y))
	return raw, nil
}

// Scan은 NULL, SRID 없는 WKB, MySQL internal SRID-prefix WKB 또는 POINT WKT를 decode한다.
func (p *Point) Scan(src any) error {
	if p == nil {
		return fmt.Errorf("%w: nil receiver", ErrInvalidPoint)
	}
	*p = Point{}
	if src == nil {
		return nil
	}
	var point Point
	var err error
	switch value := src.(type) {
	case []byte:
		copied := bytes.Clone(value)
		point, err = parseBinary(copied, 0)
		if err != nil && len(copied) > pointWKBLength {
			point, err = parseInternal(copied)
		}
	case string:
		point, err = parseWKT(value)
	case ScannedPoint:
		point, err = parseBinary(bytes.Clone(value.WKB), value.SRID)
	case *ScannedPoint:
		if value == nil {
			return fmt.Errorf("%w: nil scanned point", ErrInvalidPoint)
		}
		point, err = parseBinary(bytes.Clone(value.WKB), value.SRID)
	default:
		return fmt.Errorf("%w: unsupported source", ErrInvalidPoint)
	}
	if err != nil {
		return err
	}
	*p = point
	return nil
}

// ScanWithSRID는 ST_AsBinary 결과와 별도로 조회한 SRID를 함께 적용한다.
func (p *Point) ScanWithSRID(raw []byte, srid int) error {
	if p == nil {
		return fmt.Errorf("%w: nil receiver", ErrInvalidPoint)
	}
	*p = Point{}
	point, err := parseBinary(bytes.Clone(raw), srid)
	if err != nil {
		return err
	}
	*p = point
	return nil
}

// ParseWKB는 SRID 없는 WKB 또는 MySQL internal SRID-prefix WKB를 Point로 변환한다.
func ParseWKB(raw []byte) (Point, error) {
	point, err := parseBinary(bytes.Clone(raw), 0)
	if err == nil {
		return point, nil
	}
	return parseInternal(bytes.Clone(raw))
}

func parseBinary(raw []byte, srid int) (Point, error) {
	if len(raw) != pointWKBLength {
		return Point{}, fmt.Errorf("%w: unexpected WKB length", ErrInvalidPoint)
	}
	if raw[0] != 0 && raw[0] != 1 {
		return Point{}, fmt.Errorf("%w: invalid byte order", ErrInvalidPoint)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if raw[0] == 0 {
		order = binary.BigEndian
	}
	if order.Uint32(raw[1:5]) != 1 {
		return Point{}, fmt.Errorf("%w: non-point geometry", ErrInvalidPoint)
	}
	x := math.Float64frombits(order.Uint64(raw[5:13]))
	y := math.Float64frombits(order.Uint64(raw[13:21]))
	if err := validateSRID(srid); err != nil {
		return Point{}, err
	}
	if err := validateCoordinates(x, y); err != nil {
		return Point{}, err
	}
	return Point{X: x, Y: y, SRID: srid, Valid: true}, nil
}

func parseInternal(raw []byte) (Point, error) {
	if len(raw) != pointWKBLength+4 {
		return Point{}, fmt.Errorf("%w: unexpected internal geometry length", ErrInvalidPoint)
	}
	srid := int(binary.LittleEndian.Uint32(raw[:4]))
	return parseBinary(raw[4:], srid)
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
	return NewPoint(x, y, srid)
}

func validateSRID(srid int) error {
	if srid < 0 || uint64(srid) > math.MaxUint32 {
		return ErrInvalidSRID
	}
	return nil
}

func validateCoordinates(x, y float64) error {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || x < -180 || x > 180 || y < -90 || y > 90 {
		return ErrInvalidPoint
	}
	return nil
}
