package postgis_test

import (
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit/postgis"
)

var _ driver.Valuer = postgis.Point{}
var _ interface{ Scan(any) error } = (*postgis.Point)(nil)

func TestPointValueAndScanRoundTrip(t *testing.T) {
	want, err := postgis.NewPoint(127.0276, 37.4979, 4326)
	if err != nil {
		t.Fatalf("NewPoint failed: %v", err)
	}

	value, err := want.Value()
	if err != nil {
		t.Fatalf("Value failed: %v", err)
	}
	raw, ok := value.([]byte)
	if !ok {
		t.Fatalf("Value type = %T, want []byte", value)
	}
	if len(raw) != 25 {
		t.Fatalf("EWKB length = %d, want 25", len(raw))
	}

	var got postgis.Point
	if err := got.Scan(raw); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestPointDistinguishesNullAndZero(t *testing.T) {
	point, err := postgis.NewPoint(0, 0, 4326)
	if err != nil {
		t.Fatalf("NewPoint failed: %v", err)
	}
	if err := point.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) failed: %v", err)
	}
	if point.Valid || point.SRID != 0 {
		t.Fatalf("null point = %#v, want invalid zero value", point)
	}

	if err := point.Scan(mustEWKB(t, 0, 0, 4326)); err != nil {
		t.Fatalf("Scan(zero) failed: %v", err)
	}
	if !point.Valid || point.X != 0 || point.Y != 0 || point.SRID != 4326 {
		t.Fatalf("zero point = %#v, want valid zero coordinates", point)
	}
}

func TestPointClearsStateOnFailure(t *testing.T) {
	point, err := postgis.NewPoint(127, 37, 4326)
	if err != nil {
		t.Fatalf("NewPoint failed: %v", err)
	}

	tests := []struct {
		name string
		src  any
		want error
	}{
		{name: "truncated", src: []byte{1, 1, 0}, want: postgis.ErrInvalidPoint},
		{name: "wrong type", src: mustEWKBType(t, 2), want: postgis.ErrInvalidPoint},
		{name: "unsupported source", src: int64(1), want: postgis.ErrInvalidPoint},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := point.Scan(tt.src); !errors.Is(err, tt.want) {
				t.Fatalf("Scan error = %v, want %v", err, tt.want)
			}
			if point.Valid || point != (postgis.Point{}) {
				t.Fatalf("failed Scan retained state: %#v", point)
			}
		})
	}
}

func TestPointRejectsInvalidCoordinatesAndSRID(t *testing.T) {
	tests := []struct {
		name string
		x, y float64
		srid int
		want error
	}{
		{name: "nan", x: math.NaN(), y: 0, srid: 4326, want: postgis.ErrInvalidPoint},
		{name: "infinite", x: 0, y: math.Inf(1), srid: 4326, want: postgis.ErrInvalidPoint},
		{name: "longitude", x: 181, y: 0, srid: 4326, want: postgis.ErrInvalidPoint},
		{name: "latitude", x: 0, y: 91, srid: 4326, want: postgis.ErrInvalidPoint},
		{name: "negative SRID", x: 0, y: 0, srid: -1, want: postgis.ErrInvalidSRID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := postgis.NewPoint(tt.x, tt.y, tt.srid)
			if !errors.Is(err, tt.want) {
				t.Fatalf("NewPoint error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPointAcceptsWKTWithSRID(t *testing.T) {
	var point postgis.Point
	if err := point.Scan("SRID=4326;POINT(127.0276 37.4979)"); err != nil {
		t.Fatalf("Scan WKT failed: %v", err)
	}
	if !point.Valid || point.SRID != 4326 || point.X != 127.0276 || point.Y != 37.4979 {
		t.Fatalf("point = %#v, want WKT point", point)
	}
}

func TestPointAcceptsSRIDFreeWKB(t *testing.T) {
	raw := make([]byte, 21)
	raw[0] = 1
	binary.LittleEndian.PutUint32(raw[1:5], 1)
	binary.LittleEndian.PutUint64(raw[5:13], math.Float64bits(127.0))
	binary.LittleEndian.PutUint64(raw[13:21], math.Float64bits(37.0))

	var point postgis.Point
	if err := point.Scan(raw); err != nil {
		t.Fatalf("Scan WKB failed: %v", err)
	}
	if !point.Valid || point.SRID != 0 || point.X != 127 || point.Y != 37 {
		t.Fatalf("point = %#v, want valid SRID-free WKB point", point)
	}
}

func TestPointAcceptsBigEndianWKB(t *testing.T) {
	raw := make([]byte, 21)
	raw[0] = 0
	binary.BigEndian.PutUint32(raw[1:5], 1)
	binary.BigEndian.PutUint64(raw[5:13], math.Float64bits(-122.4))
	binary.BigEndian.PutUint64(raw[13:21], math.Float64bits(37.8))

	var point postgis.Point
	if err := point.Scan(raw); err != nil {
		t.Fatalf("Scan big-endian WKB failed: %v", err)
	}
	if point != (postgis.Point{X: -122.4, Y: 37.8, Valid: true}) {
		t.Fatalf("point = %#v, want big-endian coordinates", point)
	}
}

func TestPointRejectsDimensionalWKB(t *testing.T) {
	raw := make([]byte, 29)
	raw[0] = 1
	binary.LittleEndian.PutUint32(raw[1:5], 0x80000001)
	if err := (&postgis.Point{}).Scan(raw); !errors.Is(err, postgis.ErrInvalidPoint) {
		t.Fatalf("dimensional WKB error = %v, want ErrInvalidPoint", err)
	}
}

func mustEWKB(t *testing.T, x, y float64, srid int) []byte {
	t.Helper()
	point, err := postgis.NewPoint(x, y, srid)
	if err != nil {
		t.Fatalf("NewPoint failed: %v", err)
	}
	value, err := point.Value()
	if err != nil {
		t.Fatalf("Value failed: %v", err)
	}
	return value.([]byte)
}

func mustEWKBType(t *testing.T, geometryType uint32) []byte {
	t.Helper()
	raw := make([]byte, 25)
	raw[0] = 1
	binary.LittleEndian.PutUint32(raw[1:5], geometryType|0x20000000)
	binary.LittleEndian.PutUint32(raw[5:9], 4326)
	binary.LittleEndian.PutUint64(raw[9:17], math.Float64bits(1))
	binary.LittleEndian.PutUint64(raw[17:25], math.Float64bits(1))
	return raw
}
