package mongoleader

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes는 leader backend election에서 생성과 초기화 계약을 설명한다.
//
// 이 주석은 backend lease, ownership, consistency, cancellation 조건을 설명한다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
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
