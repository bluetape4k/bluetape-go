package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

func (e entry[V]) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
}

// Memory 는 process-local TTL cache다.
type Memory[K comparable, V any] struct {
	mu         sync.RWMutex
	values     map[K]entry[V]
	flightKeys map[K]string
	nextFlight uint64
	flights    singleflight.Group
	now        func() time.Time
}

// NewMemory 는 process-local loading cache를 만든다.
func NewMemory[K comparable, V any]() *Memory[K, V] {
	return newMemoryWithClock[K, V](time.Now)
}

func newMemoryWithClock[K comparable, V any](now func() time.Time) *Memory[K, V] {
	if now == nil {
		now = time.Now
	}
	return &Memory[K, V]{
		values:     make(map[K]entry[V]),
		flightKeys: make(map[K]string),
		now:        now,
	}
}

// Get 은 key의 값을 반환한다.
func (m *Memory[K, V]) Get(ctx context.Context, key K) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.values[key]
	if !ok {
		return zero, ErrCacheMiss
	}
	if item.expired(m.now()) {
		delete(m.values, key)
		delete(m.flightKeys, key)
		return zero, ErrCacheMiss
	}
	return item.value, nil
}

// Set 은 key에 value를 저장한다.
func (m *Memory[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateTTL(ttl); err != nil {
		return err
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = m.now().Add(ttl)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = entry[V]{value: value, expiresAt: expiresAt}
	return nil
}

// Delete 는 key의 값을 제거한다.
func (m *Memory[K, V]) Delete(ctx context.Context, key K) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	delete(m.flightKeys, key)
	return nil
}

// Clear 는 모든 값을 제거한다.
func (m *Memory[K, V]) Clear(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.values)
	clear(m.flightKeys)
	return nil
}

// GetOrLoad 는 miss일 때 loader로 값을 채운다.
func (m *Memory[K, V]) GetOrLoad(ctx context.Context, key K, ttl time.Duration, loader Loader[K, V]) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := validateTTL(ttl); err != nil {
		return zero, err
	}
	if loader == nil {
		return zero, fmt.Errorf("loader must not be nil")
	}

	value, err := m.Get(ctx, key)
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, ErrCacheMiss) {
		return zero, err
	}

	flightKey := m.flightKey(key)
	result := m.flights.DoChan(flightKey, func() (any, error) {
		loaded, err := m.Get(ctx, key)
		if err == nil {
			return loaded, nil
		}
		if !errors.Is(err, ErrCacheMiss) {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		loaded, err = loader(ctx, key)
		if err != nil {
			m.deleteFlightKey(key)
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			m.deleteFlightKey(key)
			return nil, err
		}
		if err := m.Set(ctx, key, loaded, ttl); err != nil {
			m.deleteFlightKey(key)
			return nil, err
		}
		return loaded, nil
	})

	select {
	case call := <-result:
		if call.Err != nil {
			return zero, call.Err
		}
		loaded, ok := call.Val.(V)
		if !ok {
			return zero, fmt.Errorf("loader returned incompatible value")
		}
		return loaded, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (m *Memory[K, V]) flightKey(key K) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.flightKeys[key]; ok {
		return existing
	}
	m.nextFlight++
	keyID := strconv.FormatUint(m.nextFlight, 10)
	m.flightKeys[key] = keyID
	return keyID
}

func (m *Memory[K, V]) deleteFlightKey(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.flightKeys, key)
}
