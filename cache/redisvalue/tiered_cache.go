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
