package probabilistic_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/probabilistic"
)

func ExampleBloomFilter_membership() {
	filter, _ := probabilistic.NewStringBloomFilter(probabilistic.DefaultConfig())

	filter.Put("alpha")

	fmt.Println(filter.MightContain("alpha"))
	fmt.Println(filter.MightContain("missing"))

	// Output:
	// true
	// false
}

func ExampleBloomFilter_PutAll() {
	cfg, _ := probabilistic.NewConfig(1_000, 0.01)
	left, _ := probabilistic.NewStringBloomFilter(cfg)
	right, _ := probabilistic.NewStringBloomFilter(cfg)

	left.Put("left")
	right.Put("right")
	_ = left.PutAll(right)

	fmt.Println(left.MightContain("left"))
	fmt.Println(left.MightContain("right"))

	// Output:
	// true
	// true
}

func ExampleBloomFilter_introspection() {
	cfg, _ := probabilistic.NewConfig(1_000, 0.01)
	filter, _ := probabilistic.NewStringBloomFilter(cfg)
	filter.Put("alpha")

	fmt.Println(filter.ExpectedInsertions())
	fmt.Println(filter.HashFunctionCount() > 0)
	fmt.Println(filter.ApproximateElementCount() > 0)

	// Output:
	// 1000
	// true
	// true
}
