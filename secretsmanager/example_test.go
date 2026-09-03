package secretsmanager_test

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/bluetape4k/bluetape-go/secretsmanager"
)

type exampleClient struct{}

func (exampleClient) GetSecretValue(ctx context.Context, _ *awssm.GetSecretValueInput, _ ...func(*awssm.Options)) (*awssm.GetSecretValueOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &awssm.GetSecretValueOutput{SecretString: aws.String("example-secret")}, nil
}

func ExampleProvider() {
	provider, err := secretsmanager.New(secretsmanager.Options{Client: exampleClient{}})
	if err != nil {
		panic(err)
	}
	value, err := provider.Get(context.Background(), "example/secret")
	if err != nil {
		panic(err)
	}
	fmt.Printf("set=%t bytes=%d\n", value.IsSet(), len(value.Bytes()))
	// Output: set=true bytes=14
}
