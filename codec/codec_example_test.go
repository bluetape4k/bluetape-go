package codec_test

import (
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/codec"
	"github.com/bluetape4k/bluetape-go/core"
)

func ExampleEncodeBase58String() {
	encoded := codec.EncodeBase58String("Hello, World!")
	decoded, err := codec.DecodeBase58String(encoded)
	if err != nil {
		return
	}

	fmt.Println(encoded)
	fmt.Println(decoded)

	// Output:
	// 72k1xXWG59fYdzSNoA
	// Hello, World!
}

func ExampleEncodeBase62String() {
	encoded := codec.EncodeBase62String("bluetape-go")
	decoded, err := codec.DecodeBase62String(encoded)
	if err != nil {
		return
	}

	fmt.Println(encoded)
	fmt.Println(decoded)

	// Output:
	// 9aqg9ERZ3xWGzjr
	// bluetape-go
}

func ExampleEncodeBase64URL() {
	encoded := codec.EncodeBase64URL([]byte{251, 255, 255})
	decoded, err := codec.DecodeBase64URL(encoded)
	if err != nil {
		return
	}

	fmt.Println(encoded)
	fmt.Println(decoded[0], decoded[1], decoded[2])

	// Output:
	// -___
	// 251 255 255
}

func ExampleEncodeHexString() {
	encoded := codec.EncodeHexString("Hello, World!")
	decoded, err := codec.DecodeHexString(encoded)
	if err != nil {
		return
	}

	fmt.Println(encoded)
	fmt.Println(decoded)

	// Output:
	// 48656c6c6f2c20576f726c6421
	// Hello, World!
}

func ExampleDecodeBase64String_invalidUTF8() {
	encoded := codec.EncodeBase64([]byte{0xff, 0xfe})
	if _, err := codec.DecodeBase64String(encoded); errors.Is(err, core.ErrInvalidUTF8) {
		fmt.Println("invalid text")
	}

	decoded, err := codec.DecodeBase64(encoded)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(decoded))

	// Output:
	// invalid text
	// 2
}
