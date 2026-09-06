package mysqlgis_test

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit/mysqlgis"
)

func TestPointValueScanRoundTripAndNull(t *testing.T) {
	want, err := mysqlgis.NewPoint(127.0276, 37.4979, 4326)
	if err != nil {
		t.Fatal(err)
	}
	value, err := want.Value()
	if err != nil {
		t.Fatal(err)
	}
	var got mysqlgis.Point
	if err := got.Scan(mysqlgis.ScannedPoint{WKB: value.([]byte), SRID: 4326}); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if err := got.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if got.Valid {
		t.Fatalf("NULL scan retained value: %#v", got)
	}
}

func TestPointAcceptsInternalSRIDPrefixAndRejectsMalformed(t *testing.T) {
	point, err := mysqlgis.NewPoint(1, 2, 3857)
	if err != nil {
		t.Fatal(err)
	}
	wkb, _ := point.MarshalWKB()
	raw := make([]byte, 4+len(wkb))
	binary.LittleEndian.PutUint32(raw, 3857)
	copy(raw[4:], wkb)
	var got mysqlgis.Point
	if err := got.Scan(raw); err != nil {
		t.Fatal(err)
	}
	if got != point {
		t.Fatalf("got %#v, want %#v", got, point)
	}
	if err := got.Scan([]byte{1, 1}); !errors.Is(err, mysqlgis.ErrInvalidPoint) || got.Valid {
		t.Fatalf("malformed scan error=%v point=%#v", err, got)
	}
}

func TestPointRejectsInvalidCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name string
		x, y float64
		want error
	}{
		{"nan", math.NaN(), 0, mysqlgis.ErrInvalidPoint},
		{"longitude", 181, 0, mysqlgis.ErrInvalidPoint},
		{"latitude", 0, 91, mysqlgis.ErrInvalidPoint},
		{"srid", 0, 0, mysqlgis.ErrInvalidSRID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srid := 4326
			if tc.name == "srid" {
				srid = -1
			}
			_, err := mysqlgis.NewPoint(tc.x, tc.y, srid)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want=%v", err, tc.want)
			}
		})
	}
}
