package ssm_test

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	awssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/bluetape4k/bluetape-go/ssm"
)

type exampleClient struct{}

func (exampleClient) GetParameter(ctx context.Context, input *awssm.GetParameterInput, _ ...func(*awssm.Options)) (*awssm.GetParameterOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value := "example-parameter"
	return &awssm.GetParameterOutput{Parameter: &awssmtypes.Parameter{Name: input.Name, Value: aws.String(value)}}, nil
}

func ExampleProvider() {
	provider, err := ssm.New(ssm.Options{Client: exampleClient{}})
	if err != nil {
		panic(err)
	}
	value, err := provider.Get(context.Background(), "/example/parameter")
	if err != nil {
		panic(err)
	}
	fmt.Println(value.Text())
	// Output: example-parameter
}

func ExampleProvider_GetSecure() {
	provider, err := ssm.New(ssm.Options{Client: exampleClient{}})
	if err != nil {
		panic(err)
	}
	value, err := provider.GetSecure(context.Background(), "/example/secure")
	if err != nil {
		panic(err)
	}
	fmt.Println(value.IsSet())
	// Output: true
}
