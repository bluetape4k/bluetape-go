package mongo_test

import (
	"context"
	"time"

	"github.com/bluetape4k/bluetape-go/jwt"
	mongojwt "github.com/bluetape4k/bluetape-go/jwt/mongo"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func ExampleRepository_distributedHMACProvider() {
	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelSetup()

	client, err := mongodriver.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo, err := mongojwt.New(mongojwt.Options{
		Client:    client,
		Database:  "service_auth",
		Namespace: "service-auth",
	})
	if err != nil {
		panic(err)
	}
	provider, err := jwt.NewDistributedHMACProvider(setupCtx, repo, jwt.HS256)
	if err != nil {
		panic(err)
	}

	opCtx, cancelOp := context.WithTimeout(context.Background(), time.Second)
	defer cancelOp()

	token, err := provider.ComposeContext(opCtx, jwt.WithSubject("account-42"), jwt.WithExpiresAfter(time.Hour))
	if err != nil {
		panic(err)
	}
	reader, err := provider.ParseContext(opCtx, token, jwt.WithExpectedSubject("account-42"))
	if err != nil {
		panic(err)
	}
	_ = reader
}
