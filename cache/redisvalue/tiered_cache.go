package redisvalue

import (
	"context"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btredis "github.com/bluetape4k/bluetape-go/redis"
)

// TieredOptions struct 공개 타입이며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type TieredOptions[V any] struct {
	// Local은 새 cache이거나 비어 있어야 하며, 같은 Remote/namespace/key 계약의 값만 포함해야 한다.
	// Namespace, schema, and tenant represented by Remote. It must not be shared
	// with another decorator.
	Local cache.Cache[string, V]
	// Remote 이 cache가 감싸는 serialized L2 provider다.
	Remote *ValueCache[V]
	// Config 생성 시 복사된다. nil이면 DefaultConfig().Tiered를 사용한다.
	Config *TieredConfig
}

// TieredCache struct 공개 타입이며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type TieredCache[V any] struct {
	local        cache.Cache[string, V]
	remote       *ValueCache[V]
	config       TieredConfig
	coordinators *coordinatorRegistry[V]
	localState   *localState
	now          func() time.Time
}

// NewTieredCache NewTieredCache 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewTieredCache[V any](options TieredOptions[V]) (*TieredCache[V], error) {
	if nilInterface(options.Local) || !initializedValueCache(options.Remote) {
		return nil, newCacheError("configure-tiered", ReasonConfiguration, "", nil)
	}
	config := DefaultConfig().Tiered
	if options.Config != nil {
		config = *options.Config
	}
	if err := validateTieredConfig(config); err != nil {
		return nil, err
	}
	if options.Remote.config.RemoteTTL > 0 && config.LocalTTL > options.Remote.config.RemoteTTL {
		return nil, newCacheError(
			"configure-tiered",
			ReasonConfiguration,
			"",
			errors.Join(btredis.ErrInvalidTTL, errors.New("local ttl exceeds remote default ttl")),
		)
	}
	return &TieredCache[V]{
		local:        options.Local,
		remote:       options.Remote,
		config:       config,
		coordinators: newCoordinatorRegistry[V](),
		localState:   newLocalState(),
		now:          time.Now,
	}, nil
}

// Get Get 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) Get(ctx context.Context, key string) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := c.validateCall(ctx, "get", key); err != nil {
		return zero, err
	}
	if value, hit, err := c.localGet(ctx, key); err != nil || hit {
		return value, err
	}
	return c.getRemoteCoordinated(ctx, key)
}

// GetOrLoad GetOrLoad 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - remoteTTL: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//   - loader: GetOrLoad에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) GetOrLoad(
	ctx context.Context,
	key string,
	remoteTTL time.Duration,
	loader cache.Loader[string, V],
) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := c.validateCall(ctx, "get-or-load", key); err != nil {
		return zero, err
	}
	if err := validateEntryTTL(remoteTTL); err != nil {
		return zero, err
	}
	if loader == nil {
		return zero, newCacheError("get-or-load", ReasonConfiguration, c.keyID(key), nil)
	}
	if value, hit, err := c.localGet(ctx, key); err != nil || hit {
		return value, err
	}

	coordinator := c.coordinators.acquire(key)
	defer c.coordinators.release(key, coordinator)
	flight, leader := coordinator.joinFlight()
	if !leader {
		return coordinator.waitFlight(ctx, flight)
	}

	if err := coordinator.acquireToken(ctx); err != nil {
		coordinator.publishFlight(flight, zero, err)
		return zero, err
	}
	tokenHeld := true
	value, err := c.runLoadLeader(ctx, key, remoteTTL, loader, coordinator, &tokenHeld)
	coordinator.publishFlight(flight, value, err)
	if tokenHeld {
		coordinator.releaseToken()
	}
	return value, err
}

// GetOrLoadDefault GetOrLoadDefault 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - loader: GetOrLoadDefault에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) GetOrLoadDefault(
	ctx context.Context,
	key string,
	loader cache.Loader[string, V],
) (V, error) {
	if c == nil || !initializedValueCache(c.remote) {
		var zero V
		return zero, newCacheError("get-or-load-default", ReasonUninitialized, "", nil)
	}
	return c.GetOrLoad(ctx, key, c.remote.config.RemoteTTL, loader)
}

// Set Set 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//   - remoteTTL: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) Set(ctx context.Context, key string, value V, remoteTTL time.Duration) error {
	ctx = normalizeContext(ctx)
	if err := c.validateCall(ctx, "set", key); err != nil {
		return err
	}
	if err := validateEntryTTL(remoteTTL); err != nil {
		return err
	}
	coordinator := c.coordinators.acquire(key)
	defer c.coordinators.release(key, coordinator)
	if err := coordinator.acquireToken(ctx); err != nil {
		return err
	}
	tokenHeld := true
	defer func() {
		if tokenHeld {
			coordinator.releaseToken()
		}
	}()

	var generation uint64
	for {
		lease, retry, err := c.acquireHealthyForLeader(ctx, coordinator, &tokenHeld)
		if err != nil {
			return c.withCallOperation("set", key, err)
		}
		if retry {
			continue
		}
		ticket, admitted := lease.issueTicket()
		generation = lease.generation
		lease.release()
		if !admitted {
			if c.localState.classify(generation) == localBlocked {
				return c.localBlockedCallError("set", key, nil)
			}
			continue
		}
		if err := ticket.consume(ctx); err != nil {
			return err
		}
		break
	}

	now := c.now
	if now == nil {
		now = time.Now
	}
	started := now()
	remoteErr := c.remote.Set(ctx, key, value, remoteTTL)
	disposition := c.localState.classify(generation)
	if remoteErr != nil {
		cleanupErr := c.mandatoryInvalidateLocalHeld(key)
		return c.mutationResult("set", key, generation, remoteErr, cleanupErr)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		cleanupErr := c.mandatoryInvalidateLocalHeld(key)
		return c.mutationResult("set", key, generation, contextErr, cleanupErr)
	}
	if disposition == localBlocked {
		cleanupErr := c.mandatoryInvalidateLocalHeld(key)
		return c.mutationResult("set", key, generation, nil, cleanupErr)
	}
	if disposition == localNewerGeneration {
		return nil
	}
	localTTL, ok := knownWriteLocalTTL(c.config.LocalTTL, remoteTTL, started, now)
	if !ok {
		return nil
	}
	return c.populateLocalHeld(ctx, key, value, localTTL, generation)
}

// SetDefault SetDefault 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) SetDefault(ctx context.Context, key string, value V) error {
	if c == nil || !initializedValueCache(c.remote) {
		return newCacheError("set-default", ReasonUninitialized, "", nil)
	}
	return c.Set(ctx, key, value, c.remote.config.RemoteTTL)
}

// Delete Delete 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) Delete(ctx context.Context, key string) error {
	ctx = normalizeContext(ctx)
	if err := c.validateCall(ctx, "delete", key); err != nil {
		return err
	}
	coordinator := c.coordinators.acquire(key)
	defer c.coordinators.release(key, coordinator)
	if err := coordinator.acquireToken(ctx); err != nil {
		return err
	}
	tokenHeld := true
	defer func() {
		if tokenHeld {
			coordinator.releaseToken()
		}
	}()

	var generation uint64
	for {
		lease, retry, err := c.acquireHealthyForLeader(ctx, coordinator, &tokenHeld)
		if err != nil {
			return c.withCallOperation("delete", key, err)
		}
		if retry {
			continue
		}
		ticket, admitted := lease.issueTicket()
		generation = lease.generation
		lease.release()
		if !admitted {
			if c.localState.classify(generation) == localBlocked {
				return c.localBlockedCallError("delete", key, nil)
			}
			continue
		}
		if err := ticket.consume(ctx); err != nil {
			return err
		}
		break
	}

	remoteErr := c.remote.Delete(ctx, key)
	cleanupErr := c.mandatoryInvalidateLocalHeld(key)
	if remoteErr == nil && ctx.Err() != nil {
		remoteErr = ctx.Err()
	}
	return c.mutationResult("delete", key, generation, remoteErr, cleanupErr)
}

// InvalidateLocal InvalidateLocal 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) InvalidateLocal(ctx context.Context, key string) error {
	ctx = normalizeContext(ctx)
	if err := c.validateCall(ctx, "invalidate-local", key); err != nil {
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, c.config.InvalidationWaitTimeout)
	defer cancel()
	coordinator := c.coordinators.acquire(key)
	defer c.coordinators.release(key, coordinator)
	if err := coordinator.acquireToken(waitCtx); err != nil {
		c.localState.block()
		return c.localBlockedCallError("invalidate-local", key, err)
	}
	defer coordinator.releaseToken()
	cleanupCtx, cleanupCancel := context.WithTimeout(waitCtx, c.config.LocalCleanupTimeout)
	defer cleanupCancel()
	return c.invalidateLocalHeld(cleanupCtx, key)
}

// ClearLocal ClearLocal 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) ClearLocal(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	if err := c.validateInitialized("clear-local"); err != nil {
		return err
	}
	cleanupCtx, cancel := context.WithTimeout(ctx, c.config.LocalCleanupTimeout)
	defer cancel()
	repair, err := c.localState.beginRepair(cleanupCtx, repairExplicit)
	if err != nil {
		c.localState.block()
		return c.localBlockedCallError("clear-local", "", err)
	}
	localErr := c.local.Clear(cleanupCtx)
	if localErr == nil {
		localErr = cleanupCtx.Err()
	}
	if !c.localState.finishRepair(repair, localErr) {
		return c.localBlockedCallError("clear-local", "", localErr)
	}
	return nil
}

// Clear Clear 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *TieredCache[V]) Clear(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	if err := c.validateNoKeyCall(ctx, "clear"); err != nil {
		return err
	}
	generation := c.localState.generationValue()
	remoteErr := c.remote.Clear(ctx)
	localErr := c.mandatoryClearLocal(generation)
	if localErr != nil {
		return c.localBlockedCallError("clear", "", errors.Join(remoteErr, localErr))
	}
	return remoteErr
}

func (c *TieredCache[V]) mandatoryClearLocal(generation uint64) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), c.config.LocalCleanupTimeout)
	defer cancel()
	repair, disposition, err := c.localState.beginRepairAtGeneration(cleanupCtx, repairMandatory, &generation)
	if err != nil {
		c.localState.block()
		return c.localBlockedCallError("clear-local", "", err)
	}
	if disposition == localNewerGeneration {
		return nil
	}
	if disposition == localBlocked {
		return c.localBlockedCallError("clear-local", "", nil)
	}
	localErr := c.local.Clear(cleanupCtx)
	if localErr == nil {
		localErr = cleanupCtx.Err()
	}
	if !c.localState.finishRepair(repair, localErr) {
		return c.localBlockedCallError("clear-local", "", localErr)
	}
	return nil
}

func (c *TieredCache[V]) mutationResult(
	operation string,
	key string,
	generation uint64,
	primaryErr error,
	cleanupErr error,
) error {
	if cleanupErr != nil {
		return c.localBlockedCallError(operation, key, errors.Join(primaryErr, cleanupErr))
	}
	if c.localState.classify(generation) == localBlocked {
		return c.localBlockedCallError(operation, key, primaryErr)
	}
	return primaryErr
}

func (c *TieredCache[V]) runLoadLeader(
	ctx context.Context,
	key string,
	remoteTTL time.Duration,
	loader cache.Loader[string, V],
	coordinator *keyCoordinator[V],
	tokenHeld *bool,
) (V, error) {
	var zero V
	for {
		lease, retry, err := c.acquireHealthyForLeader(ctx, coordinator, tokenHeld)
		if err != nil {
			return zero, c.withCallOperation("get-or-load", key, err)
		}
		if retry {
			continue
		}

		localValue, localErr := c.local.Get(ctx, key)
		disposition := c.localState.classify(lease.generation)
		if localErr == nil {
			lease.release()
			if disposition == localBlocked {
				return zero, c.localBlockedCallError("get-or-load", key, nil)
			}
			return localValue, nil
		}
		if !errors.Is(localErr, cache.ErrCacheMiss) {
			lease.release()
			if disposition == localBlocked {
				return zero, c.localBlockedCallError("get-or-load", key, localErr)
			}
			return zero, c.localFailureError("get-or-load", key, localErr)
		}
		remoteReadTicket, admitted := lease.issueTicket()
		generation := lease.generation
		lease.release()
		if !admitted {
			if c.localState.classify(generation) == localBlocked {
				return zero, c.localBlockedCallError("get-or-load", key, nil)
			}
			continue
		}
		if err := remoteReadTicket.consume(ctx); err != nil {
			return zero, err
		}

		remoteValue, remoteErr := c.remote.Get(ctx, key)
		disposition = c.localState.classify(generation)
		if disposition == localBlocked {
			return zero, c.localBlockedCallError("get-or-load", key, remoteErr)
		}
		if remoteErr == nil {
			if disposition == localNewerGeneration {
				return remoteValue, nil
			}
			if err := c.populateLocalHeld(ctx, key, remoteValue, c.config.LocalTTL, generation); err != nil {
				return zero, err
			}
			return remoteValue, nil
		}
		if !errors.Is(remoteErr, cache.ErrCacheMiss) {
			return zero, remoteErr
		}
		if disposition == localNewerGeneration {
			continue
		}

		loaderLease, retry, err := c.acquireHealthyForLeader(ctx, coordinator, tokenHeld)
		if err != nil {
			return zero, c.withCallOperation("get-or-load", key, err)
		}
		if retry {
			continue
		}
		if loaderLease.generation != generation {
			loaderLease.release()
			continue
		}
		loaderTicket, admitted := loaderLease.issueTicket()
		loaderLease.release()
		if !admitted {
			if c.localState.classify(generation) == localBlocked {
				return zero, c.localBlockedCallError("get-or-load", key, nil)
			}
			continue
		}
		if err := loaderTicket.consume(ctx); err != nil {
			return zero, err
		}
		loaded, loadErr := loader(ctx, key)
		disposition = c.localState.classify(generation)
		if disposition == localBlocked {
			return zero, c.localBlockedCallError("get-or-load", key, loadErr)
		}
		if loadErr != nil {
			return zero, loadErr
		}
		if disposition == localNewerGeneration {
			return loaded, nil
		}

		writeLease, changed, stateErr := c.localState.tryAcquireHealthy()
		if stateErr != nil {
			return zero, c.withCallOperation("get-or-load", key, stateErr)
		}
		if changed != nil {
			return loaded, nil
		}
		if writeLease.generation != generation {
			writeLease.release()
			if c.localState.classify(generation) == localBlocked {
				return zero, c.localBlockedCallError("get-or-load", key, nil)
			}
			return loaded, nil
		}
		writeTicket, admitted := writeLease.issueTicket()
		writeLease.release()
		if !admitted {
			if c.localState.classify(generation) == localBlocked {
				return zero, c.localBlockedCallError("get-or-load", key, nil)
			}
			return loaded, nil
		}
		if err := writeTicket.consume(ctx); err != nil {
			return zero, err
		}
		now := c.now
		if now == nil {
			now = time.Now
		}
		started := now()
		remoteErr = c.remote.Set(ctx, key, loaded, remoteTTL)
		disposition = c.localState.classify(generation)
		if disposition == localBlocked {
			return zero, c.localBlockedCallError("get-or-load", key, remoteErr)
		}
		if remoteErr != nil {
			return zero, remoteErr
		}
		if disposition == localNewerGeneration {
			return loaded, nil
		}
		localTTL, ok := knownWriteLocalTTL(c.config.LocalTTL, remoteTTL, started, now)
		if !ok {
			return loaded, nil
		}
		if err := c.populateLocalHeld(ctx, key, loaded, localTTL, generation); err != nil {
			return zero, err
		}
		return loaded, nil
	}
}

func (c *TieredCache[V]) acquireHealthyForLeader(
	ctx context.Context,
	coordinator *keyCoordinator[V],
	tokenHeld *bool,
) (localLease, bool, error) {
	lease, changed, err := c.localState.tryAcquireHealthy()
	if err != nil || changed == nil {
		return lease, false, err
	}
	coordinator.releaseToken()
	*tokenHeld = false
	if err := waitLocalState(ctx, changed); err != nil {
		return localLease{}, false, err
	}
	if err := coordinator.acquireToken(ctx); err != nil {
		return localLease{}, false, err
	}
	*tokenHeld = true
	return localLease{}, true, nil
}

func (c *TieredCache[V]) localGet(ctx context.Context, key string) (V, bool, error) {
	var zero V
	lease, err := c.localState.acquireHealthy(ctx)
	if err != nil {
		return zero, false, c.withCallOperation("get", key, err)
	}
	value, localErr := c.local.Get(ctx, key)
	disposition := c.localState.classify(lease.generation)
	lease.release()
	if disposition == localBlocked {
		return zero, false, c.localBlockedCallError("get", key, localErr)
	}
	if localErr == nil {
		return value, true, nil
	}
	if errors.Is(localErr, cache.ErrCacheMiss) {
		return zero, false, nil
	}
	return zero, false, c.localFailureError("get", key, localErr)
}

func (c *TieredCache[V]) getRemoteCoordinated(ctx context.Context, key string) (V, error) {
	var zero V
	coordinator := c.coordinators.acquire(key)
	defer c.coordinators.release(key, coordinator)
	if err := coordinator.acquireToken(ctx); err != nil {
		return zero, err
	}
	tokenHeld := true
	defer func() {
		if tokenHeld {
			coordinator.releaseToken()
		}
	}()

	for {
		lease, changed, err := c.localState.tryAcquireHealthy()
		if err != nil {
			return zero, c.withCallOperation("get", key, err)
		}
		if changed != nil {
			coordinator.releaseToken()
			tokenHeld = false
			if err := waitLocalState(ctx, changed); err != nil {
				return zero, err
			}
			if err := coordinator.acquireToken(ctx); err != nil {
				return zero, err
			}
			tokenHeld = true
			continue
		}

		value, localErr := c.local.Get(ctx, key)
		disposition := c.localState.classify(lease.generation)
		if localErr == nil {
			lease.release()
			if disposition == localBlocked {
				return zero, c.localBlockedCallError("get", key, nil)
			}
			return value, nil
		}
		if !errors.Is(localErr, cache.ErrCacheMiss) {
			lease.release()
			if disposition == localBlocked {
				return zero, c.localBlockedCallError("get", key, localErr)
			}
			return zero, c.localFailureError("get", key, localErr)
		}
		ticket, admitted := lease.issueTicket()
		generation := lease.generation
		lease.release()
		if !admitted {
			if c.localState.classify(generation) == localBlocked {
				return zero, c.localBlockedCallError("get", key, nil)
			}
			continue
		}
		if err := ticket.consume(ctx); err != nil {
			return zero, err
		}

		remoteValue, remoteErr := c.remote.Get(ctx, key)
		disposition = c.localState.classify(generation)
		if disposition == localBlocked {
			return zero, c.localBlockedCallError("get", key, remoteErr)
		}
		if remoteErr != nil {
			return zero, remoteErr
		}
		if disposition == localNewerGeneration {
			return remoteValue, nil
		}
		if err := c.populateLocalHeld(ctx, key, remoteValue, c.config.LocalTTL, generation); err != nil {
			return zero, err
		}
		return remoteValue, nil
	}
}

func (c *TieredCache[V]) populateLocalHeld(
	ctx context.Context,
	key string,
	value V,
	ttl time.Duration,
	expectedGeneration uint64,
) error {
	lease, changed, err := c.localState.tryAcquireHealthy()
	if err != nil {
		return c.withCallOperation("populate-local", key, err)
	}
	if changed != nil {
		return nil
	}
	if lease.generation != expectedGeneration {
		lease.release()
		return nil
	}
	if err := ctx.Err(); err != nil {
		lease.release()
		return err
	}
	localErr := c.local.Set(ctx, key, value, ttl)
	disposition := c.localState.classify(expectedGeneration)
	lease.release()
	if localErr != nil {
		failure := c.localFailureError("populate-local", key, localErr)
		cleanupErr := c.mandatoryInvalidateLocalHeld(key)
		if cleanupErr != nil {
			return c.localBlockedCallError("populate-local", key, errors.Join(failure, cleanupErr))
		}
		if disposition == localBlocked {
			return c.localBlockedCallError("populate-local", key, failure)
		}
		return failure
	}
	if disposition != localCurrent {
		if cleanupErr := c.mandatoryInvalidateLocalHeld(key); cleanupErr != nil {
			return cleanupErr
		}
		if disposition == localBlocked {
			return c.localBlockedCallError("populate-local", key, nil)
		}
	}
	return nil
}

func (c *TieredCache[V]) mandatoryInvalidateLocalHeld(key string) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), c.config.LocalCleanupTimeout)
	defer cancel()
	return c.invalidateLocalHeld(cleanupCtx, key)
}

func (c *TieredCache[V]) invalidateLocalHeld(ctx context.Context, key string) error {
	maintenance, err := c.localState.acquireMaintenance(ctx)
	if err != nil {
		c.localState.block()
		return c.localBlockedCallError("invalidate-local", key, err)
	}
	localErr := c.local.Delete(ctx, key)
	maintenance.release()
	if localErr != nil {
		c.localState.block()
		return c.localBlockedCallError("invalidate-local", key, c.localFailureError("delete-local", key, localErr))
	}
	return nil
}

func (c *TieredCache[V]) validateCall(ctx context.Context, operation, key string) error {
	if err := c.validateInitialized(operation); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateLogicalKey(key)
}

func (c *TieredCache[V]) validateNoKeyCall(ctx context.Context, operation string) error {
	if err := c.validateInitialized(operation); err != nil {
		return err
	}
	return ctx.Err()
}

func (c *TieredCache[V]) validateInitialized(operation string) error {
	if c == nil || nilInterface(c.local) || !initializedValueCache(c.remote) || c.coordinators == nil || c.localState == nil {
		return newCacheError(operation, ReasonUninitialized, "", nil)
	}
	return nil
}

func initializedValueCache[V any](cache *ValueCache[V]) bool {
	return cache != nil && !nilInterface(cache.client) && !nilInterface(cache.serializer) && cache.namespace != ""
}

func (c *TieredCache[V]) keyID(key string) string {
	physical, err := c.remote.keys.LogicalKey(key)
	if err != nil {
		return ""
	}
	return physical.RedactedID
}

func (c *TieredCache[V]) localFailureError(operation, key string, cause error) error {
	return newCacheError(operation, ReasonLocalFailure, c.keyID(key), cause)
}

func (c *TieredCache[V]) localBlockedCallError(operation, key string, cause error) error {
	return newCacheError(operation, ReasonLocalBlocked, c.keyID(key), cause)
}

func (c *TieredCache[V]) withCallOperation(operation, key string, err error) error {
	var cacheErr *CacheError
	if errors.As(err, &cacheErr) && cacheErr.Reason() == ReasonLocalBlocked {
		return c.localBlockedCallError(operation, key, err)
	}
	return err
}

func waitLocalState(ctx context.Context, changed <-chan struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-changed:
		return nil
	}
}
