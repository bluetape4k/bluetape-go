package mongoleader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Elector is a MongoDB-backed single leader elector.
type Elector struct {
	collection *mongo.Collection
	opts       leader.Options
	cfg        config
	key        string
	token      string

	mu          sync.RWMutex
	owned       bool
	campaigning bool
	cleanup     bool
	cancel      context.CancelFunc
	done        chan struct{}
	testHook    func(string) error
}

var _ leader.Elector = (*Elector)(nil)

type leaseDocument struct {
	ID         string    `bson:"_id"`
	Group      string    `bson:"group"`
	MemberID   string    `bson:"member_id"`
	Token      string    `bson:"token"`
	LeaseUntil time.Time `bson:"lease_until"`
	CreatedAt  time.Time `bson:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

// New creates a MongoDB-backed single leader elector.
func New(collection *mongo.Collection, opts leader.Options, optionFns ...Option) (*Elector, error) {
	if err := requireCollection(collection); err != nil {
		return nil, err
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return nil, err
	}
	cfg, err := normalizeConfig(optionFns)
	if err != nil {
		return nil, err
	}
	random, err := randomToken()
	if err != nil {
		return nil, err
	}
	return &Elector{
		collection: collection,
		opts:       normalized,
		cfg:        cfg,
		key:        normalized.KeyPrefix + ":" + normalized.Group,
		token:      normalized.MemberID + ":" + random,
	}, nil
}

// Campaign loops until this elector acquires leadership or ctx is canceled.
func (e *Elector) Campaign(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.beginCampaign(); err != nil {
		return err
	}
	defer e.endCampaign()

	for {
		acquired, err := e.tryAcquire(ctx)
		if err != nil {
			if acquired {
				e.mu.Lock()
				e.cleanup = true
				e.mu.Unlock()
				return errors.Join(
					leader.NewOperationError("mongo", "campaign", err),
					leader.ErrCommitUnknown,
				)
			}
			if errors.Is(err, leader.ErrCommitUnknown) {
				e.mu.Lock()
				e.cleanup = true
				e.mu.Unlock()
			}
			return err
		}
		if acquired {
			e.startRenewal()
			return nil
		}
		if err := sleepContext(ctx, e.cfg.retryDelay); err != nil {
			return err
		}
	}
}

// Resign releases leadership only when this elector still owns it.
func (e *Elector) Resign(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cancel, done, active := e.clearOwnership()
	if !active {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if _, err := e.collection.DeleteOne(ctx, bson.M{"_id": e.key, "token": e.token}); err != nil {
		return errors.Join(
			leader.NewOperationError("mongo", "resign", err),
			leader.ErrCommitUnknown,
		)
	}
	if err := e.afterMutation("resign"); err != nil {
		return errors.Join(
			leader.NewOperationError("mongo", "resign", err),
			leader.ErrCommitUnknown,
		)
	}
	e.mu.Lock()
	e.cleanup = false
	e.mu.Unlock()
	return nil
}

// IsLeader reports whether this elector still believes it owns leadership.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

// Leader returns the active leader token recorded in MongoDB.
func (e *Elector) Leader(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	now := e.cfg.clock().UTC()
	var doc leaseDocument
	err := e.collection.FindOne(ctx, bson.M{"_id": e.key, "lease_until": bson.M{"$gt": now}}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", nil
		}
		return "", leader.NewOperationError("mongo", "lookup", err)
	}
	return doc.Token, nil
}

func (e *Elector) beginCampaign() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleanup {
		return leader.ErrCleanupPending
	}
	if e.owned {
		return leader.ErrAlreadyLeader
	}
	if e.campaigning {
		return leader.ErrCampaignInProgress
	}
	e.campaigning = true
	return nil
}

func (e *Elector) endCampaign() {
	e.mu.Lock()
	e.campaigning = false
	e.mu.Unlock()
}

func (e *Elector) tryAcquire(ctx context.Context) (bool, error) {
	now := e.cfg.clock().UTC()
	doc := leaseDocument{
		ID:         e.key,
		Group:      e.opts.Group,
		MemberID:   e.opts.MemberID,
		Token:      e.token,
		LeaseUntil: now.Add(e.opts.Lease),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	filter := bson.M{
		"_id": e.key,
		"$or": bson.A{
			bson.M{"lease_until": bson.M{"$lte": now}},
			bson.M{"token": e.token},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"group":       doc.Group,
			"member_id":   doc.MemberID,
			"token":       doc.Token,
			"lease_until": doc.LeaseUntil,
			"updated_at":  doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id":        doc.ID,
			"created_at": doc.CreatedAt,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var updated leaseDocument
	err := e.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			_ = e.afterMutation("campaign")
			return false, nil
		}
		return false, errors.Join(
			leader.NewOperationError("mongo", "campaign", err),
			leader.ErrCommitUnknown,
		)
	}
	acquired := updated.Token == e.token && updated.LeaseUntil.After(now)
	if acquired {
		if err := e.afterMutation("campaign"); err != nil {
			return true, err
		}
	}
	return acquired, nil
}

func (e *Elector) startRenewal() {
	renewCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	e.mu.Lock()
	e.owned = true
	e.cleanup = false
	e.cancel = cancel
	e.done = done
	e.mu.Unlock()

	go e.renewLoop(renewCtx, done)
}

func (e *Elector) renewLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(e.opts.RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, err := e.renew(ctx)
			if err != nil || !ok {
				e.clearOwnershipAfterLoss(err != nil)
				return
			}
		}
	}
}

func (e *Elector) renew(ctx context.Context) (bool, error) {
	renewCtx, cancel := context.WithTimeout(ctx, e.opts.RenewInterval)
	defer cancel()

	now := e.cfg.clock().UTC()
	result, err := e.collection.UpdateOne(
		renewCtx,
		bson.M{"_id": e.key, "token": e.token, "lease_until": bson.M{"$gt": now}},
		bson.M{"$set": bson.M{"lease_until": now.Add(e.opts.Lease), "updated_at": now}},
	)
	if err != nil {
		return false, leader.NewOperationError("mongo", "renew", err)
	}
	matched := result.MatchedCount == 1
	if matched {
		if err := e.afterMutation("renew"); err != nil {
			return true, err
		}
	}
	return matched, nil
}

func (e *Elector) clearOwnership() (context.CancelFunc, chan struct{}, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	active := e.owned || e.cleanup
	cancel := e.cancel
	done := e.done
	e.owned = false
	e.cleanup = true
	e.cancel = nil
	e.done = nil
	return cancel, done, active
}

func (e *Elector) clearOwnershipAfterLoss(cleanup bool) {
	e.mu.Lock()
	e.owned = false
	e.cleanup = cleanup
	e.cancel = nil
	e.done = nil
	e.mu.Unlock()
}

func (e *Elector) afterMutation(operation string) error {
	if e.testHook == nil {
		return nil
	}
	return e.testHook(operation)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func randomToken() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
