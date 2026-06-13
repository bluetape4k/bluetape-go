package jwt

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	golangjwt "github.com/golang-jwt/jwt/v5"
)

type fakeDistributedRepository struct {
	mu         sync.Mutex
	keys       []*KeyChain
	err        error
	seenCtx    []context.Context
	findHits   int
	rotateHits int
	forceHits  int
	deleteHits int
	capacity   int
	malicious  bool
}

var _ DistributedKeyChainRepository = (*fakeDistributedRepository)(nil)

func (r *fakeDistributedRepository) Current(ctx context.Context, now time.Time) (*KeyChain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seenCtx = append(r.seenCtx, ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.err != nil {
		return nil, r.err
	}
	for _, key := range r.keys {
		if !key.Expired(now) {
			return key, nil
		}
	}
	return nil, KeyError{Kind: ErrKeyNotFound, Err: errorsNew("current key not found")}
}

func (r *fakeDistributedRepository) Find(ctx context.Context, kid string, now time.Time) (*KeyChain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seenCtx = append(r.seenCtx, ctx)
	r.findHits++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.err != nil {
		return nil, r.err
	}
	if kid == "" {
		return nil, KeyError{Kind: ErrKeyNotFound, Err: errorsNew("kid is required")}
	}
	for _, key := range r.keys {
		if key.KID() == kid {
			if key.Expired(now) {
				return nil, KeyError{Kind: ErrInvalidKey, KID: kid, Err: errorsNew("key expired")}
			}
			return key, nil
		}
	}
	return nil, KeyError{Kind: ErrKeyNotFound, KID: kid, Err: errorsNew("key not found")}
}

func (r *fakeDistributedRepository) Rotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seenCtx = append(r.seenCtx, ctx)
	r.rotateHits++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.err != nil {
		return nil, r.err
	}
	for _, key := range r.keys {
		if !key.Expired(now) {
			return key, nil
		}
	}
	key, err := create()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.prependLocked(key)
	return key, nil
}

func (r *fakeDistributedRepository) ForcedRotate(ctx context.Context, create func() (*KeyChain, error), _ time.Time) (*KeyChain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seenCtx = append(r.seenCtx, ctx)
	r.forceHits++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.malicious && len(r.keys) > 0 {
		return r.keys[0], nil
	}
	key, err := create()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.prependLocked(key)
	return key, nil
}

func (r *fakeDistributedRepository) DeleteAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seenCtx = append(r.seenCtx, ctx)
	r.deleteHits++
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.err != nil {
		return r.err
	}
	r.keys = nil
	return nil
}

func (r *fakeDistributedRepository) prependLocked(key *KeyChain) {
	r.keys = append([]*KeyChain{key}, r.keys...)
	capacity := r.capacity
	if capacity == 0 {
		capacity = defaultRepositorySize
	}
	if len(r.keys) > capacity {
		r.keys = r.keys[:capacity]
	}
}

func (r *fakeDistributedRepository) snapshot() (rotateHits int, forceHits int, deleteHits int, keys []*KeyChain, seen []context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rotateHits, r.forceHits, r.deleteHits, append([]*KeyChain(nil), r.keys...), append([]context.Context(nil), r.seenCtx...)
}

func TestNewDistributedHMACProviderBootstrapsCurrentKey(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}

	provider, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("distributed-1")),
		WithEntropy(repeatingReader('d')),
	)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider() error = %v", err)
	}
	current, err := provider.CurrentKeyChainContext(context.Background())
	if err != nil {
		t.Fatalf("CurrentKeyChainContext() error = %v", err)
	}
	if current.KID() != "distributed-1" || current.Algorithm() != HS256 {
		t.Fatalf("current key = %q/%q", current.KID(), current.Algorithm())
	}
	rotateHits, _, _, keys, seen := repo.snapshot()
	if rotateHits != 1 {
		t.Fatalf("Rotate hits = %d, want 1", rotateHits)
	}
	if len(keys) != 1 {
		t.Fatalf("keys = %d, want 1", len(keys))
	}
	if len(seen) == 0 || seen[0] == nil {
		t.Fatalf("constructor did not pass caller context")
	}
}

func TestDistributedProviderComposeAndParseAcrossInstances(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	signer, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("shared-1")),
		WithEntropy(repeatingReader('s')),
	)
	if err != nil {
		t.Fatalf("signer constructor error = %v", err)
	}
	parser, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("unused")),
		WithEntropy(repeatingReader('p')),
	)
	if err != nil {
		t.Fatalf("parser constructor error = %v", err)
	}

	token, err := signer.ComposeContext(context.Background(), WithSubject("account-42"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext() error = %v", err)
	}
	reader, err := parser.ParseContext(context.Background(), token, WithExpectedSubject("account-42"), WithParseClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("ParseContext() error = %v", err)
	}
	if reader.Kid() != "shared-1" || reader.Algorithm() != HS256 {
		t.Fatalf("reader = %q/%q", reader.Kid(), reader.Algorithm())
	}
}

func TestDistributedProviderParsesRetainedKeyAfterForcedRotate(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{capacity: 2}
	provider, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(sequenceKID("retained")),
		WithEntropy(repeatingReader('r')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	oldToken, err := provider.ComposeContext(context.Background(), WithClaim("phase", "old"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext(old) error = %v", err)
	}
	oldKey, err := provider.CurrentKeyChainContext(context.Background())
	if err != nil {
		t.Fatalf("CurrentKeyChainContext(old) error = %v", err)
	}
	if _, err := provider.ForcedRotateContext(context.Background()); err != nil {
		t.Fatalf("ForcedRotateContext() error = %v", err)
	}
	reader, err := provider.ParseContext(context.Background(), oldToken, WithParseClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("ParseContext(old retained) error = %v", err)
	}
	if reader.Kid() != oldKey.KID() {
		t.Fatalf("retained kid = %q, want %q", reader.Kid(), oldKey.KID())
	}
}

func TestDistributedProviderConstructorRejectsNilContext(t *testing.T) {
	//nolint:staticcheck // This test intentionally verifies nil context rejection.
	_, err := NewDistributedHMACProvider(nil, &fakeDistributedRepository{}, HS256)
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestDistributedProviderConstructorRejectsNilRepository(t *testing.T) {
	_, err := NewDistributedHMACProvider(context.Background(), nil, HS256)
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestDistributedProviderConstructorRejectsTypedNilRepository(t *testing.T) {
	var repo *fakeDistributedRepository
	_, err := NewDistributedHMACProvider(context.Background(), repo, HS256)
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestDistributedProviderConstructorPreservesCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewDistributedHMACProvider(ctx, &fakeDistributedRepository{}, HS256)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestDistributedProviderConstructorPreservesExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := NewDistributedHMACProvider(ctx, &fakeDistributedRepository{}, HS256)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestDistributedProviderDoesNotExposeContextFreeProviderMethods(t *testing.T) {
	typ := reflect.TypeOf((*DistributedProvider)(nil))
	for _, method := range []string{"Compose", "Parse", "TryParse", "CurrentKeyChain", "Rotate", "ForcedRotate", "FindKeyChain"} {
		if _, ok := typ.MethodByName(method); ok {
			t.Fatalf("DistributedProvider exposes context-free method %s", method)
		}
	}
	elem := typ.Elem()
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Field(i)
		if field.Anonymous && field.Type == reflect.TypeOf((*Provider)(nil)) {
			t.Fatalf("DistributedProvider anonymously embeds *Provider")
		}
	}
}

func TestDistributedProviderParseRejectsMissingKID(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("kid")),
		WithEntropy(repeatingReader('m')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	key, err := provider.CurrentKeyChainContext(context.Background())
	if err != nil {
		t.Fatalf("CurrentKeyChainContext() error = %v", err)
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	signed, err := token.SignedString(key.signingMaterial())
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := provider.ParseContext(context.Background(), signed, WithParseClock(func() time.Time { return now })); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseContext(missing kid) error = %v, want ErrInvalidToken", err)
	}
}

func TestDistributedProviderParseUnknownKIDReturnsKeyNotFound(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("known")),
		WithEntropy(repeatingReader('u')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	key, err := provider.CurrentKeyChainContext(context.Background())
	if err != nil {
		t.Fatalf("CurrentKeyChainContext() error = %v", err)
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	token.Header["kid"] = "unknown"
	signed, err := token.SignedString(key.signingMaterial())
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := provider.ParseContext(context.Background(), signed, WithParseClock(func() time.Time { return now })); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("ParseContext(unknown kid) error = %v, want ErrKeyNotFound", err)
	}
}

func TestDistributedProviderParseRejectsInvalidKIDBeforeRepositoryLookup(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("known")),
		WithEntropy(repeatingReader('v')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	key, err := provider.CurrentKeyChainContext(context.Background())
	if err != nil {
		t.Fatalf("CurrentKeyChainContext() error = %v", err)
	}

	for _, kid := range []string{strings.Repeat("a", maxKIDBytes+1), "bad\nkid"} {
		token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
		token.Header["kid"] = kid
		signed, err := token.SignedString(key.signingMaterial())
		if err != nil {
			t.Fatalf("SignedString() error = %v", err)
		}
		if _, err := provider.ParseContext(context.Background(), signed, WithParseClock(func() time.Time { return now })); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("ParseContext(invalid kid) error = %v, want ErrInvalidToken", err)
		}
	}
	if repo.findHits != 0 {
		t.Fatalf("repo Find calls = %d, want 0 for invalid kid", repo.findHits)
	}
}

func TestDistributedProviderParseExpiredRetainedKeyReturnsInvalidKey(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	key, err := newHMACKeyChain("expired", HS256, bytesOf('e', 32), now.Add(-2*time.Hour), time.Hour)
	if err != nil {
		t.Fatalf("newHMACKeyChain() error = %v", err)
	}
	repo := &fakeDistributedRepository{keys: []*KeyChain{key}}
	provider, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("fresh")),
		WithEntropy(repeatingReader('f')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	token.Header["kid"] = "expired"
	signed, err := token.SignedString(key.signingMaterial())
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := provider.ParseContext(context.Background(), signed, WithParseClock(func() time.Time { return now })); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("ParseContext(expired retained) error = %v, want ErrInvalidKey", err)
	}
}

func TestDistributedProviderRepositoryErrorsWrapCause(t *testing.T) {
	cause := errorsNew("redis unavailable")
	provider, err := NewDistributedHMACProvider(context.Background(), &fakeDistributedRepository{}, HS256,
		WithKeyIDGenerator(staticKID("cause")),
		WithEntropy(repeatingReader('c')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	provider.repo = &fakeDistributedRepository{err: cause}
	if _, err := provider.CurrentKeyChainContext(context.Background()); !errors.Is(err, cause) {
		t.Fatalf("CurrentKeyChainContext() error = %v, want cause", err)
	}
}

func TestDistributedProviderDeleteKeyChainsDelegatesToRepository(t *testing.T) {
	repo := &fakeDistributedRepository{}
	provider, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithKeyIDGenerator(staticKID("delete")),
		WithEntropy(repeatingReader('d')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	if err := provider.DeleteKeyChainsContext(context.Background()); err != nil {
		t.Fatalf("DeleteKeyChainsContext() error = %v", err)
	}
	_, _, deleteHits, keys, _ := repo.snapshot()
	if deleteHits != 1 {
		t.Fatalf("delete hits = %d, want 1", deleteHits)
	}
	if len(keys) != 0 {
		t.Fatalf("keys after delete = %d, want 0", len(keys))
	}
}

func TestDistributedProviderRSAAlgorithmValidation(t *testing.T) {
	if _, err := NewDistributedHMACProvider(context.Background(), &fakeDistributedRepository{}, RS256); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("HMAC constructor with RSA alg error = %v, want ErrInvalidOptions", err)
	}
	if _, err := NewDistributedRSAProvider(context.Background(), &fakeDistributedRepository{}, HS256); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("RSA constructor with HMAC alg error = %v, want ErrInvalidOptions", err)
	}
}

func TestDistributedProviderFixedLocalMigrationIsNotImplicit(t *testing.T) {
	typ := reflect.TypeOf((*DistributedProvider)(nil))
	for _, method := range []string{"ExportKeyChain", "ImportKeyChain", "SeedKeyChain", "LoadKeyChain", "NewFromKeyChain"} {
		if _, ok := typ.MethodByName(method); ok {
			t.Fatalf("DistributedProvider exposes implicit migration method %s", method)
		}
	}
}

func TestDistributedProviderGoroutineStressComposeParseAndRotate(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	repo := &fakeDistributedRepository{capacity: 256}
	provider, err := NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(sequenceKID("stress")),
		WithEntropy(repeatingReader('g')),
	)
	if err != nil {
		t.Fatalf("constructor error = %v", err)
	}
	retained, err := provider.ComposeContext(context.Background(), WithClaim("retained", "yes"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext(retained) error = %v", err)
	}
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 20,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t,
		func(ctx context.Context) error {
			token, err := provider.ComposeContext(ctx, WithClaim("n", "v"), WithExpiresAfter(time.Hour))
			if err != nil {
				return err
			}
			reader, err := provider.ParseContext(ctx, token, WithParseClock(func() time.Time { return now }))
			if err != nil {
				return err
			}
			if value, ok := reader.ClaimString("n"); !ok || value != "v" {
				return errorsNew("claim mismatch")
			}
			retainedReader, err := provider.ParseContext(ctx, retained, WithParseClock(func() time.Time { return now }))
			if err != nil {
				return err
			}
			if value, ok := retainedReader.ClaimString("retained"); !ok || value != "yes" {
				return errorsNew("retained claim mismatch")
			}
			return nil
		},
		func(ctx context.Context) error {
			_, err := provider.ForcedRotateContext(ctx)
			return err
		},
	)
}

func TestDistributedProviderRejectsWrongAlgorithmRepositoryKey(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	wrong, err := newHMACKeyChain("wrong", HS384, bytesOf('w', 48), now, time.Hour)
	if err != nil {
		t.Fatalf("newHMACKeyChain() error = %v", err)
	}
	repo := &fakeDistributedRepository{keys: []*KeyChain{wrong}, malicious: true}
	_, err = NewDistributedHMACProvider(context.Background(), repo, HS256,
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("fresh")),
		WithEntropy(repeatingReader('f')),
	)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("constructor error = %v, want ErrInvalidKey", err)
	}
}

func TestDistributedProviderRejectsWrongAlgorithmOnEveryRepositoryResult(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	wrong, err := newHMACKeyChain("wrong", HS384, bytesOf('w', 48), now, time.Hour)
	if err != nil {
		t.Fatalf("newHMACKeyChain() error = %v", err)
	}
	repo := &fakeDistributedRepository{keys: []*KeyChain{wrong}, malicious: true}
	provider := &DistributedProvider{
		provider: &Provider{algorithm: HS256, cfg: providerConfig{now: func() time.Time { return now }, keyTTL: time.Hour}},
		repo:     repo,
	}
	checks := []struct {
		name string
		call func() error
	}{
		{name: "current", call: func() error { _, err := provider.CurrentKeyChainContext(context.Background()); return err }},
		{name: "find", call: func() error { _, err := provider.FindKeyChainContext(context.Background(), "wrong"); return err }},
		{name: "rotate", call: func() error { _, err := provider.RotateContext(context.Background()); return err }},
		{name: "forced rotate", call: func() error { _, err := provider.ForcedRotateContext(context.Background()); return err }},
		{name: "compose", call: func() error { _, err := provider.ComposeContext(context.Background()); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("%s error = %v, want ErrInvalidKey", check.name, err)
			}
		})
	}

	token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	token.Header["kid"] = "wrong"
	signed, err := token.SignedString(bytesOf('w', 32))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := provider.ParseContext(context.Background(), signed, WithParseClock(func() time.Time { return now })); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("parse error = %v, want ErrInvalidKey", err)
	}
}
