package geo_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/geo"
)

func Example() {
	if err := printExample(); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// 111195
	// true
	// u4pruydqqvj
	// true
}

func printExample() error {
	origin, err := geo.NewPoint(0, 0)
	if err != nil {
		return err
	}
	oneDegreeEast, err := geo.NewPoint(0, 1)
	if err != nil {
		return err
	}
	distance, err := geo.DistanceMeters(origin, oneDegreeEast)
	if err != nil {
		return err
	}
	crossing, err := geo.NewBounds(170, -10, -170, 10)
	if err != nil {
		return err
	}
	nearDateLine, err := geo.NewPoint(0, 179)
	if err != nil {
		return err
	}
	point, err := geo.NewPoint(57.64911, 10.40744)
	if err != nil {
		return err
	}
	hash, err := geo.Encode(point, 11)
	if err != nil {
		return err
	}
	cell, err := geo.Decode(hash)
	if err != nil {
		return err
	}

	fmt.Printf("%.0f\n", distance)
	fmt.Println(crossing.Contains(nearDateLine))
	fmt.Println(hash)
	fmt.Println(cell.Bounds().Contains(point))
	return nil
}
