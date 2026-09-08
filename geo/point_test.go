package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestNewPointAndAccessors(t *testing.T) {
	point, err := NewPoint(37.5665, 126.9780)
	if err != nil {
		t.Fatalf("NewPoint failed: %v", err)
	}
	if point.Latitude() != 37.5665 || point.Longitude() != 126.9780 {
		t.Fatalf("point = (%v, %v)", point.Latitude(), point.Longitude())
	}
	if err := point.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if err := (Point{}).Validate(); err != nil {
		t.Fatalf("zero Point must be valid: %v", err)
	}
}

func TestNewPointAcceptsClosedWGS84RangeAndSignedZero(t *testing.T) {
	for _, input := range []struct {
		name      string
		latitude  float64
		longitude float64
	}{
		{name: "south-west", latitude: -90, longitude: -180},
		{name: "north-east", latitude: 90, longitude: 180},
		{name: "signed-zero", latitude: math.Copysign(0, -1), longitude: math.Copysign(0, -1)},
	} {
		t.Run(input.name, func(t *testing.T) {
			point, err := NewPoint(input.latitude, input.longitude)
			if err != nil {
				t.Fatalf("NewPoint failed: %v", err)
			}
			if point.Latitude() != input.latitude || point.Longitude() != input.longitude {
				t.Fatalf("point = (%v, %v)", point.Latitude(), point.Longitude())
			}
		})
	}
}

func TestNewPointRejectsNonFiniteAndOutOfRangeValuesInFieldOrder(t *testing.T) {
	for _, input := range []struct {
		name      string
		latitude  float64
		longitude float64
		field     string
	}{
		{name: "latitude NaN", latitude: math.NaN(), field: "latitude"},
		{name: "latitude infinity", latitude: math.Inf(1), field: "latitude"},
		{name: "latitude low", latitude: -90.0001, field: "latitude"},
		{name: "latitude high", latitude: 90.0001, field: "latitude"},
		{name: "longitude NaN", longitude: math.NaN(), field: "longitude"},
		{name: "longitude infinity", longitude: math.Inf(-1), field: "longitude"},
		{name: "longitude low", longitude: -180.0001, field: "longitude"},
		{name: "longitude high", longitude: 180.0001, field: "longitude"},
		{name: "latitude wins", latitude: 91, longitude: 181, field: "latitude"},
	} {
		t.Run(input.name, func(t *testing.T) {
			point, err := NewPoint(input.latitude, input.longitude)
			if point != (Point{}) {
				t.Fatalf("failure result = %#v", point)
			}
			if !errors.Is(err, ErrInvalidPoint) {
				t.Fatalf("error = %v", err)
			}
			if !strings.Contains(err.Error(), input.field) {
				t.Fatalf("error %q does not identify %s", err, input.field)
			}
		})
	}
}
