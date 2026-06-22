package core_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/core"
)

func ExampleClosedOpenRange() {
	r, err := core.ClosedOpenRange(10, 20)
	if err != nil {
		return
	}

	fmt.Println(r.Contains(10))
	fmt.Println(r.Contains(20))
	fmt.Println(r.String())

	// Output:
	// true
	// false
	// [10,20)
}

func ExampleRange_Overlaps() {
	left, err := core.ClosedRange(1, 5)
	if err != nil {
		return
	}
	right, err := core.OpenOpenRange(5, 8)
	if err != nil {
		return
	}

	fmt.Println(left.Overlaps(right))

	// Output:
	// false
}
