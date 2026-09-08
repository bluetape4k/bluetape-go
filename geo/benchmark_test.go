package geo

import (
	"fmt"
	"testing"
)

var (
	benchmarkPoint    Point
	benchmarkContains bool
	benchmarkDistance float64
	benchmarkHash     string
	benchmarkCell     Cell
)

func BenchmarkNewPoint(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		point, err := NewPoint(37.5665, 126.9780)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPoint = point
	}
}

func BenchmarkBoundsContains(b *testing.B) {
	fixtures := []struct {
		name                     string
		west, south, east, north float64
		latitude, longitude      float64
	}{
		{name: "ordinary", west: 120, south: 30, east: 130, north: 40, latitude: 37.5665, longitude: 126.9780},
		{name: "antimeridian", west: 170, south: -10, east: -170, north: 10, latitude: 0, longitude: 180},
	}
	for _, fixture := range fixtures {
		b.Run(fixture.name, func(b *testing.B) {
			bounds, err := NewBounds(fixture.west, fixture.south, fixture.east, fixture.north)
			if err != nil {
				b.Fatal(err)
			}
			point, err := NewPoint(fixture.latitude, fixture.longitude)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkContains = bounds.Contains(point)
			}
		})
	}
}

func BenchmarkDistanceMeters(b *testing.B) {
	left, err := NewPoint(37.5665, 126.9780)
	if err != nil {
		b.Fatal(err)
	}
	right, err := NewPoint(35.1796, 129.0756)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		distance, err := DistanceMeters(left, right)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDistance = distance
	}
}

func BenchmarkEncode(b *testing.B) {
	point, err := NewPoint(57.64911, 10.40744)
	if err != nil {
		b.Fatal(err)
	}
	for _, precision := range []int{1, 12} {
		b.Run(fmt.Sprintf("precision-%d", precision), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hash, err := Encode(point, precision)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkHash = hash
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	for _, hash := range []string{"u", "u4pruydqqvj8"} {
		b.Run(fmt.Sprintf("precision-%d", len(hash)), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cell, err := Decode(hash)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkCell = cell
			}
		})
	}
}
