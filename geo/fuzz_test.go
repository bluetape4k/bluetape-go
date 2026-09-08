package geo

import (
	"math"
	"testing"
)

func FuzzDecode(f *testing.F) {
	for _, seed := range []string{"u4pruydqqvj", "s", "", "UPPER", "u4pru y"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, hash string) {
		cell, err := Decode(hash)
		if err != nil {
			return
		}
		if err := cell.Validate(); err != nil {
			t.Fatalf("successful Decode returned invalid cell: %v", err)
		}
		if !cell.Bounds().Contains(cell.Center()) {
			t.Fatal("successful Decode returned center outside bounds")
		}
	})
}

func FuzzEncodeDecodeContains(f *testing.F) {
	for _, seed := range []struct {
		latitude, longitude float64
		precision           uint8
	}{
		{0, 0, 1}, {57.64911, 10.40744, 11}, {-90, -180, 12}, {90, 180, 12},
		{math.Nextafter(0, -1), math.Nextafter(0, 1), 12},
	} {
		f.Add(seed.latitude, seed.longitude, seed.precision)
	}
	f.Fuzz(func(t *testing.T, latitude, longitude float64, rawPrecision uint8) {
		point, err := NewPoint(latitude, longitude)
		if err != nil {
			return
		}
		precision := int(rawPrecision%maximumPrecision) + minimumPrecision
		hash, err := Encode(point, precision)
		if err != nil {
			t.Fatalf("Encode(valid point) error = %v", err)
		}
		cell, err := Decode(hash)
		if err != nil || !cell.Bounds().Contains(point) {
			t.Fatalf("round trip hash=%q cell=%#v error=%v", hash, cell, err)
		}
	})
}
