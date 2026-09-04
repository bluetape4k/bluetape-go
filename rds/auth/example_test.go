package auth_test

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdsauth "github.com/bluetape4k/bluetape-go/rds/auth"
)

type exampleCredentials struct{}

func (exampleCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "secret-access-key",
		Source:          "example",
	}, nil
}

func ExampleBuildAuthToken_postgres() {
	token, err := rdsauth.BuildAuthToken(context.Background(), rdsauth.Request{
		Endpoint: "postgres.example.com:5432",
		Region:   "ap-northeast-2",
		Username: "app_user",
	}, exampleCredentials{})
	if err != nil {
		panic(err)
	}
	// 호출자는 token.Text()를 PostgreSQL driver의 password field에 전달한다.
	fmt.Println(token.IsSet())
	// Output: true
}

func ExampleBuildAuthToken_mysql() {
	token, err := rdsauth.BuildAuthToken(context.Background(), rdsauth.Request{
		Endpoint: "mysql.example.com:3306",
		Region:   "ap-northeast-2",
		Username: "app_user",
	}, exampleCredentials{})
	if err != nil {
		panic(err)
	}
	// 호출자는 token.Text()를 MySQL driver의 password field에 전달한다.
	fmt.Println(token.Len() > 0)
	// Output: true
}
