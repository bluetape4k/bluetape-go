package workreport_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/workreport"
)

func ExampleAggregate() {
	report, err := workreport.Aggregate(
		"import",
		workreport.ContinueOnFailure,
		workreport.Completed("read"),
		workreport.Failed("write", errors.New("disk full")),
	)
	if err != nil {
		return
	}

	fmt.Println(report.Status)
	fmt.Println(report.Children[1].Name)

	// Output:
	// partial
	// write
}

func ExampleCancelled() {
	report := workreport.Cancelled("sync", context.Canceled)

	fmt.Println(report.IsCancelled())
	fmt.Println(errors.Is(report.Err, context.Canceled))

	// Output:
	// true
	// true
}
