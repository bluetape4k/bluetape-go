package id_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/id"
)

func ExampleNewUUIDV7() {
	value, err := id.NewUUIDV7()
	if err != nil {
		return
	}
	parsed, err := id.ParseUUID(value)
	if err != nil {
		return
	}

	fmt.Println(len(value))
	fmt.Println(parsed == value)

	// Output:
	// 36
	// true
}

func ExampleNewUUIDV7Generator_withClock() {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	generator, err := id.NewUUIDV7Generator(
		id.WithUUIDTime(func() time.Time { return fixed }),
		id.WithUUIDReader(strings.NewReader("abcdefghijklmnopqrstuvwxyzabcdef")),
	)
	if err != nil {
		return
	}

	first, err := generator.NextString()
	if err != nil {
		return
	}
	second, err := generator.NextString()
	if err != nil {
		return
	}

	fmt.Println(len(first))
	fmt.Println(first < second)

	// Output:
	// 36
	// true
}

func ExampleNewMonotonicULIDGenerator() {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	generator, err := id.NewMonotonicULIDGenerator(
		id.WithULIDTime(func() time.Time { return fixed }),
		id.WithULIDEntropy(strings.NewReader("abcdefghij\x01\x00\x00\x00")),
	)
	if err != nil {
		return
	}

	first, err := generator.NextString()
	if err != nil {
		return
	}
	second, err := generator.NextString()
	if err != nil {
		return
	}

	fmt.Println(len(first))
	fmt.Println(first < second)

	// Output:
	// 26
	// true
}

func ExampleNewKSUIDGenerator() {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	generator, err := id.NewKSUIDGenerator(
		id.WithKSUIDTime(func() time.Time { return fixed }),
		id.WithKSUIDEntropy(strings.NewReader("abcdefghijklmnop")),
	)
	if err != nil {
		return
	}

	value, err := generator.NextString()
	if err != nil {
		return
	}
	parsed, err := id.ParseKSUID(value)
	if err != nil {
		return
	}
	createdAt, err := id.KSUIDTime(value)
	if err != nil {
		return
	}

	fmt.Println(len(value))
	fmt.Println(parsed == value)
	fmt.Println(createdAt.Equal(fixed))

	// Output:
	// 27
	// true
	// true
}

func ExampleNewSnowflakeGenerator() {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := epoch.Add(42 * time.Millisecond)
	generator, err := id.NewSnowflakeGenerator(
		7,
		id.WithSnowflakeEpoch(epoch),
		id.WithSnowflakeTime(func() time.Time { return now }),
	)
	if err != nil {
		return
	}

	value, err := generator.NextInt64()
	if err != nil {
		return
	}
	parts, err := id.DecodeSnowflake(value, id.WithSnowflakeEpoch(epoch))
	if err != nil {
		return
	}

	fmt.Println(parts.MachineID)
	fmt.Println(parts.Sequence)
	fmt.Println(parts.Time.Equal(now))

	// Output:
	// 7
	// 0
	// true
}
