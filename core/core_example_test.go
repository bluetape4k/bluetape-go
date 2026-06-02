package core_test

import (
	"fmt"

	"github.com/bluetape4k/bluetape-go/core"
)

func ExamplePtr() {
	value := core.Ptr("worker-1")

	fmt.Println(*value)

	// Output:
	// worker-1
}

func ExampleBlankToDefault() {
	fmt.Println(core.BlankToDefault("  ", "fallback"))
	fmt.Println(core.EmptyToDefault("", "fallback"))

	// Output:
	// fallback
	// fallback
}

func ExampleClamp() {
	value, err := core.Clamp(120, 0, 100)
	if err != nil {
		return
	}

	fmt.Println(value)

	// Output:
	// 100
}

func ExampleTruncateUTF8Bytes() {
	value, err := core.TruncateUTF8Bytes("안녕하세요", 7)
	if err != nil {
		return
	}

	fmt.Println(value)

	// Output:
	// 안녕
}

func ExampleFirstNonZero() {
	fmt.Println(core.FirstNonZero(0, 0, 42, 7))

	// Output:
	// 42
}
