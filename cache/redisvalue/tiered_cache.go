package redisvalue

import (
	"context"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btredis "github.com/bluetape4k/bluetape-go/redis"
)

// TieredOptions configures a process-local L1 decorator around a serialized
// Redis ValueCache L2. Local becomes exclusively owned by the decorator for
// cache operations, while its lifecycle remains caller-owned.
type TieredOptions[V any] struct {
	Local  cache.Cache[string, V]
	Remote *ValueCache[V]
	Config *TieredConfig
}

// TieredCache composes a caller-owned process-local L1 with a ValueCache L2.
// It stores V directly in L1 and is not a coherent multi-process near cache.
// Its zero value is not usable; construct it with NewTieredCache.
type TieredCache[V any] struct {
	local        cache.Cache[string, V]
	remote       *ValueCache[V]
	config       TieredConfig
	coordinators *coordinatorRegistry[V]
	localState   *localState
	now          func() time.Time
}

// NewTieredCache constructs a reference-preserving process-local L1 decorator.
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

// Get returns a stable L1 hit without serialization, or reads and decodes L2
// after an exact L1 miss.
func (c *TieredCache[V]) Get(ctx context.Context, key string) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := c.validateCall("get", ctx, key); err != nil {
		return zero, err
	}
	if value, hit, err := c.localGet(ctx, key); err != nil || hit {
		return value, err
	}
	return c.getRemoteCoordinated(ctx, key)
}

// GetOrLoad returns an L1 or L2 hit, or collapses one caller loader invocation
// for the active same-key process-local flight.
func (c *TieredCache[V]) GetOrLoad(
	ctx context.Context,
	key string,
	remoteTTL time.Duration,
	loader cache.Loader[string, V],
) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := c.validateCall("get-or-load", ctx, key); err != nil {
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

// GetOrLoadDefault delegates to GetOrLoad using the copied L2 default TTL.
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

func (c *TieredCache[V]) validateCall(operation string, ctx context.Context, key string) error {
	if err := c.validateInitialized(operation); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateLogicalKey(key)
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
