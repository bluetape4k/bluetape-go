package btredis

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func ExampleNewOwnerToken() {
	token, err := NewOwnerToken()
	if err != nil {
		panic(err)
	}
	fmt.Println(token.String())
	// Output:
	// redis-owner-token:<redacted>
}

func ExampleKeyBuilder_LogicalKey() {
	builder, err := NewKeyBuilder("bluetape:lock:v1")
	if err != nil {
		panic(err)
	}
	key, err := builder.LogicalKey(" caller-owned:key ")
	if err != nil {
		panic(err)
	}
	fmt.Println(key.RedactedID)
	// Output:
	// redis-key:0ef010269cf9cb3509c4a9b1
}

func ExampleKeyBuilder_StructuralKey() {
	builder, err := NewKeyBuilder("bluetape:probabilistic:bloom:v1")
	if err != nil {
		panic(err)
	}
	builder, err = builder.WithHashTag("tenant:shared")
	if err != nil {
		panic(err)
	}
	key, err := builder.StructuralKey("bits")
	if err != nil {
		panic(err)
	}
	fmt.Println(key.RedactedID)
	// Output:
	// redis-key:a818d2572f7a83eeb6c4fe50
}

func ExampleCompareAndDelete() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	lease := exampleLease()
	ok, err := CompareAndDelete(ctx, &fakeScripter{result: 0}, lease, "redis example")
	if err != nil {
		panic(err)
	}
	if !ok {
		fmt.Println("ownership drift")
	}
	// Output:
	// ownership drift
}

func ExampleOpError() {
	err := NewOpError(
		OpLabels{Family: "redis lock", Operation: "release"},
		"caller:key",
		context.DeadlineExceeded,
	)
	fmt.Println(errors.Is(err, context.DeadlineExceeded))
	// Output:
	// true
}

func exampleLease() Lease {
	token, err := NewOwnerToken()
	if err != nil {
		panic(err)
	}
	lease, err := NewLease("caller:key", token)
	if err != nil {
		panic(err)
	}
	return lease
}
