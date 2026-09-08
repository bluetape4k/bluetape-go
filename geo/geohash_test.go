package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestEncodeKnownVectorAndPrecision(t *testing.T) {
	point, _ := NewPoint(57.64911, 10.40744)
	for index, want := range []string{
		"u", "u4", "u4p", "u4pr", "u4pru", "u4pruy",
		"u4pruyd", "u4pruydq", "u4pruydqq", "u4pruydqqv",
		"u4pruydqqvj", "u4pruydqqvj8",
	} {
		precision := index + 1
		hash, err := Encode(point, precision)
		if err != nil || hash != want {
			t.Fatalf("precision=%d hash=%q want=%q error=%v", precision, hash, want, err)
		}
		cell, err := Decode(hash)
		if err != nil || !cell.Bounds().Contains(point) {
			t.Fatalf("precision=%d round trip cell=%#v error=%v", precision, cell, err)
		}
	}
}

func TestEncodeSelectsUpperIntervalAtMidpoint(t *testing.T) {
	point, _ := NewPoint(0, 0)
	hash, err := Encode(point, 1)
	if err != nil || hash != "s" {
		t.Fatalf("midpoint hash=%q error=%v", hash, err)
	}
}

func TestDecodeReturnsValidContainingCell(t *testing.T) {
	original, _ := NewPoint(57.64911, 10.40744)
	cell, err := Decode("u4pruydqqvj")
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if err := cell.Validate(); err != nil {
		t.Fatalf("cell Validate failed: %v", err)
	}
	if !cell.Bounds().Contains(original) || !cell.Bounds().Contains(cell.Center()) {
		t.Fatalf("cell does not contain original and center: %#v", cell)
	}
	if cell.Center().Latitude() < cell.Bounds().South() || cell.Center().Latitude() > cell.Bounds().North() {
		t.Fatalf("center latitude outside bounds: %#v", cell)
	}
}

func TestEncodeDecodeContainsWGS84CornersAndAdjacentMidpoints(t *testing.T) {
	for _, coordinates := range [][2]float64{
		{-90, -180}, {-90, 180}, {90, -180}, {90, 180},
		{math.Nextafter(0, -1), math.Nextafter(0, -1)},
		{0, 0},
		{math.Nextafter(0, 1), math.Nextafter(0, 1)},
	} {
		point, err := NewPoint(coordinates[0], coordinates[1])
		if err != nil {
			t.Fatalf("NewPoint(%v) error = %v", coordinates, err)
		}
		for _, precision := range []int{1, 12} {
			first, err := Encode(point, precision)
			if err != nil {
				t.Fatalf("Encode(%v, %d) error = %v", coordinates, precision, err)
			}
			second, err := Encode(point, precision)
			if err != nil || second != first {
				t.Fatalf("Encode repeat = %q, want %q, error=%v", second, first, err)
			}
			cell, err := Decode(first)
			if err != nil || !cell.Bounds().Contains(point) {
				t.Fatalf("round trip point=%v precision=%d cell=%#v error=%v", coordinates, precision, cell, err)
			}
		}
	}
}

func TestDecodeRejectsNonCanonicalInput(t *testing.T) {
	for _, hash := range []string{"", "u4pruydqqvj0x", "U4PRUYDQQVJ", "u4pru y", "u4pruydqqva", "u4pruydqqvi"} {
		cell, err := Decode(hash)
		if cell != (Cell{}) || !errors.Is(err, ErrInvalidGeohash) {
			t.Fatalf("hash=%q cell=%#v error=%v", hash, cell, err)
		}
	}
}

func TestDecodeErrorPrecedenceAndCellValidationOrder(t *testing.T) {
	cell, err := Decode("UPPER-AND-TOO-LONG")
	if cell != (Cell{}) || !errors.Is(err, ErrInvalidGeohash) || !strings.Contains(err.Error(), "length") {
		t.Fatalf("Decode precedence cell=%#v error=%v", cell, err)
	}
	invalid := Cell{precision: 0, center: Point{latitude: math.NaN()}, bounds: Bounds{west: math.NaN()}}
	if err := invalid.Validate(); !errors.Is(err, ErrInvalidCell) || !strings.Contains(err.Error(), "precision") {
		t.Fatalf("Cell.Validate precedence error=%v", err)
	}
	invalidCenterAndBounds := Cell{precision: 1, center: Point{latitude: math.NaN()}, bounds: Bounds{west: math.NaN()}}
	if err := invalidCenterAndBounds.Validate(); !errors.Is(err, ErrInvalidCell) || !strings.Contains(err.Error(), "center") {
		t.Fatalf("Cell.Validate center precedence error=%v", err)
	}
	invalidBounds := Cell{precision: 1, center: Point{}, bounds: Bounds{west: math.NaN()}}
	if err := invalidBounds.Validate(); !errors.Is(err, ErrInvalidCell) || !strings.Contains(err.Error(), "bounds") {
		t.Fatalf("Cell.Validate bounds error=%v", err)
	}
}

func TestEncodeErrorPrecedenceAndZeroResult(t *testing.T) {
	invalid := Point{latitude: math.NaN()}
	hash, err := Encode(invalid, 0)
	if hash != "" || !errors.Is(err, ErrInvalidPoint) {
		t.Fatalf("hash=%q error=%v", hash, err)
	}
	valid, _ := NewPoint(0, 0)
	for _, precision := range []int{0, 13} {
		hash, err = Encode(valid, precision)
		if hash != "" || !errors.Is(err, ErrInvalidPrecision) {
			t.Fatalf("precision=%d hash=%q error=%v", precision, hash, err)
		}
	}
}

func TestCellZeroValueIsSafeButInvalid(t *testing.T) {
	var cell Cell
	if cell.Center() != (Point{}) || cell.Bounds() != (Bounds{}) {
		t.Fatalf("zero cell accessors changed: %#v", cell)
	}
	if !errors.Is(cell.Validate(), ErrInvalidCell) {
		t.Fatalf("zero cell error = %v", cell.Validate())
	}
}
