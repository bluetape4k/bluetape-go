package mongoleader

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/bluetape4k/bluetape-go/core"
	"github.com/bluetape4k/bluetape-go/leader"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultStrategicKeyPrefix = "bluetape:leader-strategy"

// StrategicElector is a MongoDB-backed candidate-registry leader elector.
type StrategicElector[T any] struct {
	collection *mongo.Collection
	opts       leader.Options
	cfg        config
}

var _ leader.StrategicElector[string] = (*StrategicElector[string])(nil)

type strategicCandidateDocument struct {
	ID              string            `bson:"_id"`
	GroupKey        string            `bson:"group_key"`
	Group           string            `bson:"group"`
	NodeID          string            `bson:"node_id"`
	RegisteredAt    time.Time         `bson:"registered_at"`
	LastStartedAt   time.Time         `bson:"last_started_at,omitempty"`
	LastCompletedAt time.Time         `bson:"last_completed_at,omitempty"`
	SuccessCount    int64             `bson:"success_count"`
	FailureCount    int64             `bson:"failure_count"`
	Weight          float64           `bson:"weight"`
	Metadata        map[string]string `bson:"metadata,omitempty"`
	LeaseUntil      time.Time         `bson:"lease_until"`
	CreatedAt       time.Time         `bson:"created_at"`
	UpdatedAt       time.Time         `bson:"updated_at"`
}

// NewStrategic creates a MongoDB-backed strategic leader elector.
func NewStrategic[T any](collection *mongo.Collection, opts leader.Options, optionFns ...Option) (*StrategicElector[T], error) {
	if err := requireCollection(collection); err != nil {
		return nil, err
	}
	if opts.KeyPrefix == "" {
		opts.KeyPrefix = defaultStrategicKeyPrefix
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return nil, err
	}
	cfg, err := normalizeConfig(optionFns)
	if err != nil {
		return nil, err
	}
	return &StrategicElector[T]{
		collection: collection,
		opts:       normalized,
		cfg:        cfg,
	}, nil
}

// RegisterCandidate registers or refreshes a candidate with a MongoDB lease.
func (e *StrategicElector[T]) RegisterCandidate(
	ctx context.Context,
	group string,
	info leader.CandidateInfo,
	ttl time.Duration,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateGroup(group); err != nil {
		return err
	}
	if err := core.RequireNotBlank("nodeID", info.NodeID); err != nil {
		return err
	}
	if err := core.RequirePositive("ttl", ttl); err != nil {
		return err
	}

	now := e.cfg.clock().UTC()
	info = normalizeCandidateInfo(info, now)
	doc := strategicCandidateDocument{
		ID:              e.candidateID(group, info.NodeID),
		GroupKey:        e.groupKey(group),
		Group:           group,
		NodeID:          info.NodeID,
		RegisteredAt:    info.RegisteredAt,
		LastStartedAt:   info.LastStartedAt,
		LastCompletedAt: info.LastCompletedAt,
		SuccessCount:    info.SuccessCount,
		FailureCount:    info.FailureCount,
		Weight:          info.Weight,
		Metadata:        cloneMetadata(info.Metadata),
		LeaseUntil:      now.Add(ttl),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	update := bson.M{
		"$set": bson.M{
			"group_key":         doc.GroupKey,
			"group":             doc.Group,
			"node_id":           doc.NodeID,
			"registered_at":     doc.RegisteredAt,
			"last_started_at":   doc.LastStartedAt,
			"last_completed_at": doc.LastCompletedAt,
			"success_count":     doc.SuccessCount,
			"failure_count":     doc.FailureCount,
			"weight":            doc.Weight,
			"metadata":          doc.Metadata,
			"lease_until":       doc.LeaseUntil,
			"updated_at":        doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id":        doc.ID,
			"created_at": doc.CreatedAt,
		},
	}
	_, err := e.collection.UpdateOne(
		ctx,
		bson.M{"_id": doc.ID},
		update,
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("mongo strategic candidate register: %w", err)
	}
	return nil
}

// UnregisterCandidate removes a candidate from the registry.
func (e *StrategicElector[T]) UnregisterCandidate(ctx context.Context, group string, nodeID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateGroup(group); err != nil {
		return err
	}
	if err := core.RequireNotBlank("nodeID", nodeID); err != nil {
		return err
	}

	if _, err := e.collection.DeleteOne(ctx, bson.M{"_id": e.candidateID(group, nodeID)}); err != nil {
		return fmt.Errorf("mongo strategic candidate unregister: %w", err)
	}
	return nil
}

// ListCandidates returns the current live candidates for a group.
func (e *StrategicElector[T]) ListCandidates(ctx context.Context, group string) ([]leader.CandidateInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateGroup(group); err != nil {
		return nil, err
	}
	now := e.cfg.clock().UTC()
	if err := e.pruneExpiredCandidates(ctx, group, now); err != nil {
		return nil, err
	}

	cursor, err := e.collection.Find(
		ctx,
		bson.M{"group_key": e.groupKey(group), "lease_until": bson.M{"$gt": now}},
		options.Find().SetSort(bson.D{{Key: "node_id", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("mongo strategic candidate list: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	candidates := make([]leader.CandidateInfo, 0)
	for cursor.Next(ctx) {
		var doc strategicCandidateDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo strategic candidate decode: %w", err)
		}
		candidates = append(candidates, doc.candidateInfo())
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("mongo strategic candidate cursor: %w", err)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].NodeID < candidates[j].NodeID
	})
	return candidates, nil
}

// UpdateResult records an action outcome for a live candidate.
func (e *StrategicElector[T]) UpdateResult(
	ctx context.Context,
	group string,
	nodeID string,
	result leader.CandidateResult,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateGroup(group); err != nil {
		return err
	}
	if err := core.RequireNotBlank("nodeID", nodeID); err != nil {
		return err
	}

	var increments bson.M
	switch result {
	case leader.CandidateSucceeded:
		increments = bson.M{"success_count": int64(1)}
	case leader.CandidateFailed:
		increments = bson.M{"failure_count": int64(1)}
	default:
		return fmt.Errorf("mongo strategic candidate result: unknown result %d", result)
	}

	now := e.cfg.clock().UTC()
	updated, err := e.collection.UpdateOne(
		ctx,
		bson.M{
			"_id":         e.candidateID(group, nodeID),
			"lease_until": bson.M{"$gt": now},
		},
		bson.M{
			"$inc": increments,
			"$set": bson.M{
				"last_completed_at": now,
				"updated_at":        now,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("mongo strategic candidate result store: %w", err)
	}
	if updated.MatchedCount == 0 {
		return leader.ErrNotLeader
	}
	return nil
}

// RunIfLeader runs action only when this elector's member is elected.
func (e *StrategicElector[T]) RunIfLeader(
	ctx context.Context,
	group string,
	strategy leader.ElectionStrategy,
	action func(context.Context) (T, error),
) (T, bool, error) {
	var zero T
	if strategy == nil {
		return zero, false, errors.New("strategy must not be nil")
	}
	if action == nil {
		return zero, false, errors.New("action must not be nil")
	}

	candidates, err := e.ListCandidates(ctx, group)
	if err != nil {
		return zero, false, err
	}
	winner, ok := strategy.Elect(candidates)
	if !ok || winner.NodeID != e.opts.MemberID {
		return zero, false, nil
	}

	result, actionErr := action(ctx)
	outcome := leader.CandidateSucceeded
	if actionErr != nil {
		outcome = leader.CandidateFailed
	}
	updateErr := e.UpdateResult(ctx, group, e.opts.MemberID, outcome)
	if actionErr != nil || updateErr != nil {
		return result, true, errors.Join(actionErr, updateErr)
	}
	return result, true, nil
}

func (e *StrategicElector[T]) pruneExpiredCandidates(ctx context.Context, group string, now time.Time) error {
	if _, err := e.collection.DeleteMany(ctx, bson.M{
		"group_key":   e.groupKey(group),
		"lease_until": bson.M{"$lte": now},
	}); err != nil {
		return fmt.Errorf("mongo strategic candidate prune: %w", err)
	}
	return nil
}

func (e *StrategicElector[T]) groupKey(group string) string {
	return fmt.Sprintf("%s:%s", e.opts.KeyPrefix, group)
}

func (e *StrategicElector[T]) candidateID(group string, nodeID string) string {
	return fmt.Sprintf("%s:candidate:%s", e.groupKey(group), nodeID)
}

func (doc strategicCandidateDocument) candidateInfo() leader.CandidateInfo {
	return leader.CandidateInfo{
		NodeID:          doc.NodeID,
		RegisteredAt:    doc.RegisteredAt.UTC(),
		LastStartedAt:   normalizeTime(doc.LastStartedAt),
		LastCompletedAt: normalizeTime(doc.LastCompletedAt),
		SuccessCount:    doc.SuccessCount,
		FailureCount:    doc.FailureCount,
		Weight:          doc.Weight,
		Metadata:        cloneMetadata(doc.Metadata),
	}
}

func validateGroup(group string) error {
	return core.RequireNotBlank("group", group)
}

func normalizeCandidateInfo(info leader.CandidateInfo, now time.Time) leader.CandidateInfo {
	if info.RegisteredAt.IsZero() {
		info.RegisteredAt = now
	} else {
		info.RegisteredAt = info.RegisteredAt.UTC()
	}
	info.LastStartedAt = normalizeTime(info.LastStartedAt)
	info.LastCompletedAt = normalizeTime(info.LastCompletedAt)
	info.Metadata = cloneMetadata(info.Metadata)
	return info
}

func normalizeTime(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return value.UTC()
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	copied := make(map[string]string, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}
