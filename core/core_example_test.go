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
	fmt.Println(core.NoText("  "))

	// Output:
	// fallback
	// fallback
	// true
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

func ExampleMask() {
	fmt.Println(core.Mask("secret", '#'))
	fmt.Println(core.CommonPrefix("안녕-blue", "안녕-red"))

	// Output:
	// ######
	// 안녕-
}

func ExampleCanonicalUUID() {
	value, err := core.CanonicalUUID("24738134-9D88-6645-4EC8-D63AA2031015")
	if err != nil {
		return
	}

	fmt.Println(value)
	fmt.Println(core.IsZeroUUID(core.ZeroUUID))

	// Output:
	// 24738134-9d88-6645-4ec8-d63aa2031015
	// true
}

func ExampleFirstNonZero() {
	fmt.Println(core.FirstNonZero(0, 0, 42, 7))

	// Output:
	// 42
}
