package stepfunctions

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

type exampleClient struct{}

func (exampleClient) StartExecution(context.Context, *sfn.StartExecutionInput, ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	return startOutput("arn:aws:states:ap-northeast-2:123456789012:execution:orders:example", time.Now().UTC()), nil
}

func (exampleClient) DescribeExecution(_ context.Context, input *sfn.DescribeExecutionInput, _ ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error) {
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:example"
	if input != nil && input.ExecutionArn != nil {
		executionARN = *input.ExecutionArn
	}
	return describeOutput("SUCCEEDED", executionARN, testStateMachineARN, time.Now().UTC()), nil
}

func ExampleNew() {
	bridge, err := New(Options{Client: exampleClient{}})
	if err != nil {
		fmt.Println(err)
		return
	}
	execution, err := bridge.Start(context.Background(), StartRequest{StateMachineARN: testStateMachineARN})
	if err != nil {
		fmt.Println(err)
		return
	}
	execution, err = bridge.Wait(context.Background(), execution.ExecutionARN, WaitOptions{Timeout: time.Second})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(execution.Status)
	// Output:
	// SUCCEEDED
}
