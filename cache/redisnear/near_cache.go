package redisnear

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/redis/go-redis/v9"
)

const (
	defaultNamespace       = "default"
	defaultChannelPrefix   = "bluetape:cache:near"
	defaultErrorBuffer     = 16
	initialReceiveBackoff  = 10 * time.Millisecond
	maximumReceiveBackoff  = 250 * time.Millisecond
	receiverShutdownBudget = time.Second
)

// ErrClosed는 변수 공개 값이며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
// 호출자는 이 식별자를 cache 오류, 옵션, event, 또는 기본값 계약을 비교할 때 사용한다.
var ErrClosed = errors.New("near cache is closed")

// Client는 interface 공개 타입이며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Client interface {
	Publish(ctx context.Context, channel string, message any) *redis.IntCmd
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
}

// OnError는 func 공개 타입이며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type OnError func(context.Context, error)

// Options는 struct 공개 타입이며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Options[V any] struct {
	// Client는 Redis publish/subscribe backend다. 필수다.
	Client Client
	// Namespace는 같은 invalidation scope를 공유하는 cache group이다.
	Namespace string
	// Channel은 Redis Pub/Sub channel이다. 비우면 namespace 기반 기본값을 쓴다.
	Channel string
	// OriginID는 자기 자신이 보낸 invalidation을 무시하기 위한 token이다.
	OriginID string
	// Local은 값 저장을 담당하는 process-local cache다.
	Local cache.LoadingCache[string, V]
	// OnError는 malformed message나 subscriber 오류를 보고한다.
	OnError OnError
}

type config[V any] struct {
	client    Client
	namespace string
	channel   string
	originID  string
	local     cache.LoadingCache[string, V]
	onError   OnError
}

type errorReport struct {
	ctx context.Context
	err error
}

// NearCache는 struct 공개 타입이며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type NearCache[V any] struct {
	cfg       config[V]
	pubsub    *redis.PubSub
	cancel    context.CancelFunc
	done      chan struct{}
	errorCh   chan errorReport
	errorDone chan struct{}

	mu       sync.Mutex
	closed   bool
	inflight sync.WaitGroup
}

var _ cache.LoadingCache[string, string] = (*NearCache[string])(nil)

// NewPubSub는 NewPubSub 공개 API의 동작을 수행하며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - options: NewPubSub 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func NewPubSub[V any](ctx context.Context, options Options[V]) (*NearCache[V], error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}

	pubsub := cfg.client.Subscribe(ctx, cfg.channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	near := &NearCache[V]{
		cfg:     cfg,
		pubsub:  pubsub,
		cancel:  cancel,
		done:    make(chan struct{}),
		errorCh: newErrorChannel[V](cfg),
	}
	if near.errorCh != nil {
		near.errorDone = make(chan struct{})
		go near.reportErrors(runCtx)
	}
	go near.receive(runCtx)
	return near, nil
}

// Get는 Get 공개 API의 동작을 수행하며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *NearCache[V]) Get(ctx context.Context, key string) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	release, err := c.enter()
	if err != nil {
		return zero, err
	}
	defer release()
	return c.cfg.local.Get(ctx, key)
}

// Set는 Set 공개 API의 동작을 수행하며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//   - ttl: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *NearCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := c.enter()
	if err != nil {
		return err
	}
	defer release()
	if err := c.cfg.local.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	return c.publish(ctx, operationSet, key)
}

// Delete는 Delete 공개 API의 동작을 수행하며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *NearCache[V]) Delete(ctx context.Context, key string) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := c.enter()
	if err != nil {
		return err
	}
	defer release()
	if err := c.cfg.local.Delete(ctx, key); err != nil {
		return err
	}
	return c.publish(ctx, operationDelete, key)
}

// Clear는 Clear 공개 API의 동작을 수행하며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *NearCache[V]) Clear(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := c.enter()
	if err != nil {
		return err
	}
	defer release()
	if err := c.cfg.local.Clear(ctx); err != nil {
		return err
	}
	return c.publish(ctx, operationClear, "")
}

// GetOrLoad는 GetOrLoad 공개 API의 동작을 수행하며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - ttl: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//   - loader: GetOrLoad 동작에 필요한 loader 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *NearCache[V]) GetOrLoad(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loader cache.Loader[string, V],
) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	release, err := c.enter()
	if err != nil {
		return zero, err
	}
	defer release()
	return c.cfg.local.GetOrLoad(ctx, key, ttl, loader)
}

// Close는 Close 공개 API의 동작을 수행하며 near-cache local state, invalidation message, Redis Pub/Sub 계약을 보존한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *NearCache[V]) Close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	cancel := c.cancel
	pubsub := c.pubsub
	done := c.done
	errorDone := c.errorDone
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	var closeErr error
	if pubsub != nil {
		closeErr = pubsub.Close()
	}
	inflightErr := c.waitInflight()
	if done != nil {
		select {
		case <-done:
		case <-time.After(receiverShutdownBudget):
			return errors.Join(closeErr, inflightErr, fmt.Errorf("near cache subscriber did not stop"))
		}
	}
	errorReporterErr := waitForShutdown(errorDone, "near cache error reporter")
	return errors.Join(closeErr, inflightErr, errorReporterErr)
}

func normalizeOptions[V any](options Options[V]) (config[V], error) {
	if options.Client == nil {
		return config[V]{}, fmt.Errorf("redis client must not be nil")
	}
	namespace := options.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	channel := options.Channel
	if channel == "" {
		channel = defaultChannel(namespace)
	}
	originID := options.OriginID
	if originID == "" {
		generated, err := randomOriginID()
		if err != nil {
			return config[V]{}, err
		}
		originID = generated
	}
	local := options.Local
	if local == nil {
		local = cache.NewMemory[string, V]()
	}
	return config[V]{
		client:    options.Client,
		namespace: namespace,
		channel:   channel,
		originID:  originID,
		local:     local,
		onError:   options.OnError,
	}, nil
}

func defaultChannel(namespace string) string {
	return defaultChannelPrefix + ":" + namespace + ":invalidate"
}

func randomOriginID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate near-cache origin id: %w", err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

func (c *NearCache[V]) enter() (func(), error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	c.inflight.Add(1)
	c.mu.Unlock()
	return c.inflight.Done, nil
}

func (c *NearCache[V]) publish(ctx context.Context, op operation, key string) error {
	payload, err := encodeMessage(invalidationMessage{
		Namespace: c.cfg.namespace,
		OriginID:  c.cfg.originID,
		Operation: op,
		Key:       key,
	})
	if err != nil {
		return err
	}
	return c.cfg.client.Publish(ctx, c.cfg.channel, payload).Err()
}

func (c *NearCache[V]) receive(ctx context.Context) {
	defer close(c.done)

	backoff := initialReceiveBackoff
	for {
		message, err := c.pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || c.isClosed() {
				return
			}
			_ = c.cfg.local.Clear(context.Background())
			c.reportError(ctx, err)
			if !sleep(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}

		backoff = initialReceiveBackoff
		c.applyMessage(ctx, message.Payload)
	}
}

func (c *NearCache[V]) applyMessage(ctx context.Context, payload string) {
	message, err := decodeMessage(payload)
	if err != nil {
		c.reportError(ctx, err)
		return
	}
	if message.Namespace != c.cfg.namespace || message.OriginID == c.cfg.originID {
		return
	}

	switch message.Operation {
	case operationSet, operationDelete:
		if err := c.cfg.local.Delete(context.Background(), message.Key); err != nil {
			c.reportError(ctx, err)
		}
	case operationClear:
		if err := c.cfg.local.Clear(context.Background()); err != nil {
			c.reportError(ctx, err)
		}
	}
}

func (c *NearCache[V]) reportError(ctx context.Context, err error) {
	if err == nil || c.cfg.onError == nil {
		return
	}
	if c.errorCh == nil {
		c.callOnError(ctx, err)
		return
	}
	select {
	case c.errorCh <- errorReport{ctx: ctx, err: err}:
	default:
	}
}

func (c *NearCache[V]) reportErrors(ctx context.Context) {
	defer close(c.errorDone)
	for {
		select {
		case <-ctx.Done():
			return
		case report, ok := <-c.errorCh:
			if !ok {
				return
			}
			c.callOnError(report.ctx, report.err)
		}
	}
}

func (c *NearCache[V]) callOnError(ctx context.Context, err error) {
	defer func() {
		_ = recover()
	}()
	c.cfg.onError(ctx, err)
}

func (c *NearCache[V]) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > maximumReceiveBackoff {
		return maximumReceiveBackoff
	}
	return next
}

func sleep(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func newErrorChannel[V any](cfg config[V]) chan errorReport {
	if cfg.onError == nil {
		return nil
	}
	return make(chan errorReport, defaultErrorBuffer)
}

func (c *NearCache[V]) waitInflight() error {
	done := make(chan struct{})
	go func() {
		c.inflight.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(receiverShutdownBudget):
		return fmt.Errorf("near cache operations did not stop")
	}
}

func waitForShutdown(done <-chan struct{}, name string) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-time.After(receiverShutdownBudget):
		return fmt.Errorf("%s did not stop", name)
	}
}
