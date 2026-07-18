package redisnear_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9/push"
)

const sensitiveMarker = "secret-token=issue-536"

type localInvalidator interface {
	InvalidateLocal(context.Context, string) error
	ClearLocal(context.Context) error
}

const (
	maxSpikeInvalidationKeys  = 64
	maxSpikePhysicalKeyBytes  = 2 << 10
	maxSpikeAggregateKeyBytes = 64 << 10

	defaultSpikeKeyCleanupTimeout = time.Second
	defaultSpikeRepairTimeout     = 250 * time.Millisecond
)

var errRESP3InvalidationRejected = errors.New("resp3 invalidation rejected")

type observationReason string

const (
	observationReasonShape               observationReason = "shape"
	observationReasonType                observationReason = "type"
	observationReasonKeyCount            observationReason = "key-count"
	observationReasonKeySize             observationReason = "key-size"
	observationReasonAggregateSize       observationReason = "aggregate-size"
	observationReasonDuplicate           observationReason = "duplicate"
	observationReasonUnknownKey          observationReason = "unknown-key"
	observationReasonLocalCleanup        observationReason = "local-cleanup"
	observationReasonRepairFailed        observationReason = "repair-failed"
	observationReasonCleanupTimeout      observationReason = "cleanup-timeout"
	observationReasonShutdown            observationReason = "shutdown"
	observationReasonObservationOverflow observationReason = "observation-overflow"
)

type invalidationObservation struct {
	success  bool
	global   bool
	count    int
	reason   observationReason
	repaired bool
}

type spikeHandler struct {
	root              context.Context
	local             localInvalidator
	allowed           map[string]string
	keyCleanupTimeout time.Duration
	repairTimeout     time.Duration
	events            chan invalidationObservation
	overflow          atomic.Bool
	gate              callbackGate
}

var _ push.NotificationHandler = (*spikeHandler)(nil)

type callbackGate struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

func (g *callbackGate) begin() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return false
	}
	g.wg.Add(1)
	return true
}

func (g *callbackGate) close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

func (g *callbackGate) done() {
	g.wg.Done()
}

func (g *callbackGate) wait() {
	g.wg.Wait()
}

func newSpikeHandler(
	root context.Context,
	local localInvalidator,
	allowed map[string]string,
	events chan invalidationObservation,
) *spikeHandler {
	if root == nil {
		root = context.Background()
	}
	allowedCopy := make(map[string]string, len(allowed))
	for physical, logical := range allowed {
		allowedCopy[physical] = logical
	}

	return &spikeHandler{
		root:              root,
		local:             local,
		allowed:           allowedCopy,
		keyCleanupTimeout: defaultSpikeKeyCleanupTimeout,
		repairTimeout:     defaultSpikeRepairTimeout,
		events:            events,
	}
}

func (h *spikeHandler) HandlePushNotification(
	_ context.Context,
	_ push.NotificationHandlerContext,
	notification []interface{},
) error {
	if !h.gate.begin() {
		h.record(invalidationObservation{reason: observationReasonShutdown})
		return errRESP3InvalidationRejected
	}
	defer h.gate.done()

	if len(notification) != 2 {
		return h.reject(observationReasonShape, false)
	}
	kind, ok := notification[0].(string)
	if !ok || kind != "invalidate" {
		return h.reject(observationReasonType, false)
	}

	if notification[1] == nil {
		return h.clearAll()
	}

	physicalKeys, ok := notification[1].([]interface{})
	if !ok {
		return h.reject(observationReasonType, false)
	}
	logicalKeys, reason := h.validateKeys(physicalKeys)
	if reason != "" {
		return h.reject(reason, false)
	}

	cleanupCtx, cancelCleanup := context.WithTimeout(h.rootContext(), h.keyTimeout())
	defer cancelCleanup()
	for _, logicalKey := range logicalKeys {
		if err := h.local.InvalidateLocal(cleanupCtx, logicalKey); err != nil {
			return h.repairAfterKeyFailure(cleanupCtx)
		}
	}

	if !h.record(invalidationObservation{success: true, count: len(logicalKeys)}) {
		return errRESP3InvalidationRejected
	}
	return nil
}

func (h *spikeHandler) validateKeys(physicalKeys []interface{}) ([]string, observationReason) {
	if len(physicalKeys) == 0 || len(physicalKeys) > maxSpikeInvalidationKeys {
		return nil, observationReasonKeyCount
	}

	logicalKeys := make([]string, 0, len(physicalKeys))
	seen := make(map[string]struct{}, len(physicalKeys))
	aggregateBytes := 0
	for _, rawKey := range physicalKeys {
		physicalKey, ok := rawKey.(string)
		if !ok {
			return nil, observationReasonType
		}
		if len(physicalKey) == 0 || len(physicalKey) > maxSpikePhysicalKeyBytes {
			return nil, observationReasonKeySize
		}
		aggregateBytes += len(physicalKey)
		if aggregateBytes > maxSpikeAggregateKeyBytes {
			return nil, observationReasonAggregateSize
		}
		if _, duplicate := seen[physicalKey]; duplicate {
			return nil, observationReasonDuplicate
		}
		seen[physicalKey] = struct{}{}

		logicalKey, allowed := h.allowed[physicalKey]
		if !allowed {
			return nil, observationReasonUnknownKey
		}
		logicalKeys = append(logicalKeys, logicalKey)
	}
	return logicalKeys, ""
}

func (h *spikeHandler) clearAll() error {
	cleanupCtx, cancelCleanup := context.WithTimeout(h.rootContext(), h.keyTimeout())
	defer cancelCleanup()

	if err := h.local.ClearLocal(cleanupCtx); err != nil {
		reason := observationReasonLocalCleanup
		if cleanupCtx.Err() != nil {
			reason = observationReasonCleanupTimeout
		}
		return h.reject(reason, false)
	}
	if !h.record(invalidationObservation{success: true, global: true}) {
		return errRESP3InvalidationRejected
	}
	return nil
}

func (h *spikeHandler) repairAfterKeyFailure(cleanupCtx context.Context) error {
	cleanupTimedOut := cleanupCtx.Err() != nil
	repairCtx, cancelRepair := context.WithTimeout(h.rootContext(), h.repairDuration())
	defer cancelRepair()

	repairErr := h.local.ClearLocal(repairCtx)
	repaired := repairErr == nil && repairCtx.Err() == nil
	reason := observationReasonLocalCleanup
	if cleanupTimedOut {
		reason = observationReasonCleanupTimeout
	} else if !repaired {
		reason = observationReasonRepairFailed
	}
	return h.reject(reason, repaired)
}

func (h *spikeHandler) reject(reason observationReason, repaired bool) error {
	h.record(invalidationObservation{reason: reason, repaired: repaired})
	return errRESP3InvalidationRejected
}

func (h *spikeHandler) record(observation invalidationObservation) bool {
	select {
	case h.events <- observation:
		return true
	default:
		h.overflow.Store(true)
		return false
	}
}

func (h *spikeHandler) rootContext() context.Context {
	if h.root == nil {
		return context.Background()
	}
	return h.root
}

func (h *spikeHandler) keyTimeout() time.Duration {
	if h.keyCleanupTimeout <= 0 {
		return defaultSpikeKeyCleanupTimeout
	}
	return h.keyCleanupTimeout
}

func (h *spikeHandler) repairDuration() time.Duration {
	if h.repairTimeout <= 0 {
		return defaultSpikeRepairTimeout
	}
	return h.repairTimeout
}

type fakeLocalInvalidator struct {
	mu           sync.Mutex
	invalidated  []string
	clearCalls   int
	invalidateFn func(context.Context, string) error
	clearFn      func(context.Context) error
}

func (f *fakeLocalInvalidator) InvalidateLocal(ctx context.Context, key string) error {
	f.mu.Lock()
	f.invalidated = append(f.invalidated, key)
	fn := f.invalidateFn
	f.mu.Unlock()

	if fn == nil {
		return nil
	}
	return fn(ctx, key)
}

func (f *fakeLocalInvalidator) ClearLocal(ctx context.Context) error {
	f.mu.Lock()
	f.clearCalls++
	fn := f.clearFn
	f.mu.Unlock()

	if fn == nil {
		return nil
	}
	return fn(ctx)
}

func (f *fakeLocalInvalidator) calls() ([]string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.invalidated), f.clearCalls
}

func TestRESP3TrackingSpikeHandlerRejectsUnsafePayloadsWithoutDisclosure(t *testing.T) {
	oversizedKey := strings.Repeat("k", maxSpikePhysicalKeyBytes+1)
	aggregateKeys := make([]interface{}, 33)
	aggregateAllowed := make(map[string]string, len(aggregateKeys))
	for i := range aggregateKeys {
		physical := fmt.Sprintf("%02d", i) + strings.Repeat("a", maxSpikePhysicalKeyBytes-2)
		aggregateKeys[i] = physical
		aggregateAllowed[physical] = fmt.Sprintf("logical-%02d", i)
	}

	tooManyKeys := make([]interface{}, maxSpikeInvalidationKeys+1)
	for i := range tooManyKeys {
		tooManyKeys[i] = "physical-one"
	}

	tests := []struct {
		name         string
		notification []interface{}
		reason       observationReason
		allowed      map[string]string
	}{
		{name: "wrong arity zero", notification: []interface{}{}, reason: observationReasonShape},
		{name: "wrong arity one", notification: []interface{}{"invalidate"}, reason: observationReasonShape},
		{name: "wrong arity three", notification: []interface{}{"invalidate", nil, sensitiveMarker}, reason: observationReasonShape},
		{name: "non-string notification type", notification: []interface{}{7, nil}, reason: observationReasonType},
		{name: "foreign notification type", notification: []interface{}{sensitiveMarker, nil}, reason: observationReasonType},
		{name: "scalar key collection", notification: []interface{}{"invalidate", sensitiveMarker}, reason: observationReasonType},
		{name: "map key collection", notification: []interface{}{"invalidate", map[string]string{sensitiveMarker: sensitiveMarker}}, reason: observationReasonType},
		{name: "empty key list", notification: []interface{}{"invalidate", []interface{}{}}, reason: observationReasonKeyCount},
		{name: "integer key", notification: []interface{}{"invalidate", []interface{}{7}}, reason: observationReasonType},
		{name: "boolean key", notification: []interface{}{"invalidate", []interface{}{true}}, reason: observationReasonType},
		{name: "nil key", notification: []interface{}{"invalidate", []interface{}{nil}}, reason: observationReasonType},
		{name: "empty key", notification: []interface{}{"invalidate", []interface{}{string("")}}, reason: observationReasonKeySize},
		{name: "too many keys", notification: []interface{}{"invalidate", tooManyKeys}, reason: observationReasonKeyCount},
		{name: "oversized key", notification: []interface{}{"invalidate", []interface{}{oversizedKey}}, reason: observationReasonKeySize, allowed: map[string]string{oversizedKey: "logical"}},
		{name: "oversized aggregate", notification: []interface{}{"invalidate", aggregateKeys}, reason: observationReasonAggregateSize, allowed: aggregateAllowed},
		{name: "duplicate key", notification: []interface{}{"invalidate", []interface{}{"physical-one", "physical-one"}}, reason: observationReasonDuplicate},
		{name: "foreign key", notification: []interface{}{"invalidate", []interface{}{sensitiveMarker}}, reason: observationReasonUnknownKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := map[string]string{"physical-one": "logical-one"}
			for physical, logical := range tt.allowed {
				allowed[physical] = logical
			}
			local := &fakeLocalInvalidator{}
			events := make(chan invalidationObservation, 1)
			handler := newSpikeHandler(context.Background(), local, allowed, events)

			err := handleSpikeNotification(handler, tt.notification)

			assertRejectedWithoutDisclosure(t, err)
			keys, clears := local.calls()
			if len(keys) != 0 || clears != 0 {
				t.Fatalf("local calls = keys %v, clears %d; want none", keys, clears)
			}
			assertFailureObservation(t, events, invalidationObservation{reason: tt.reason})
			if handler.overflow.Load() {
				t.Fatal("overflow = true, want false")
			}
		})
	}
}

func TestRESP3TrackingSpikeHandlerProcessesBoundedMultiKeyPayload(t *testing.T) {
	t.Run("ordered exact allowlist lookup", func(t *testing.T) {
		allowed := map[string]string{
			"physical-one": "logical-one",
			"physical-two": "logical-two",
		}
		local := &fakeLocalInvalidator{}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(context.Background(), local, allowed, events)
		allowed["physical-one"] = sensitiveMarker
		delete(allowed, "physical-two")

		err := handleSpikeNotification(handler, []interface{}{
			"invalidate",
			[]interface{}{"physical-one", "physical-two"},
		})

		if err != nil {
			t.Fatalf("HandlePushNotification() error = %v", err)
		}
		keys, clears := local.calls()
		if !slices.Equal(keys, []string{"logical-one", "logical-two"}) || clears != 0 {
			t.Fatalf("local calls = keys %v, clears %d", keys, clears)
		}
		observation := requireSingleObservation(t, events)
		want := invalidationObservation{success: true, count: 2}
		if observation != want {
			t.Fatalf("observation = %+v, want %+v", observation, want)
		}
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})

	t.Run("accepts exact key count limit", func(t *testing.T) {
		physicalKeys := make([]interface{}, maxSpikeInvalidationKeys)
		allowed := make(map[string]string, len(physicalKeys))
		wantKeys := make([]string, len(physicalKeys))
		for i := range physicalKeys {
			physical := fmt.Sprintf("physical-%02d", i)
			logical := fmt.Sprintf("logical-%02d", i)
			physicalKeys[i] = physical
			allowed[physical] = logical
			wantKeys[i] = logical
		}

		assertSuccessfulKeyInvalidation(t, physicalKeys, allowed, wantKeys)
	})

	t.Run("accepts exact per-key byte limit", func(t *testing.T) {
		physical := strings.Repeat("k", maxSpikePhysicalKeyBytes)
		assertSuccessfulKeyInvalidation(
			t,
			[]interface{}{physical},
			map[string]string{physical: "logical"},
			[]string{"logical"},
		)
	})

	t.Run("accepts exact aggregate byte limit", func(t *testing.T) {
		const keyBytes = maxSpikeAggregateKeyBytes / maxSpikeInvalidationKeys
		physicalKeys := make([]interface{}, maxSpikeInvalidationKeys)
		allowed := make(map[string]string, len(physicalKeys))
		wantKeys := make([]string, len(physicalKeys))
		for i := range physicalKeys {
			physical := fmt.Sprintf("%02d", i) + strings.Repeat("a", keyBytes-2)
			logical := fmt.Sprintf("logical-%02d", i)
			physicalKeys[i] = physical
			allowed[physical] = logical
			wantKeys[i] = logical
		}

		assertSuccessfulKeyInvalidation(t, physicalKeys, allowed, wantKeys)
	})

	t.Run("global invalidation clears local", func(t *testing.T) {
		local := &fakeLocalInvalidator{}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(context.Background(), local, nil, events)

		err := handleSpikeNotification(handler, []interface{}{"invalidate", nil})

		if err != nil {
			t.Fatalf("HandlePushNotification() error = %v", err)
		}
		keys, clears := local.calls()
		if len(keys) != 0 || clears != 1 {
			t.Fatalf("local calls = keys %v, clears %d; want one clear", keys, clears)
		}
		observation := requireSingleObservation(t, events)
		want := invalidationObservation{success: true, global: true}
		if observation != want {
			t.Fatalf("observation = %+v, want %+v", observation, want)
		}
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})
}

func TestRESP3TrackingSpikeHandlerReportsLocalCleanupFailure(t *testing.T) {
	allowed := map[string]string{
		"physical-one":   "logical-one",
		"physical-two":   "logical-two",
		"physical-three": "logical-three",
	}
	threeKeys := []interface{}{
		"invalidate",
		[]interface{}{"physical-one", "physical-two", "physical-three"},
	}

	t.Run("middle key failure repairs with full clear", func(t *testing.T) {
		local := &fakeLocalInvalidator{
			invalidateFn: func(_ context.Context, key string) error {
				if key == "logical-two" {
					return errors.New(sensitiveMarker)
				}
				return nil
			},
		}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(context.Background(), local, allowed, events)

		err := handleSpikeNotification(handler, threeKeys)

		assertRejectedWithoutDisclosure(t, err)
		assertFailureCalls(t, local, []string{"logical-one", "logical-two"}, 1)
		assertFailureObservation(t, events, invalidationObservation{
			reason:   observationReasonLocalCleanup,
			repaired: true,
		})
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})

	t.Run("middle key and repair failure stay redacted", func(t *testing.T) {
		local := &fakeLocalInvalidator{
			invalidateFn: func(_ context.Context, key string) error {
				if key == "logical-two" {
					return errors.New(sensitiveMarker)
				}
				return nil
			},
			clearFn: func(context.Context) error {
				return errors.New(sensitiveMarker)
			},
		}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(context.Background(), local, allowed, events)

		err := handleSpikeNotification(handler, threeKeys)

		assertRejectedWithoutDisclosure(t, err)
		assertFailureCalls(t, local, []string{"logical-one", "logical-two"}, 1)
		assertFailureObservation(t, events, invalidationObservation{reason: observationReasonRepairFailed})
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})

	t.Run("global clear failure stays redacted", func(t *testing.T) {
		local := &fakeLocalInvalidator{
			clearFn: func(context.Context) error {
				return errors.New(sensitiveMarker)
			},
		}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(context.Background(), local, nil, events)

		err := handleSpikeNotification(handler, []interface{}{"invalidate", nil})

		assertRejectedWithoutDisclosure(t, err)
		assertFailureCalls(t, local, nil, 1)
		assertFailureObservation(t, events, invalidationObservation{reason: observationReasonLocalCleanup})
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})

	t.Run("canceled root stays redacted", func(t *testing.T) {
		root, cancel := context.WithCancel(context.Background())
		cancel()
		local := &fakeLocalInvalidator{
			invalidateFn: func(ctx context.Context, _ string) error {
				return fmt.Errorf("%s: %w", sensitiveMarker, ctx.Err())
			},
			clearFn: func(ctx context.Context) error {
				return fmt.Errorf("%s: %w", sensitiveMarker, ctx.Err())
			},
		}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(root, local, allowed, events)

		err := handleSpikeNotification(handler, []interface{}{"invalidate", []interface{}{"physical-one"}})

		assertRejectedWithoutDisclosure(t, err)
		assertFailureCalls(t, local, []string{"logical-one"}, 1)
		assertFailureObservation(t, events, invalidationObservation{reason: observationReasonCleanupTimeout})
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})

	t.Run("cleanup timeout is bounded and repaired independently", func(t *testing.T) {
		type contextObservation struct {
			err         error
			deadline    time.Time
			observedAt  time.Time
			hasDeadline bool
		}
		repairContexts := make(chan contextObservation, 1)
		local := &fakeLocalInvalidator{
			invalidateFn: func(ctx context.Context, _ string) error {
				<-ctx.Done()
				return fmt.Errorf("%s: %w", sensitiveMarker, ctx.Err())
			},
			clearFn: func(ctx context.Context) error {
				deadline, hasDeadline := ctx.Deadline()
				repairContexts <- contextObservation{
					err:         ctx.Err(),
					deadline:    deadline,
					observedAt:  time.Now(),
					hasDeadline: hasDeadline,
				}
				return nil
			},
		}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(context.Background(), local, allowed, events)
		handler.keyCleanupTimeout = 100 * time.Millisecond
		done := make(chan error, 1)
		go func() {
			done <- handleSpikeNotification(handler, []interface{}{"invalidate", []interface{}{"physical-one"}})
		}()

		var err error
		select {
		case err = <-done:
		case <-time.After(time.Second):
			t.Fatal("HandlePushNotification did not respect cleanup timeout")
		}
		assertRejectedWithoutDisclosure(t, err)
		assertFailureCalls(t, local, []string{"logical-one"}, 1)
		assertFailureObservation(t, events, invalidationObservation{
			reason:   observationReasonCleanupTimeout,
			repaired: true,
		})
		repairContext := <-repairContexts
		if repairContext.err != nil {
			t.Fatalf("repair context was not live on entry: %v", repairContext.err)
		}
		if !repairContext.hasDeadline {
			t.Fatal("repair context has no deadline")
		}
		remaining := repairContext.deadline.Sub(repairContext.observedAt)
		if remaining <= 0 || remaining > handler.repairTimeout || remaining < handler.repairTimeout/2 {
			t.Fatalf("repair context remaining budget = %s, want live budget near %s", remaining, handler.repairTimeout)
		}
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})

	t.Run("repair timeout is bounded and redacted", func(t *testing.T) {
		local := &fakeLocalInvalidator{
			invalidateFn: func(context.Context, string) error {
				return errors.New(sensitiveMarker)
			},
			clearFn: func(ctx context.Context) error {
				<-ctx.Done()
				return fmt.Errorf("%s: %w", sensitiveMarker, ctx.Err())
			},
		}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(context.Background(), local, allowed, events)
		handler.repairTimeout = 100 * time.Millisecond
		done := make(chan error, 1)
		go func() {
			done <- handleSpikeNotification(handler, []interface{}{"invalidate", []interface{}{"physical-one"}})
		}()

		var err error
		select {
		case err = <-done:
		case <-time.After(time.Second):
			t.Fatal("HandlePushNotification did not respect repair timeout")
		}
		assertRejectedWithoutDisclosure(t, err)
		assertFailureCalls(t, local, []string{"logical-one"}, 1)
		assertFailureObservation(t, events, invalidationObservation{reason: observationReasonRepairFailed})
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})

	t.Run("closed gate rejects before local cleanup", func(t *testing.T) {
		local := &fakeLocalInvalidator{}
		events := make(chan invalidationObservation, 1)
		handler := newSpikeHandler(context.Background(), local, allowed, events)
		handler.gate.close()

		err := handleSpikeNotification(handler, []interface{}{"invalidate", []interface{}{"physical-one"}})

		assertRejectedWithoutDisclosure(t, err)
		assertFailureCalls(t, local, nil, 0)
		assertFailureObservation(t, events, invalidationObservation{reason: observationReasonShutdown})
		if handler.overflow.Load() {
			t.Fatal("overflow = true, want false")
		}
	})
}

func TestRESP3TrackingSpikeHandlerOverflowDoesNotBlock(t *testing.T) {
	local := &fakeLocalInvalidator{}
	events := make(chan invalidationObservation, 1)
	handler := newSpikeHandler(
		context.Background(),
		local,
		map[string]string{"physical-one": "logical-one"},
		events,
	)
	notification := []interface{}{"invalidate", []interface{}{"physical-one"}}
	if err := handleSpikeNotification(handler, notification); err != nil {
		t.Fatalf("first HandlePushNotification() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- handleSpikeNotification(handler, notification)
	}()

	select {
	case err := <-done:
		assertRejectedWithoutDisclosure(t, err)
	case <-time.After(time.Second):
		t.Fatal("second HandlePushNotification blocked on observation delivery")
	}
	if !handler.overflow.Load() {
		t.Fatal("overflow = false, want true")
	}
	observation := requireSingleObservation(t, events)
	if !observation.success || observation.count != 1 {
		t.Fatalf("retained observation = %+v, want first success", observation)
	}
	keys, clears := local.calls()
	if !slices.Equal(keys, []string{"logical-one", "logical-one"}) || clears != 0 {
		t.Fatalf("local calls = keys %v, clears %d", keys, clears)
	}
}

func handleSpikeNotification(handler *spikeHandler, notification []interface{}) error {
	return handler.HandlePushNotification(
		context.Background(),
		push.NotificationHandlerContext{},
		notification,
	)
}

func assertRejectedWithoutDisclosure(t *testing.T, err error) {
	t.Helper()

	if err != errRESP3InvalidationRejected {
		t.Fatalf("error = %v, want exact rejection sentinel", err)
	}
	if !errors.Is(err, errRESP3InvalidationRejected) {
		t.Fatalf("errors.Is(%v, errRESP3InvalidationRejected) = false", err)
	}
	if strings.Contains(err.Error(), sensitiveMarker) {
		t.Fatalf("error disclosed sensitive marker: %v", err)
	}
}

func assertFailureCalls(t *testing.T, local *fakeLocalInvalidator, wantKeys []string, wantClears int) {
	t.Helper()

	keys, clears := local.calls()
	if !slices.Equal(keys, wantKeys) || clears != wantClears {
		t.Fatalf("local calls = keys %v, clears %d; want keys %v, clears %d", keys, clears, wantKeys, wantClears)
	}
}

func assertSuccessfulKeyInvalidation(
	t *testing.T,
	physicalKeys []interface{},
	allowed map[string]string,
	wantKeys []string,
) {
	t.Helper()

	local := &fakeLocalInvalidator{}
	events := make(chan invalidationObservation, 1)
	handler := newSpikeHandler(context.Background(), local, allowed, events)
	err := handleSpikeNotification(handler, []interface{}{"invalidate", physicalKeys})
	if err != nil {
		t.Fatalf("HandlePushNotification() error = %v", err)
	}
	keys, clears := local.calls()
	if !slices.Equal(keys, wantKeys) || clears != 0 {
		t.Fatalf("local calls = keys %v, clears %d; want keys %v, clears 0", keys, clears, wantKeys)
	}
	wantObservation := invalidationObservation{success: true, count: len(wantKeys)}
	if observation := requireSingleObservation(t, events); observation != wantObservation {
		t.Fatalf("observation = %+v, want %+v", observation, wantObservation)
	}
	if handler.overflow.Load() {
		t.Fatal("overflow = true, want false")
	}
}

func assertFailureObservation(
	t *testing.T,
	events <-chan invalidationObservation,
	want invalidationObservation,
) {
	t.Helper()

	observation := requireSingleObservation(t, events)
	if observation != want {
		t.Fatalf("observation = %+v, want %+v", observation, want)
	}
	if strings.Contains(fmt.Sprint(observation), sensitiveMarker) {
		t.Fatalf("observation disclosed sensitive marker: %+v", observation)
	}
}

func requireSingleObservation(t *testing.T, events <-chan invalidationObservation) invalidationObservation {
	t.Helper()

	select {
	case observation := <-events:
		select {
		case extra := <-events:
			t.Fatalf("extra observation = %+v", extra)
		default:
		}
		return observation
	default:
		t.Fatal("missing observation")
		return invalidationObservation{}
	}
}
