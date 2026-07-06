package mongodbtestcontainer_test

import (
	"context"
	"os"
	"testing"
	"time"

	mongodbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mongodb"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestStartMongoDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	srv := mongodbtestcontainer.StartServer(ctx, t)
	details, err := srv.ConnectionDetails(ctx)
	if err != nil {
		t.Fatalf("mongodb server details: %v", err)
	}
	uri, err := details.Require(mongodbtestcontainer.URIKey)
	if err != nil {
		t.Fatalf("mongodb uri detail: %v", err)
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongodb: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		if err := client.Disconnect(cleanupCtx); err != nil {
			t.Fatalf("disconnect mongodb client: %v", err)
		}
	})

	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping mongodb: %v", err)
	}

	collection := client.Database("bluetape_test").Collection("fixture")
	if _, err := collection.InsertOne(ctx, bson.M{"_id": "probe", "value": "ok"}); err != nil {
		t.Fatalf("insert mongodb document: %v", err)
	}
	var out struct {
		Value string `bson:"value"`
	}
	if err := collection.FindOne(ctx, bson.M{"_id": "probe"}).Decode(&out); err != nil {
		t.Fatalf("find mongodb document: %v", err)
	}
	if out.Value != "ok" {
		t.Fatalf("mongodb value = %q, want ok", out.Value)
	}
}

func TestStartReturnsURI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	uri := mongodbtestcontainer.Start(ctx, t)
	if uri == "" {
		t.Fatal("Start returned blank URI")
	}
}

func TestConnectionDetailKey(t *testing.T) {
	if mongodbtestcontainer.URIKey != "mongodb.uri" {
		t.Fatalf("URIKey = %q", mongodbtestcontainer.URIKey)
	}
}

func TestExportEnvMapping(t *testing.T) {
	details := tcserver.ConnectionDetails{mongodbtestcontainer.URIKey: "mongodb://127.0.0.1:27017/"}

	if err := tcserver.ExportEnv(t, details, map[string]string{
		mongodbtestcontainer.URIKey: "BLUETAPE_MONGODB_URI",
	}); err != nil {
		t.Fatalf("export mongodb env: %v", err)
	}
	if got := os.Getenv("BLUETAPE_MONGODB_URI"); got != "mongodb://127.0.0.1:27017/" {
		t.Fatalf("BLUETAPE_MONGODB_URI = %q", got)
	}
}
