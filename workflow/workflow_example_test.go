package workflow_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/workflow"
	"github.com/bluetape4k/bluetape-go/workreport"
)

func ExampleSequential() {
	runner := workflow.Sequential(
		"import",
		workreport.ContinueOnFailure,
		func(context.Context) workreport.Report { return workreport.Completed("read") },
		func(context.Context) workreport.Report { return workreport.Failed("write", errors.New("disk full")) },
	)

	report := runner.Run(context.Background())
	fmt.Println(report.Status)
	fmt.Println(len(report.Children))

	// Output:
	// partial
	// 2
}

func ExampleConditional() {
	runner := workflow.Conditional(
		"sync",
		func(context.Context) (bool, error) { return true, nil },
		func(context.Context) workreport.Report { return workreport.Completed("remote") },
		func(context.Context) workreport.Report { return workreport.Completed("local") },
	)

	report := runner.Run(context.Background())
	fmt.Println(report.Status)
	fmt.Println(report.Children[0].Name)

	// Output:
	// completed
	// remote
}

func ExampleParallel() {
	runner := workflow.Parallel(
		"fanout",
		workreport.ContinueOnFailure,
		func(context.Context) workreport.Report { return workreport.Completed("left") },
		func(context.Context) workreport.Report { return workreport.Completed("right") },
	)

	report := runner.Run(context.Background())
	fmt.Println(report.Status)
	fmt.Println(report.Children[1].Name)

	// Output:
	// completed
	// right
}
