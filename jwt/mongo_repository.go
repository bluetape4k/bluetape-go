package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	mongoDocumentKindCurrent = "current"
	mongoDocumentKindKey     = "key"
	mongoCursorCloseTimeout  = 5 * time.Second
)

// MongoRepository stores distributed JWT KeyChains in MongoDB.
type MongoRepository struct {
	collection *mongo.Collection
	opts       mongoRepositoryOptions
}

var _ DistributedKeyChainRepository = (*MongoRepository)(nil)

type mongoKeyDocument struct {
	ID        string    `bson:"_id"`
	Kind      string    `bson:"kind"`
	Namespace string    `bson:"namespace"`
	KID       string    `bson:"kid"`
	Payload   []byte    `bson:"payload"`
	CreatedAt time.Time `bson:"created_at"`
	ExpiresAt time.Time `bson:"expires_at"`
}

type mongoCurrentDocument struct {
	ID        string `bson:"_id"`
	Kind      string `bson:"kind"`
	Namespace string `bson:"namespace"`
	KID       string `bson:"kid"`
}

// NewMongoRepository creates a MongoDB-backed distributed KeyChain repository.
func NewMongoRepository(options MongoRepositoryOptions) (*MongoRepository, error) {
	normalized, err := options.normalize()
	if err != nil {
		return nil, err
	}
	collection := normalized.client.Database(normalized.database).Collection(normalized.collection)
	return &MongoRepository{collection: collection, opts: normalized}, nil
}

// Current returns the current non-expired KeyChain.
func (r *MongoRepository) Current(ctx context.Context, now time.Time) (*KeyChain, error) {
	if err := r.validateReady(ctx); err != nil {
		return nil, err
	}
	kid, err := r.currentKID(ctx)
	if err != nil {
		return nil, err
	}
	return r.findPayload(ctx, kid, now)
}

// Find returns a non-expired KeyChain by kid.
func (r *MongoRepository) Find(ctx context.Context, kid string, now time.Time) (*KeyChain, error) {
	if err := r.validateReady(ctx); err != nil {
		return nil, err
	}
	if err := validateLookupKID(kid); err != nil {
		return nil, err
	}
	return r.findPayload(ctx, kid, now)
}

// Rotate returns the current key or stores a new one when no live key exists.
func (r *MongoRepository) Rotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error) {
	if err := r.validateReady(ctx); err != nil {
		return nil, err
	}
	if create == nil {
		return nil, OptionError{Option: "create", Err: errorsNew("must not be nil")}
	}
	current, observedKID, err := r.currentPayload(ctx, now)
	if err == nil {
		return current, nil
	}
	if !errors.Is(err, ErrKeyNotFound) && !errors.Is(err, ErrInvalidKey) {
		return nil, err
	}
	key, err := createWithContext(ctx, create)()
	if err != nil {
		return nil, err
	}
	return r.storeCAS(ctx, observedKID, key, now)
}

// ForcedRotate always stores a newly created KeyChain.
func (r *MongoRepository) ForcedRotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error) {
	if err := r.validateReady(ctx); err != nil {
		return nil, err
	}
	if create == nil {
		return nil, OptionError{Option: "create", Err: errorsNew("must not be nil")}
	}
	key, err := createWithContext(ctx, create)()
	if err != nil {
		return nil, err
	}
	return r.store(ctx, key, now)
}

// DeleteAll removes all MongoDB state for this repository namespace.
func (r *MongoRepository) DeleteAll(ctx context.Context) error {
	if err := r.validateReady(ctx); err != nil {
		return err
	}
	if _, err := r.collection.DeleteMany(ctx, bson.M{"namespace": r.opts.namespace, "kind": bson.M{"$in": bson.A{mongoDocumentKindCurrent, mongoDocumentKindKey}}}); err != nil {
		return fmt.Errorf("mongo jwt delete all: %w", err)
	}
	return nil
}

func (r *MongoRepository) validateReady(ctx context.Context) error {
	if r == nil || r.collection == nil {
		return OptionError{Option: "repository", Err: errorsNew("must be constructed by a constructor")}
	}
	return requireContext(ctx)
}

func (r *MongoRepository) findPayload(ctx context.Context, kid string, now time.Time) (*KeyChain, error) {
	var doc mongoKeyDocument
	err := r.collection.FindOne(ctx, r.keyFilter(kid)).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, KeyError{Kind: ErrKeyNotFound, KID: kid, Err: errorsNew("key not found")}
		}
		return nil, fmt.Errorf("mongo jwt find key: %w", err)
	}
	key, err := decodeRedisKeyChain(doc.Payload, r.opts.maxKeyBytes)
	if err != nil {
		return nil, err
	}
	if key.Expired(now) {
		return nil, KeyError{Kind: ErrInvalidKey, KID: kid, Err: errorsNew("key expired")}
	}
	return key, nil
}

func (r *MongoRepository) currentPayload(ctx context.Context, now time.Time) (*KeyChain, string, error) {
	kid, err := r.currentKID(ctx)
	if err != nil {
		return nil, "", err
	}
	key, err := r.findPayload(ctx, kid, now)
	if err != nil {
		return nil, kid, err
	}
	return key, kid, nil
}

func (r *MongoRepository) currentKID(ctx context.Context) (string, error) {
	var doc mongoCurrentDocument
	err := r.collection.FindOne(ctx, r.currentFilter()).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", KeyError{Kind: ErrKeyNotFound, Err: errorsNew("current key not found")}
		}
		return "", fmt.Errorf("mongo jwt current: %w", err)
	}
	if doc.KID == "" {
		return "", KeyError{Kind: ErrKeyNotFound, Err: errorsNew("current key not found")}
	}
	return doc.KID, nil
}

func (r *MongoRepository) storeCAS(ctx context.Context, observedKID string, key *KeyChain, now time.Time) (*KeyChain, error) {
	doc, err := r.prepareStoreDocument(ctx, key, now)
	if err != nil {
		return nil, err
	}
	if err := r.upsertKey(ctx, doc); err != nil {
		return nil, err
	}
	won, err := r.compareAndSetCurrent(ctx, observedKID, key.KID())
	if err != nil {
		return nil, err
	}
	if !won {
		_ = r.deleteKey(ctx, key.KID())
		return r.currentAfterCAS(ctx, now)
	}
	if err := r.trim(ctx, key.KID()); err != nil {
		return nil, err
	}
	return r.findPayload(ctx, key.KID(), now)
}

func (r *MongoRepository) store(ctx context.Context, key *KeyChain, now time.Time) (*KeyChain, error) {
	doc, err := r.prepareStoreDocument(ctx, key, now)
	if err != nil {
		return nil, err
	}
	if err := r.upsertKey(ctx, doc); err != nil {
		return nil, err
	}
	if err := r.setCurrent(ctx, key.KID()); err != nil {
		return nil, err
	}
	if err := r.trim(ctx, key.KID()); err != nil {
		return nil, err
	}
	return r.findPayload(ctx, key.KID(), now)
}

func (r *MongoRepository) currentAfterCAS(ctx context.Context, now time.Time) (*KeyChain, error) {
	var lastErr error
	for range 10 {
		key, err := r.Current(ctx, now)
		if err == nil {
			return key, nil
		}
		if !errors.Is(err, ErrKeyNotFound) {
			return nil, err
		}
		lastErr = err
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func (r *MongoRepository) prepareStoreDocument(ctx context.Context, key *KeyChain, now time.Time) (mongoKeyDocument, error) {
	if err := requireContext(ctx); err != nil {
		return mongoKeyDocument{}, err
	}
	if key == nil {
		return mongoKeyDocument{}, KeyError{Kind: ErrInvalidKey, Err: errorsNew("key must not be nil")}
	}
	if key.Expired(now) {
		return mongoKeyDocument{}, KeyError{Kind: ErrInvalidKey, KID: key.KID(), Err: errorsNew("key expired")}
	}
	payload, err := encodeRedisKeyChain(key)
	if err != nil {
		return mongoKeyDocument{}, err
	}
	if len(payload) > r.opts.maxKeyBytes {
		return mongoKeyDocument{}, KeyError{Kind: ErrInvalidKey, KID: key.KID(), Err: errorsNew("mongo key payload exceeds max key bytes")}
	}
	if err := requireContext(ctx); err != nil {
		return mongoKeyDocument{}, err
	}
	return mongoKeyDocument{
		ID:        r.keyID(key.KID()),
		Kind:      mongoDocumentKindKey,
		Namespace: r.opts.namespace,
		KID:       key.KID(),
		Payload:   payload,
		CreatedAt: key.CreatedAt(),
		ExpiresAt: key.ExpiresAt(),
	}, nil
}

func (r *MongoRepository) compareAndSetCurrent(ctx context.Context, observedKID string, kid string) (bool, error) {
	if observedKID == "" {
		result, err := r.collection.UpdateOne(
			ctx,
			r.currentFilter(),
			bson.M{"$setOnInsert": r.currentDocument(kid)},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return false, nil
			}
			return false, fmt.Errorf("mongo jwt rotate current: %w", err)
		}
		return result.UpsertedCount == 1, nil
	}
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": r.currentID(), "namespace": r.opts.namespace, "kind": mongoDocumentKindCurrent, "kid": observedKID},
		bson.M{"$set": r.currentDocument(kid)},
	)
	if err != nil {
		return false, fmt.Errorf("mongo jwt rotate current: %w", err)
	}
	return result.MatchedCount == 1, nil
}

func (r *MongoRepository) upsertKey(ctx context.Context, doc mongoKeyDocument) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": doc.ID},
		bson.M{"$set": doc},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("mongo jwt store key: %w", err)
	}
	return nil
}

func (r *MongoRepository) deleteKey(ctx context.Context, kid string) error {
	_, err := r.collection.DeleteOne(ctx, r.keyFilter(kid))
	if err != nil {
		return fmt.Errorf("mongo jwt delete key: %w", err)
	}
	return nil
}

func (r *MongoRepository) setCurrent(ctx context.Context, kid string) error {
	if err := validateLookupKID(kid); err != nil {
		return err
	}
	_, err := r.collection.UpdateOne(
		ctx,
		r.currentFilter(),
		bson.M{"$set": r.currentDocument(kid)},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("mongo jwt set current: %w", err)
	}
	return nil
}

func (r *MongoRepository) trim(ctx context.Context, keepKID string) error {
	cursor, err := r.collection.Find(
		ctx,
		r.namespaceFilter(),
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "kid", Value: 1}}),
	)
	if err != nil {
		return fmt.Errorf("mongo jwt trim list: %w", err)
	}
	defer func() {
		closeCtx, cancel := mongoCleanupContext(ctx)
		defer cancel()
		_ = cursor.Close(closeCtx)
	}()

	retainedOthers := 0
	otherCapacity := r.opts.capacity
	if keepKID != "" {
		otherCapacity--
	}
	removeIDs := make([]string, 0)
	for cursor.Next(ctx) {
		var doc mongoKeyDocument
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("mongo jwt trim decode: %w", err)
		}
		if doc.KID == keepKID {
			continue
		}
		if retainedOthers < otherCapacity {
			retainedOthers++
			continue
		}
		removeIDs = append(removeIDs, doc.ID)
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("mongo jwt trim cursor: %w", err)
	}
	if len(removeIDs) == 0 {
		return nil
	}
	if _, err := r.collection.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": removeIDs}}); err != nil {
		return fmt.Errorf("mongo jwt trim delete: %w", err)
	}
	return nil
}

func mongoCleanupContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(parent), mongoCursorCloseTimeout)
}

func (r *MongoRepository) keysCollection() *mongo.Collection {
	return r.collection
}

func (r *MongoRepository) namespaceFilter() bson.M {
	return bson.M{"namespace": r.opts.namespace, "kind": mongoDocumentKindKey}
}

func (r *MongoRepository) keyFilter(kid string) bson.M {
	return bson.M{"_id": r.keyID(kid), "namespace": r.opts.namespace, "kind": mongoDocumentKindKey}
}

func (r *MongoRepository) currentFilter() bson.M {
	return bson.M{"_id": r.currentID(), "namespace": r.opts.namespace, "kind": mongoDocumentKindCurrent}
}

func (r *MongoRepository) currentDocument(kid string) mongoCurrentDocument {
	return mongoCurrentDocument{
		ID:        r.currentID(),
		Kind:      mongoDocumentKindCurrent,
		Namespace: r.opts.namespace,
		KID:       kid,
	}
}

func (r *MongoRepository) currentID() string {
	return r.opts.namespace + ":current"
}

func (r *MongoRepository) keyID(kid string) string {
	return r.opts.namespace + ":key:" + kid
}
