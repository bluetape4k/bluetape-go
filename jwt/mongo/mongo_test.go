package mongo

import (
	"testing"

	jwt "github.com/bluetape4k/bluetape-go/jwt"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestMongoFacadeNewReturnsRepository(t *testing.T) {
	client, err := mongodriver.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("mongo.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(t.Context()) })

	repo, err := New(Options{Client: client, Database: "jwt", Namespace: "prod"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, ok := any(repo).(*jwt.MongoRepository); !ok {
		t.Fatalf("New() returned %T, want *jwt.MongoRepository", repo)
	}
}
