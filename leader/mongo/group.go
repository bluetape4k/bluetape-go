package mongoleader

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// GroupElector는 leader backend election에서 caller-visible 상태와 의미를 설명한다.
type GroupElector struct {
	collection *mongo.Collection
	opts       leader.GroupOptions
	cfg        config
	groupKey   string
	token      string

	mu          sync.RWMutex
	owned       bool
	campaigning bool
	slot        int
	cancel      context.CancelFunc
	done        chan struct{}
}

var _ leader.GroupElector = (*GroupElector)(nil)

type groupLeaseDocument struct {
	ID         string    `bson:"_id"`
	GroupKey   string    `bson:"group_key"`
	Group      string    `bson:"group"`
	Slot       int       `bson:"slot"`
	MemberID   string    `bson:"member_id"`
	Token      string    `bson:"token"`
	LeaseUntil time.Time `bson:"lease_until"`
	CreatedAt  time.Time `bson:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

// NewGroup는 leader backend election에서 생성과 초기화 계약을 설명한다.
func NewGroup(collection *mongo.Collection, opts leader.GroupOptions, optionFns ...Option) (*GroupElector, error) {
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
	return &GroupElector{
		collection: collection,
		opts:       normalized,
		cfg:        cfg,
		groupKey:   normalized.KeyPrefix + ":" + normalized.Group,
		token:      normalized.MemberID + ":" + random,
		slot:       -1,
	}, nil
}

// Campaign는 leader backend election에서 caller-visible 상태와 의미를 설명한다.
func (e *GroupElector) Campaign(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := e.beginCampaign(); err != nil {
		return err
	}
	defer e.endCampaign()

	for {
		acquired, slot, err := e.tryAcquireSlot(ctx)
		if err != nil {
			return err
		}
		if acquired {
			e.startRenewal(slot)
			return nil
		}
		if err := sleepContext(ctx, e.cfg.retryDelay); err != nil {
			return err
		}
	}
}

// Resign는 leader backend election에서 실행, cancellation, cleanup 계약을 설명한다.
func (e *GroupElector) Resign(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cancel, done, slot, owned := e.clearOwnership()
	if !owned {
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
	if _, err := e.collection.DeleteOne(ctx, bson.M{"_id": e.slotID(slot), "token": e.token}); err != nil {
		return fmt.Errorf("mongo leader group resign: %w", err)
	}
	return nil
}

// IsLeader는 leader backend election에서 반환값과 오류 의미를 설명한다.
func (e *GroupElector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

// ActiveCount는 leader backend election에서 반환값과 오류 의미를 설명한다.
func (e *GroupElector) ActiveCount(ctx context.Context) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := e.cfg.clock().UTC()
	count, err := e.collection.CountDocuments(ctx, bson.M{
		"group_key":   e.groupKey,
		"lease_until": bson.M{"$gt": now},
	})
	if err != nil {
		return 0, fmt.Errorf("mongo leader group active count: %w", err)
	}
	return int(count), nil
}

// AvailableSlots는 leader backend election에서 반환값과 오류 의미를 설명한다.
func (e *GroupElector) AvailableSlots(ctx context.Context) (int, error) {
	active, err := e.ActiveCount(ctx)
	if err != nil {
		return 0, fmt.Errorf("mongo leader group available slots: %w", err)
	}
	available := e.opts.MaxLeaders - active
	if available < 0 {
		return 0, nil
	}
	return available, nil
}

func (e *GroupElector) beginCampaign() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.owned || e.campaigning {
		return leader.ErrAlreadyLeader
	}
	e.campaigning = true
	return nil
}

func (e *GroupElector) endCampaign() {
	e.mu.Lock()
	e.campaigning = false
	e.mu.Unlock()
}

func (e *GroupElector) tryAcquireSlot(ctx context.Context) (bool, int, error) {
	start := e.slotStart()
	for offset := 0; offset < e.opts.MaxLeaders; offset++ {
		slot := (start + offset) % e.opts.MaxLeaders
		acquired, err := e.tryAcquireSpecificSlot(ctx, slot)
		if err != nil {
			return false, -1, err
		}
		if acquired {
			return true, slot, nil
		}
	}
	return false, -1, nil
}

func (e *GroupElector) tryAcquireSpecificSlot(ctx context.Context, slot int) (bool, error) {
	now := e.cfg.clock().UTC()
	doc := groupLeaseDocument{
		ID:         e.slotID(slot),
		GroupKey:   e.groupKey,
		Group:      e.opts.Group,
		Slot:       slot,
		MemberID:   e.opts.MemberID,
		Token:      e.token,
		LeaseUntil: now.Add(e.opts.Lease),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	filter := bson.M{
		"_id": doc.ID,
		"$or": bson.A{
			bson.M{"lease_until": bson.M{"$lte": now}},
			bson.M{"token": e.token},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"group_key":   doc.GroupKey,
			"group":       doc.Group,
			"slot":        doc.Slot,
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

	var updated groupLeaseDocument
	err := e.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) || errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, fmt.Errorf("mongo leader group campaign: %w", err)
	}
	return updated.Token == e.token && updated.LeaseUntil.After(now), nil
}

func (e *GroupElector) startRenewal(slot int) {
	renewCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	e.mu.Lock()
	e.owned = true
	e.slot = slot
	e.cancel = cancel
	e.done = done
	e.mu.Unlock()

	go e.renewLoop(renewCtx, done)
}

func (e *GroupElector) renewLoop(ctx context.Context, done chan<- struct{}) {
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
				e.clearOwnershipAfterLoss()
				return
			}
		}
	}
}

func (e *GroupElector) renew(ctx context.Context) (bool, error) {
	renewCtx, cancel := context.WithTimeout(ctx, e.opts.RenewInterval)
	defer cancel()

	e.mu.RLock()
	slot := e.slot
	e.mu.RUnlock()
	if slot < 0 {
		return false, nil
	}

	now := e.cfg.clock().UTC()
	result, err := e.collection.UpdateOne(
		renewCtx,
		bson.M{"_id": e.slotID(slot), "token": e.token, "lease_until": bson.M{"$gt": now}},
		bson.M{"$set": bson.M{"lease_until": now.Add(e.opts.Lease), "updated_at": now}},
	)
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}

func (e *GroupElector) clearOwnership() (context.CancelFunc, chan struct{}, int, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	owned := e.owned
	cancel := e.cancel
	done := e.done
	slot := e.slot
	e.owned = false
	e.slot = -1
	e.cancel = nil
	e.done = nil
	return cancel, done, slot, owned
}

func (e *GroupElector) clearOwnershipAfterLoss() {
	e.mu.Lock()
	e.owned = false
	e.slot = -1
	e.cancel = nil
	e.done = nil
	e.mu.Unlock()
}

func (e *GroupElector) slotStart() int {
	var sum int
	for _, b := range []byte(e.token) {
		sum += int(b)
	}
	return sum % e.opts.MaxLeaders
}

func (e *GroupElector) slotID(slot int) string {
	return fmt.Sprintf("%s:slot:%06d", e.groupKey, slot)
}
