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

// Memory struct 공개 타입이며 in-memory cache의 key, TTL, snapshot, miss 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Memory[K comparable, V any] struct {
	mu         sync.RWMutex
	values     map[K]entry[V]
	flightKeys map[K]string
	nextFlight uint64
	flights    singleflight.Group
	now        func() time.Time
}

// NewMemory NewMemory 공개 API의 동작을 수행하며 in-memory cache의 key, TTL, snapshot, miss 계약을 보존한다.
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

func (m *Memory[K, V]) currentTime() time.Time {
	if m.now == nil {
		return time.Now()
	}
	return m.now()
}

// Get Get 공개 API의 동작을 수행하며 in-memory cache의 key, TTL, snapshot, miss 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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
	if item.expired(m.currentTime()) {
		delete(m.values, key)
		delete(m.flightKeys, key)
		return zero, ErrCacheMiss
	}
	return item.value, nil
}

// Set Set 공개 API의 동작을 수행하며 in-memory cache의 key, TTL, snapshot, miss 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//   - ttl: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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
		expiresAt = m.currentTime().Add(ttl)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureValues()
	m.values[key] = entry[V]{value: value, expiresAt: expiresAt}
	return nil
}

// Delete Delete 공개 API의 동작을 수행하며 in-memory cache의 key, TTL, snapshot, miss 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// Clear Clear 공개 API의 동작을 수행하며 in-memory cache의 key, TTL, snapshot, miss 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

// GetOrLoad GetOrLoad 공개 API의 동작을 수행하며 in-memory cache의 key, TTL, snapshot, miss 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - ttl: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//   - loader: GetOrLoad에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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

	for {
		flightKey := m.flightKey(key)
		result := m.flights.DoChan(flightKey, func() (any, error) {
			loaded, err := m.Get(ctx, key)
			if err == nil {
				return loaded, nil
			}
			if !errors.Is(err, ErrCacheMiss) {
				if isContextError(err) {
					m.deleteFlightKey(key)
				}
				return nil, err
			}
			if err := ctx.Err(); err != nil {
				m.deleteFlightKey(key)
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
				if call.Shared && isContextError(call.Err) && ctx.Err() == nil {
					m.flights.Forget(flightKey)
					continue
				}
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
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (m *Memory[K, V]) flightKey(key K) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureFlightKeys()

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

func (m *Memory[K, V]) ensureValues() {
	if m.values == nil {
		m.values = make(map[K]entry[V])
	}
}

func (m *Memory[K, V]) ensureFlightKeys() {
	if m.flightKeys == nil {
		m.flightKeys = make(map[K]string)
	}
}
