package collections_test

import (
	"fmt"
	"slices"

	"github.com/bluetape4k/bluetape-go/collections"
)

func ExampleChunk() {
	chunks, err := collections.Chunk([]int{1, 2, 3, 4, 5}, 2)
	if err != nil {
		return
	}

	fmt.Println(chunks)

	// Output:
	// [[1 2] [3 4] [5]]
}

func ExampleDistinctBy() {
	type member struct {
		ID   string
		Role string
	}

	members := []member{
		{ID: "a", Role: "admin"},
		{ID: "b", Role: "user"},
		{ID: "c", Role: "admin"},
	}
	distinct, err := collections.DistinctBy(members, func(value member) string {
		return value.Role
	})
	if err != nil {
		return
	}

	for _, member := range distinct {
		fmt.Println(member.ID, member.Role)
	}

	// Output:
	// a admin
	// b user
}

func ExampleGroupBy() {
	words := []string{"api", "app", "job", "jar"}
	groups, err := collections.GroupBy(words, func(value string) byte {
		return value[0]
	})
	if err != nil {
		return
	}

	keys := make([]byte, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	for _, key := range keys {
		fmt.Printf("%c: %v\n", key, groups[key])
	}

	// Output:
	// a: [api app]
	// j: [job jar]
}

func ExampleMapErr() {
	squares, err := collections.MapErr([]int{1, 2, 3}, func(value int) (int, error) {
		return value * value, nil
	})
	if err != nil {
		return
	}

	fmt.Println(squares)

	// Output:
	// [1 4 9]
}
