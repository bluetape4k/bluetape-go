package redisvalue

import (
	"context"
	"sync"
	"sync/atomic"
)

type loadFlight[V any] struct {
	generation   uint64
	done         chan struct{}
	value        V
	err          error
	published    bool
	participants atomic.Int64
}

type keyCoordinator[V any] struct {
	mu          sync.Mutex
	token       chan struct{}
	flight      *loadFlight[V]
	nextFlight  uint64
	externalRef int64
	tokenUsers  int64
}

type coordinatorRegistry[V any] struct {
	mu           sync.Mutex
	items        map[string]*keyCoordinator[V]
	beforeRetire func()
}

func newCoordinatorRegistry[V any]() *coordinatorRegistry[V] {
	return &coordinatorRegistry[V]{items: make(map[string]*keyCoordinator[V])}
}

func newKeyCoordinator[V any]() *keyCoordinator[V] {
	coordinator := &keyCoordinator[V]{token: make(chan struct{}, 1)}
	coordinator.token <- struct{}{}
	return coordinator
}

func (r *coordinatorRegistry[V]) acquire(key string) *keyCoordinator[V] {
	r.mu.Lock()
	defer r.mu.Unlock()
	coordinator := r.items[key]
	if coordinator == nil {
		coordinator = newKeyCoordinator[V]()
		r.items[key] = coordinator
	}
	coordinator.mu.Lock()
	coordinator.externalRef++
	coordinator.mu.Unlock()
	return coordinator
}

func (r *coordinatorRegistry[V]) release(key string, coordinator *keyCoordinator[V]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.items[key] != coordinator {
		return
	}
	coordinator.mu.Lock()
	if coordinator.externalRef <= 0 {
		coordinator.mu.Unlock()
		panic("redisvalue: coordinator external reference underflow")
	}
	coordinator.externalRef--
	idle := coordinator.idleLocked()
	coordinator.mu.Unlock()
	if !idle {
		return
	}
	if r.beforeRetire != nil {
		hook := r.beforeRetire
		r.beforeRetire = nil
		hook()
	}
	delete(r.items, key)
}

func (r *coordinatorRegistry[V]) active() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

func (c *keyCoordinator[V]) idleLocked() bool {
	return c.externalRef == 0 && c.tokenUsers == 0 && c.flight == nil
}

func (c *keyCoordinator[V]) acquireToken(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	c.mu.Lock()
	c.tokenUsers++
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		c.mu.Lock()
		c.tokenUsers--
		c.mu.Unlock()
		return ctx.Err()
	case <-c.token:
		return nil
	}
}

func (c *keyCoordinator[V]) releaseToken() {
	c.mu.Lock()
	if c.tokenUsers <= 0 {
		c.mu.Unlock()
		panic("redisvalue: coordinator token reference underflow")
	}
	c.tokenUsers--
	c.mu.Unlock()
	c.token <- struct{}{}
}

func (c *keyCoordinator[V]) joinFlight() (*loadFlight[V], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flight != nil && !c.flight.published {
		c.flight.participants.Add(1)
		return c.flight, false
	}
	c.nextFlight++
	flight := &loadFlight[V]{
		generation: c.nextFlight,
		done:       make(chan struct{}),
	}
	flight.participants.Store(1)
	c.flight = flight
	return flight, true
}

func (c *keyCoordinator[V]) publishFlight(flight *loadFlight[V], value V, err error) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flight != flight || flight.published {
		return false
	}
	flight.value = value
	flight.err = err
	flight.published = true
	c.flight = nil
	flight.participants.Add(-1)
	close(flight.done)
	return true
}

func (c *keyCoordinator[V]) waitFlight(ctx context.Context, flight *loadFlight[V]) (V, error) {
	ctx = normalizeContext(ctx)
	select {
	case <-flight.done:
		return c.consumePublishedFlight(flight)
	case <-ctx.Done():
		c.mu.Lock()
		defer c.mu.Unlock()
		if flight.published {
			flight.participants.Add(-1)
			return flight.value, flight.err
		}
		flight.participants.Add(-1)
		var zero V
		return zero, ctx.Err()
	}
}

func (c *keyCoordinator[V]) consumePublishedFlight(flight *loadFlight[V]) (V, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !flight.published {
		panic("redisvalue: consumed unpublished load flight")
	}
	flight.participants.Add(-1)
	return flight.value, flight.err
}
