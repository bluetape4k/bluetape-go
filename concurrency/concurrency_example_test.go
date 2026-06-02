package concurrency_test

import (
	"context"
	"fmt"

	"github.com/bluetape4k/bluetape-go/concurrency"
)

func ExampleMap() {
	values, err := concurrency.Map(context.Background(), []int{1, 2, 3}, 2, func(_ context.Context, value int) (int, error) {
		return value * value, nil
	})
	if err != nil {
		return
	}

	fmt.Println(values)

	// Output:
	// [1 4 9]
}

func ExampleWorkerPool() {
	jobs := make(chan int)
	go func() {
		defer close(jobs)
		for _, value := range []int{1, 2, 3} {
			jobs <- value
		}
	}()

	var total int
	pool, err := concurrency.NewWorkerPool[int](1, func(_ context.Context, value int) error {
		total += value
		return nil
	})
	if err != nil {
		return
	}
	if err := pool.Run(context.Background(), jobs); err != nil {
		return
	}

	fmt.Println(total)

	// Output:
	// 6
}
