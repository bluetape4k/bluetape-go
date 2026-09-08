package geo

import (
	"errors"
	"math"
	"testing"
)

func TestDistanceMetersKnownValuesAndSymmetry(t *testing.T) {
	seoul, _ := NewPoint(37.5665, 126.9780)
	busan, _ := NewPoint(35.1796, 129.0756)
	forward, err := DistanceMeters(seoul, busan)
	if err != nil {
		t.Fatalf("DistanceMeters failed: %v", err)
	}
	reverse, err := DistanceMeters(busan, seoul)
	if err != nil {
		t.Fatalf("DistanceMeters reverse failed: %v", err)
	}
	if math.Abs(forward-325_000) > 2_000 {
		t.Fatalf("Seoul-Busan distance = %v", forward)
	}
	if math.Abs(forward-reverse) > 1e-9 {
		t.Fatalf("distance is not symmetric: %v vs %v", forward, reverse)
	}
}

func TestDistanceMetersHandlesSameMeridianRepresentations(t *testing.T) {
	minus180, _ := NewPoint(0, -180)
	plus180, _ := NewPoint(0, 180)
	distance, err := DistanceMeters(minus180, plus180)
	if err != nil || distance != 0 {
		t.Fatalf("equivalent meridian distance=%v error=%v", distance, err)
	}

	west, _ := NewPoint(0, 179.9)
	east, _ := NewPoint(0, -179.9)
	distance, err = DistanceMeters(west, east)
	if err != nil {
		t.Fatalf("antimeridian distance failed: %v", err)
	}
	if math.Abs(distance-22_239) > 100 {
		t.Fatalf("antimeridian distance = %v", distance)
	}
}

func TestDistanceMetersIsFiniteForAntipodes(t *testing.T) {
	left, _ := NewPoint(0, 0)
	right, _ := NewPoint(0, 180)
	distance, err := DistanceMeters(left, right)
	if err != nil {
		t.Fatalf("DistanceMeters failed: %v", err)
	}
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance < 0 {
		t.Fatalf("distance = %v", distance)
	}
}

func TestDistanceMetersIsFiniteAtAndNearPoles(t *testing.T) {
	for _, coordinates := range [][4]float64{
		{90, 0, 90, 180},
		{-90, 0, 90, 180},
		{89.999999, -179.999999, 89.999999, 179.999999},
		{-89.999999, -45, -89.999999, 135},
	} {
		left, err := NewPoint(coordinates[0], coordinates[1])
		if err != nil {
			t.Fatal(err)
		}
		right, err := NewPoint(coordinates[2], coordinates[3])
		if err != nil {
			t.Fatal(err)
		}
		distance, err := DistanceMeters(left, right)
		if err != nil || math.IsNaN(distance) || math.IsInf(distance, 0) || distance < 0 {
			t.Fatalf("coordinates=%v distance=%v error=%v", coordinates, distance, err)
		}
	}
}

func TestDistanceMetersValidatesLeftBeforeRight(t *testing.T) {
	invalidLeft := Point{latitude: math.NaN()}
	invalidRight := Point{longitude: math.Inf(1)}
	distance, err := DistanceMeters(invalidLeft, invalidRight)
	if distance != 0 || !errors.Is(err, ErrInvalidPoint) {
		t.Fatalf("distance=%v error=%v", distance, err)
	}
}
