package mongoleader

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/bluetape4k/bluetape-go/leader/leadertest"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoConformanceControl struct {
	collection *mongo.Collection
	mu         sync.Mutex
	failures   map[string]map[leadertest.Operation]error
	counts     map[string]map[leadertest.Operation]int64
}

func newMongoConformanceControl(collection *mongo.Collection) *mongoConformanceControl {
	return &mongoConformanceControl{
		collection: collection,
		failures:   make(map[string]map[leadertest.Operation]error),
		counts:     make(map[string]map[leadertest.Operation]int64),
	}
}

func (c *mongoConformanceControl) ReplaceOwner(ctx context.Context, opts leader.Options, owner string) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil || strings.TrimSpace(owner) == "" {
		return errors.New("mongo leader conformance: invalid control input")
	}
	now := time.Now().UTC()
	_, err = c.collection.UpdateOne(
		ctx,
		bson.M{"_id": mongoLeaderKey(normalized)},
		bson.M{
			"$set": bson.M{
				"group":       normalized.Group,
				"member_id":   "control",
				"token":       owner,
				"lease_until": now.Add(normalized.Lease),
				"updated_at":  now,
			},
			"$setOnInsert": bson.M{"created_at": now},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (c *mongoConformanceControl) FailNext(ctx context.Context, opts leader.Options, operation leadertest.Operation, cause error) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil || cause == nil || !validMongoOperation(operation) {
		return errors.New("mongo leader conformance: invalid failure injection")
	}
	key := mongoLeaderKey(normalized)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures[key] == nil {
		c.failures[key] = make(map[leadertest.Operation]error)
	}
	c.failures[key][operation] = cause
	return nil
}

func (c *mongoConformanceControl) Owner(ctx context.Context, opts leader.Options) (string, error) {
	if ctx == nil {
		return "", leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return "", errors.New("mongo leader conformance: invalid options")
	}
	var doc leaseDocument
	err = c.collection.FindOne(ctx, bson.M{
		"_id":         mongoLeaderKey(normalized),
		"lease_until": bson.M{"$gt": time.Now().UTC()},
	}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return doc.Token, nil
}

func (c *mongoConformanceControl) OperationCount(opts leader.Options, operation leadertest.Operation) int64 {
	normalized, err := opts.Normalize()
	if err != nil || !validMongoOperation(operation) {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[mongoLeaderKey(normalized)][operation]
}

func (c *mongoConformanceControl) after(opts leader.Options, rawOperation string) error {
	operation := leadertest.Operation(rawOperation)
	if !validMongoOperation(operation) {
		return errors.New("mongo leader conformance: invalid mutation operation")
	}
	key := mongoLeaderKey(opts)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts[key] == nil {
		c.counts[key] = make(map[leadertest.Operation]int64)
	}
	c.counts[key][operation]++
	err := c.failures[key][operation]
	delete(c.failures[key], operation)
	return err
}

func mongoLeaderKey(opts leader.Options) string {
	normalized, err := opts.Normalize()
	if err != nil {
		return ""
	}
	return normalized.KeyPrefix + ":" + normalized.Group
}

func validMongoOperation(operation leadertest.Operation) bool {
	switch operation {
	case leadertest.OperationCampaign, leadertest.OperationRenew, leadertest.OperationResign:
		return true
	default:
		return false
	}
}
