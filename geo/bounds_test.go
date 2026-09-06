package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestBoundsAccessorsAndValidation(t *testing.T) {
	bounds, err := NewBounds(120, 30, 130, 40)
	if err != nil {
		t.Fatalf("NewBounds failed: %v", err)
	}
	if bounds.West() != 120 || bounds.South() != 30 || bounds.East() != 130 || bounds.North() != 40 {
		t.Fatalf("bounds = %#v", bounds)
	}
	if bounds.CrossesAntimeridian() {
		t.Fatal("ordinary bounds must not cross antimeridian")
	}
	if err := (Bounds{}).Validate(); err != nil {
		t.Fatalf("zero Bounds must be valid: %v", err)
	}
}

func TestBoundsContainsInclusiveAndCrossingCoordinates(t *testing.T) {
	ordinary, _ := NewBounds(120, 30, 130, 40)
	crossing, _ := NewBounds(170, -10, -170, 10)
	full, _ := NewBounds(-180, -90, 180, 90)
	degenerate, _ := NewBounds(0, 0, 0, 0)

	assertContains := func(t *testing.T, bounds Bounds, latitude, longitude float64, want bool) {
		t.Helper()
		point, err := NewPoint(latitude, longitude)
		if err != nil {
			t.Fatalf("NewPoint failed: %v", err)
		}
		if got := bounds.Contains(point); got != want {
			t.Fatalf("Contains(%v, %v) = %v, want %v", latitude, longitude, got, want)
		}
	}

	assertContains(t, ordinary, 30, 120, true)
	assertContains(t, ordinary, 40, 130, true)
	assertContains(t, ordinary, 35, 119.999, false)
	assertContains(t, crossing, 0, 179, true)
	assertContains(t, crossing, 0, -179, true)
	assertContains(t, crossing, 0, 0, false)
	assertContains(t, full, -90, -180, true)
	assertContains(t, full, 90, 180, true)
	assertContains(t, degenerate, 0, 0, true)
	if !crossing.CrossesAntimeridian() {
		t.Fatal("crossing bounds must report antimeridian crossing")
	}
}

func TestBoundsTreatsMinusAndPlus180AsSameMeridian(t *testing.T) {
	westEdge, _ := NewBounds(-180, -10, -170, 10)
	eastEdge, _ := NewBounds(170, -10, 180, 10)
	plus180, _ := NewPoint(0, 180)
	minus180, _ := NewPoint(0, -180)
	if !westEdge.Contains(plus180) || !eastEdge.Contains(minus180) {
		t.Fatal("-180 and 180 must be equivalent for boundary inclusion")
	}
}

func TestNewBoundsRejectsInvalidFieldsInOrder(t *testing.T) {
	for _, input := range []struct {
		name, field              string
		west, south, east, north float64
	}{
		{name: "west NaN", field: "west", west: math.NaN()},
		{name: "west positive infinity", field: "west", west: math.Inf(1)},
		{name: "west negative infinity", field: "west", west: math.Inf(-1)},
		{name: "south NaN", field: "south", west: 0, south: math.NaN()},
		{name: "south positive infinity", field: "south", west: 0, south: math.Inf(1)},
		{name: "south negative infinity", field: "south", west: 0, south: math.Inf(-1)},
		{name: "east NaN", field: "east", west: 0, south: 0, east: math.NaN()},
		{name: "east positive infinity", field: "east", west: 0, south: 0, east: math.Inf(1)},
		{name: "east negative infinity", field: "east", west: 0, south: 0, east: math.Inf(-1)},
		{name: "north NaN", field: "north", west: 0, south: 0, east: 0, north: math.NaN()},
		{name: "north positive infinity", field: "north", west: 0, south: 0, east: 0, north: math.Inf(1)},
		{name: "north negative infinity", field: "north", west: 0, south: 0, east: 0, north: math.Inf(-1)},
		{name: "west below range", field: "west", west: -181},
		{name: "west above range", field: "west", west: 181},
		{name: "south below range", field: "south", west: 0, south: -91},
		{name: "south above range", field: "south", west: 0, south: 91},
		{name: "east below range", field: "east", west: 0, south: 0, east: -181},
		{name: "east above range", field: "east", west: 0, south: 0, east: 181},
		{name: "north below range", field: "north", west: 0, south: 0, east: 0, north: -91},
		{name: "north above range", field: "north", west: 0, south: 0, east: 0, north: 91},
		{name: "west before south", field: "west", west: math.NaN(), south: math.NaN()},
		{name: "south before east", field: "south", west: 0, south: math.NaN(), east: math.NaN()},
		{name: "east before north", field: "east", west: 0, south: 0, east: math.NaN(), north: math.NaN()},
		{name: "west range before south range", field: "west", west: 181, south: 91},
		{name: "south range before east range", field: "south", west: 0, south: 91, east: 181},
		{name: "east range before north range", field: "east", west: 0, south: 0, east: 181, north: 91},
		{name: "ordering", field: "south/north ordering", west: 0, south: 10, east: 0, north: 9},
	} {
		t.Run(input.name, func(t *testing.T) {
			bounds, err := NewBounds(input.west, input.south, input.east, input.north)
			if bounds != (Bounds{}) || !errors.Is(err, ErrInvalidBounds) {
				t.Fatalf("bounds=%#v error=%v", bounds, err)
			}
			if !strings.Contains(err.Error(), input.field) {
				t.Fatalf("error %q does not identify %s", err, input.field)
			}
		})
	}
}

func TestBoundsContainsRejectsPackageInternalInvalidPoint(t *testing.T) {
	bounds, _ := NewBounds(-180, -90, 180, 90)
	if bounds.Contains(Point{latitude: math.NaN()}) {
		t.Fatal("invalid internal Point must not be contained")
	}
}
