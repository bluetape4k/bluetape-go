package mariadbgis_test

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit/mariadbgis"
)

func TestPointValueScanRoundTripAndNull(t *testing.T) {
	want, err := mariadbgis.NewPoint(127.0276, 37.4979, 4326)
	if err != nil {
		t.Fatal(err)
	}
	value, err := want.Value()
	if err != nil {
		t.Fatal(err)
	}
	var got mariadbgis.Point
	if err := got.Scan(mariadbgis.ScannedPoint{WKB: value.([]byte), SRID: 4326}); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if err := got.Scan(nil); err != nil || got.Valid {
		t.Fatalf("NULL scan err=%v point=%#v", err, got)
	}
}

func TestPointAcceptsInternalSRIDPrefixAndRejectsMalformed(t *testing.T) {
	point, err := mariadbgis.NewPoint(1, 2, 3857)
	if err != nil {
		t.Fatal(err)
	}
	wkb, _ := point.MarshalWKB()
	raw := make([]byte, 4+len(wkb))
	binary.LittleEndian.PutUint32(raw, 3857)
	copy(raw[4:], wkb)
	var got mariadbgis.Point
	if err := got.Scan(raw); err != nil || got != point {
		t.Fatalf("internal scan err=%v point=%#v want=%#v", err, got, point)
	}
	if err := got.Scan([]byte{1, 1}); !errors.Is(err, mariadbgis.ErrInvalidPoint) || got.Valid {
		t.Fatalf("malformed scan err=%v point=%#v", err, got)
	}
}

func TestPointRejectsInvalidCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name string
		x, y float64
		want error
	}{
		{"nan", math.NaN(), 0, mariadbgis.ErrInvalidPoint},
		{"longitude", 181, 0, mariadbgis.ErrInvalidPoint},
		{"latitude", 0, 91, mariadbgis.ErrInvalidPoint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mariadbgis.NewPoint(tc.x, tc.y, 4326)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want=%v", err, tc.want)
			}
		})
	}
}
