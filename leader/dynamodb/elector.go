package dynamodbleader

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bluetape4k/bluetape-go/leader"
)

var (
	errMalformedResponse = errors.New("dynamodb leader: malformed response")
)

// Elector DynamoDB 단일 item lease를 사용하는 leader.Elector 구현이다.
type Elector struct {
	client    Client
	tableName string
	opts      leader.Options
	cfg       config
	token     string

	mu          sync.RWMutex
	owned       bool
	campaigning bool
	cleanup     bool
	resigning   int
	resolved    bool
	generation  uint64
	cancel      context.CancelFunc
	done        chan struct{}
}

var _ leader.Elector = (*Elector)(nil)

// Campaign lease를 획득하거나 caller context가 끝날 때까지 조건부 쓰기를 반복한다.
func (e *Elector) Campaign(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := e.beginCampaign(); err != nil {
		return err
	}
	defer e.endCampaign()

	for {
		acquired, err := e.acquireAttempt(ctx)
		if err != nil {
			e.log(ctx, "dynamodb leader campaign failed", slogLevelError, "campaign")
			return err
		}
		if acquired {
			e.startRenewal()
			if err := ctx.Err(); err != nil {
				e.markCleanupPending()
				return errors.Join(err, leader.ErrCommitUnknown)
			}
			e.log(ctx, "dynamodb leader campaign acquired", slogLevelInfo, "campaign")
			return nil
		}
		if err := sleepContext(ctx, e.cfg.retryDelay); err != nil {
			return err
		}
	}
}

// Resign 이 elector의 owner token만 조건부로 삭제하고 renewal을 종료한다.
func (e *Elector) Resign(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	generation, cancel, done, active := e.clearOwnership()
	if !active {
		return nil
	}
	resolved := false
	defer func() { e.finishResign(generation, resolved) }()
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
	if err := ctx.Err(); err != nil {
		return err
	}

	output, err := e.client.DeleteItem(ctx, e.deleteInput())
	if err == nil && output == nil {
		err = errMalformedResponse
	}
	if err == nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			resolved = true
			return ctxErr
		}
		resolved = true
		e.log(ctx, "dynamodb leader resigned", slogLevelInfo, "resign")
		return nil
	}
	if isConditionalFailure(err) {
		resolved = true
		return nil
	}
	operationErr := providerError("resign", err, false)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(operationErr, ctxErr, leader.ErrCommitUnknown)
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, e.attemptBudget())
	defer probeCancel()
	confirmed, probeErr := e.probeOwnership(probeCtx)
	if probeErr == nil && !confirmed {
		resolved = true
		return nil
	}
	e.markCleanupPending()
	if probeErr != nil {
		return errors.Join(operationErr, probeErr, leader.ErrCommitUnknown)
	}
	return errors.Join(operationErr, leader.ErrCommitUnknown)
}

// IsLeader local ownership state를 반환하며 DynamoDB read를 수행하지 않는다.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

// Leader strongly consistent read로 현재 active owner token을 조회한다.
func (e *Elector) Leader(ctx context.Context) (string, error) {
	if err := validateContext(ctx); err != nil {
		return "", err
	}
	state, err := e.lookup(ctx)
	if err != nil {
		return "", err
	}
	if !state.active {
		return "", nil
	}
	return state.owner, nil
}

type ownerState struct {
	owner  string
	lease  int64
	active bool
}

func (e *Elector) acquireAttempt(ctx context.Context) (bool, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, e.attemptBudget())
	defer cancel()
	deadline := e.deadline()
	output, err := e.client.PutItem(attemptCtx, e.putInput(deadline))
	if err == nil && output == nil {
		err = errMalformedResponse
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		e.markCleanupPending()
		return false, errors.Join(ctxErr, leader.ErrCommitUnknown)
	}
	if attemptErr := attemptCtx.Err(); attemptErr != nil {
		if err != nil {
			attemptErr = errors.Join(err, attemptErr)
		}
		return e.reconcileAfterTimeout(ctx, "campaign", attemptErr)
	}
	if err == nil {
		return true, nil
	}
	if isConditionalFailure(err) {
		return e.takeover(ctx)
	}
	return e.reconcileAfterError(ctx, "campaign", err)
}

func (e *Elector) takeover(ctx context.Context) (bool, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, e.attemptBudget())
	defer cancel()
	output, err := e.client.UpdateItem(attemptCtx, e.takeoverInput(e.deadline()))
	if err == nil && output == nil {
		err = errMalformedResponse
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		e.markCleanupPending()
		return false, errors.Join(ctxErr, leader.ErrCommitUnknown)
	}
	if attemptErr := attemptCtx.Err(); attemptErr != nil {
		if err != nil {
			attemptErr = errors.Join(err, attemptErr)
		}
		return e.reconcileAfterTimeout(ctx, "campaign", attemptErr)
	}
	if err == nil {
		return true, nil
	}
	if isConditionalFailure(err) {
		return false, nil
	}
	return e.reconcileAfterError(ctx, "campaign", err)
}

func (e *Elector) reconcileAfterError(ctx context.Context, operation string, cause error) (bool, error) {
	return e.reconcile(ctx, operation, cause, false)
}

func (e *Elector) reconcileAfterTimeout(ctx context.Context, operation string, cause error) (bool, error) {
	return e.reconcile(ctx, operation, cause, true)
}

func (e *Elector) reconcile(ctx context.Context, operation string, cause error, retryOnNoOwner bool) (bool, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		e.markCleanupPending()
		return false, errors.Join(providerError(operation, cause, true), ctxErr)
	}
	probeCtx, cancel := context.WithTimeout(ctx, e.attemptBudget())
	defer cancel()
	confirmed, probeErr := e.probeOwnership(probeCtx)
	if probeErr == nil && confirmed {
		return true, nil
	}
	if probeErr == nil && retryOnNoOwner {
		return false, nil
	}
	if probeErr != nil {
		e.markCleanupPending()
		return false, errors.Join(providerError(operation, cause, true), probeErr)
	}
	return false, providerError(operation, cause, false)
}

func (e *Elector) renewLoop(ctx context.Context, generation uint64, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(e.opts.RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attemptCtx, cancel := context.WithTimeout(ctx, e.renewalBudget())
			ok, err := e.renew(attemptCtx)
			cancel()
			if ctx.Err() != nil {
				return
			}
			if err == nil && ok {
				continue
			}
			if err != nil {
				probeCtx, probeCancel := context.WithTimeout(ctx, e.attemptBudget())
				confirmed, probeErr := e.probeOwnership(probeCtx)
				probeCancel()
				if probeErr == nil && confirmed {
					e.log(ctx, "dynamodb leader renewal response reconciled", slogLevelWarn, "renew")
					continue
				}
				e.clearOwnershipAfterLoss(generation, done, probeErr != nil)
				return
			}
			e.clearOwnershipAfterLoss(generation, done, false)
			return
		}
	}
}

func (e *Elector) renew(ctx context.Context) (bool, error) {
	deadline := e.deadline()
	output, err := e.client.UpdateItem(ctx, e.renewInput(deadline))
	if err == nil && output == nil {
		err = errMalformedResponse
	}
	if err != nil {
		if isConditionalFailure(err) {
			return false, nil
		}
		return false, providerError("renew", err, false)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false, errors.Join(ctxErr, leader.ErrCommitUnknown)
	}
	return true, nil
}

func (e *Elector) lookup(ctx context.Context) (ownerState, error) {
	output, err := e.client.GetItem(ctx, e.getInput())
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ownerState{}, errors.Join(providerError("lookup", err, false), ctxErr)
		}
		return ownerState{}, providerError("lookup", err, false)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ownerState{}, ctxErr
	}
	if output == nil {
		return ownerState{}, ErrMalformedItem
	}
	if len(output.Item) == 0 {
		return ownerState{}, nil
	}
	leaseValue, ok := output.Item[e.cfg.leaseAttribute]
	leaseText, leaseOK := numberValue(leaseValue)
	if !ok || !leaseOK || leaseText <= 0 {
		return ownerState{}, ErrMalformedItem
	}
	state := ownerState{lease: leaseText, active: leaseText > e.now().UnixMilli()}
	if !state.active {
		return state, nil
	}
	ownerValue, ownerOK := stringValue(output.Item[e.cfg.ownerAttribute])
	if !ownerOK || ownerValue == "" {
		return ownerState{}, ErrMalformedItem
	}
	state.owner = ownerValue
	return state, nil
}

func (e *Elector) probeOwnership(ctx context.Context) (bool, error) {
	state, err := e.lookup(ctx)
	if err != nil {
		return false, err
	}
	return state.active && state.owner == e.token, nil
}

func (e *Elector) putInput(deadline time.Time) *dynamodb.PutItemInput {
	return &dynamodb.PutItemInput{
		TableName:                aws.String(e.tableName),
		Item:                     e.item(deadline),
		ConditionExpression:      aws.String("attribute_not_exists(#key)"),
		ExpressionAttributeNames: map[string]string{"#key": e.cfg.keyAttribute},
	}
}

func (e *Elector) takeoverInput(deadline time.Time) *dynamodb.UpdateItemInput {
	return &dynamodb.UpdateItemInput{
		TableName:           aws.String(e.tableName),
		Key:                 e.key(),
		ConditionExpression: aws.String("attribute_not_exists(#lease) OR #lease <= :now"),
		ExpressionAttributeNames: map[string]string{
			"#owner": e.cfg.ownerAttribute,
			"#lease": e.cfg.leaseAttribute, "#ttl": e.cfg.ttlAttribute,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":   &types.AttributeValueMemberN{Value: strconv.FormatInt(e.now().UnixMilli(), 10)},
			":owner": &types.AttributeValueMemberS{Value: e.token},
			":lease": &types.AttributeValueMemberN{Value: strconv.FormatInt(deadline.UnixMilli(), 10)},
			":ttl":   &types.AttributeValueMemberN{Value: strconv.FormatInt(ttlSeconds(deadline), 10)},
		},
		UpdateExpression: aws.String("SET #owner = :owner, #lease = :lease, #ttl = :ttl"),
	}
}

func (e *Elector) renewInput(deadline time.Time) *dynamodb.UpdateItemInput {
	return &dynamodb.UpdateItemInput{
		TableName:           aws.String(e.tableName),
		Key:                 e.key(),
		ConditionExpression: aws.String("#owner = :owner AND #lease > :now"),
		ExpressionAttributeNames: map[string]string{
			"#owner": e.cfg.ownerAttribute,
			"#lease": e.cfg.leaseAttribute, "#ttl": e.cfg.ttlAttribute,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":   &types.AttributeValueMemberN{Value: strconv.FormatInt(e.now().UnixMilli(), 10)},
			":owner": &types.AttributeValueMemberS{Value: e.token},
			":lease": &types.AttributeValueMemberN{Value: strconv.FormatInt(deadline.UnixMilli(), 10)},
			":ttl":   &types.AttributeValueMemberN{Value: strconv.FormatInt(ttlSeconds(deadline), 10)},
		},
		UpdateExpression: aws.String("SET #lease = :lease, #ttl = :ttl"),
	}
}

func (e *Elector) deleteInput() *dynamodb.DeleteItemInput {
	return &dynamodb.DeleteItemInput{
		TableName:           aws.String(e.tableName),
		Key:                 e.key(),
		ConditionExpression: aws.String("#owner = :owner"),
		ExpressionAttributeNames: map[string]string{
			"#owner": e.cfg.ownerAttribute,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":owner": &types.AttributeValueMemberS{Value: e.token},
		},
	}
}

func (e *Elector) getInput() *dynamodb.GetItemInput {
	consistent := true
	return &dynamodb.GetItemInput{
		TableName:      aws.String(e.tableName),
		Key:            e.key(),
		ConsistentRead: &consistent,
	}
}

func (e *Elector) key() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		e.cfg.keyAttribute: &types.AttributeValueMemberS{Value: e.opts.Group},
	}
}

func (e *Elector) item(deadline time.Time) map[string]types.AttributeValue {
	item := e.key()
	item[e.cfg.ownerAttribute] = &types.AttributeValueMemberS{Value: e.token}
	item[e.cfg.leaseAttribute] = &types.AttributeValueMemberN{Value: strconv.FormatInt(deadline.UnixMilli(), 10)}
	item[e.cfg.ttlAttribute] = &types.AttributeValueMemberN{Value: strconv.FormatInt(ttlSeconds(deadline), 10)}
	return item
}

func (e *Elector) deadline() time.Time {
	return e.now().Add(e.opts.Lease)
}

func (e *Elector) now() time.Time {
	return e.cfg.clock().UTC()
}

func (e *Elector) attemptBudget() time.Duration {
	budget := e.opts.Lease / 4
	if budget <= 0 {
		budget = time.Millisecond
	}
	if budget > e.opts.RenewInterval {
		budget = e.opts.RenewInterval
	}
	if budget > time.Second {
		budget = time.Second
	}
	return budget
}

func (e *Elector) renewalBudget() time.Duration {
	budget := e.opts.Lease / 2
	if budget <= 0 {
		budget = time.Millisecond
	}
	if budget > e.opts.RenewInterval {
		budget = e.opts.RenewInterval
	}
	return budget
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

func (e *Elector) startRenewal() {
	renewCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	e.mu.Lock()
	e.generation++
	e.owned = true
	e.cleanup = false
	e.cancel = cancel
	e.done = done
	generation := e.generation
	e.mu.Unlock()
	go e.renewLoop(renewCtx, generation, done)
}

func (e *Elector) clearOwnership() (uint64, context.CancelFunc, chan struct{}, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.owned && !e.cleanup {
		return e.generation, nil, nil, false
	}
	generation := e.generation
	e.owned = false
	e.cleanup = true
	e.resigning++
	return generation, e.cancel, e.done, true
}

func (e *Elector) finishResign(generation uint64, resolved bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.generation != generation || e.resigning == 0 {
		return
	}
	if resolved {
		e.resolved = true
	}
	e.resigning--
	if e.resigning == 0 && e.resolved {
		e.cleanup = false
		e.resolved = false
		e.cancel = nil
		e.done = nil
	}
}

func (e *Elector) clearOwnershipAfterLoss(generation uint64, done chan struct{}, cleanup bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.generation != generation || e.done != done {
		return
	}
	e.owned = false
	e.cleanup = cleanup || e.resigning > 0
	if e.resigning == 0 {
		e.cancel = nil
		e.done = nil
	}
}

func (e *Elector) markCleanupPending() {
	e.mu.Lock()
	e.owned = false
	e.cleanup = true
	cancel := e.cancel
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *Elector) log(ctx context.Context, message string, level slogLevel, operation string) {
	if e == nil || e.cfg.logger == nil {
		return
	}
	e.cfg.logger.Log(ctx, level.value(), message, "operation", operation)
}

type slogLevel int

const (
	slogLevelError slogLevel = 8
	slogLevelWarn  slogLevel = 4
	slogLevelInfo  slogLevel = 0
)

func (l slogLevel) value() slog.Level {
	return slog.Level(l)
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	return ctx.Err()
}

func isConditionalFailure(err error) bool {
	var conditional *types.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return true
	}
	var coded interface{ ErrorCode() string }
	return errors.As(err, &coded) && coded.ErrorCode() == "ConditionalCheckFailedException"
}

func stringValue(value types.AttributeValue) (string, bool) {
	typed, ok := value.(*types.AttributeValueMemberS)
	if !ok {
		return "", false
	}
	return typed.Value, true
}

func numberValue(value types.AttributeValue) (int64, bool) {
	typed, ok := value.(*types.AttributeValueMemberN)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseInt(typed.Value, 10, 64)
	return parsed, err == nil
}

func ttlSeconds(deadline time.Time) int64 {
	seconds := deadline.Unix()
	if deadline.Nanosecond() != 0 {
		seconds++
	}
	return seconds
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
