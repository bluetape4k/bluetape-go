package mongoleader

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes creates optional cleanup indexes for the MongoDB elector.
//
// The TTL index is not used for correctness. Campaign and Leader decide lease
// validity with lease_until predicates so expired documents can be taken over
// before MongoDB's TTL monitor deletes them.
func EnsureIndexes(ctx context.Context, collection *mongo.Collection) error {
	if err := requireCollection(collection); err != nil {
		return err
	}
	_, err := collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "lease_until", Value: 1}},
			Options: options.Index().
				SetName("leader_mongo_lease_until_ttl").
				SetExpireAfterSeconds(0),
		},
		{
			Keys: bson.D{{Key: "group_key", Value: 1}, {Key: "lease_until", Value: 1}},
			Options: options.Index().
				SetName("leader_mongo_group_active"),
		},
	})
	if err != nil {
		return fmt.Errorf("mongo leader ensure indexes: %w", err)
	}
	return nil
}
