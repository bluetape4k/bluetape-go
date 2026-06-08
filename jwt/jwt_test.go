package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	golangjwt "github.com/golang-jwt/jwt/v5"
)

func TestFixedHMACProviderComposesAndParsesClaims(t *testing.T) {
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	provider, err := NewFixedHMACProvider(HS256, bytesOf('a', 32),
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("kid-1")),
	)
	if err != nil {
		t.Fatalf("NewFixedHMACProvider() error = %v", err)
	}

	token, err := provider.Compose(
		WithIssuer("issuer"),
		WithSubject("subject"),
		WithAudience("api", "worker"),
		WithExpiresAfter(10*time.Minute),
		WithNotBefore(now.Add(-time.Minute)),
		WithJWTID("jwt-id"),
		WithHeader("env", "test"),
		WithHeader("trace", []any{"a", "b"}),
		WithClaim("role", "admin"),
		WithClaim("meta", map[string]any{"team": "core"}),
	)
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	reader, err := provider.Parse(token,
		WithExpectedIssuer("issuer"),
		WithExpectedSubject("subject"),
		WithExpectedAudience("worker"),
		WithExpirationRequired(),
		WithParseClock(func() time.Time { return now.Add(time.Minute) }),
	)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if reader.Kid() != "kid-1" {
		t.Fatalf("Kid() = %q", reader.Kid())
	}
	if reader.Algorithm() != HS256 {
		t.Fatalf("Algorithm() = %q", reader.Algorithm())
	}
	if reader.Issuer() != "issuer" || reader.Subject() != "subject" {
		t.Fatalf("issuer/subject = %q/%q", reader.Issuer(), reader.Subject())
	}
	if role, ok := reader.ClaimString("role"); !ok || role != "admin" {
		t.Fatalf("role = %q ok=%v", role, ok)
	}
	headerTrace, ok := reader.Header("trace")
	if !ok {
		t.Fatalf("Header(trace) missing")
	}
	headerTrace.([]any)[0] = "mutated"
	headerTraceAgain, _ := reader.Header("trace")
	if headerTraceAgain.([]any)[0] != "a" {
		t.Fatalf("Header() did not return isolated copy: %v", headerTraceAgain)
	}
	claimMeta, ok := reader.Claim("meta")
	if !ok {
		t.Fatalf("Claim(meta) missing")
	}
	claimMeta.(map[string]any)["team"] = "mutated"
	claimMetaAgain, _ := reader.Claim("meta")
	if claimMetaAgain.(map[string]any)["team"] != "core" {
		t.Fatalf("Claim() did not return isolated copy: %v", claimMetaAgain)
	}
	if issuedAt, ok := reader.IssuedAt(); !ok || !issuedAt.Equal(now) {
		t.Fatalf("IssuedAt() = %v ok=%v", issuedAt, ok)
	}
	if expiresAt, ok := reader.ExpiresAt(); !ok || !expiresAt.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("ExpiresAt() = %v ok=%v", expiresAt, ok)
	}
	if reader.IsExpired(now.Add(9 * time.Minute)) {
		t.Fatalf("IsExpired() before expiration = true")
	}
	if !reader.IsExpired(now.Add(10 * time.Minute)) {
		t.Fatalf("IsExpired() at expiration = false")
	}
	if ttl := reader.RemainingTTL(now.Add(4 * time.Minute)); ttl != 6*time.Minute {
		t.Fatalf("RemainingTTL() = %v", ttl)
	}
}

func TestFixedHMACRejectsWeakSecrets(t *testing.T) {
	tests := []struct {
		name      string
		alg       Algorithm
		secretLen int
	}{
		{name: "hs256 empty", alg: HS256, secretLen: 0},
		{name: "hs256 short", alg: HS256, secretLen: 31},
		{name: "hs384 short", alg: HS384, secretLen: 47},
		{name: "hs512 short", alg: HS512, secretLen: 63},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFixedHMACProvider(tt.alg, bytesOf('x', tt.secretLen), WithKeyIDGenerator(staticKID("kid")))
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("error = %v, want ErrInvalidKey", err)
			}
		})
	}
}

func TestFixedRSAProviderComposesAndParses(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	provider, err := NewFixedRSAProvider(RS256, privateKey, WithKeyIDGenerator(staticKID("rsa-1")))
	if err != nil {
		t.Fatalf("NewFixedRSAProvider() error = %v", err)
	}
	token, err := provider.Compose(WithSubject("subject"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	reader, err := provider.Parse(token, WithExpectedSubject("subject"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if reader.Kid() != "rsa-1" || reader.Algorithm() != RS256 {
		t.Fatalf("reader kid/alg = %q/%q", reader.Kid(), reader.Algorithm())
	}
}

func TestFixedRSAProviderRejectsWeakPrivateKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	_, err = NewFixedRSAProvider(RS256, privateKey, WithKeyIDGenerator(staticKID("weak-rsa")))
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("NewFixedRSAProvider() error = %v, want ErrInvalidKey", err)
	}
}

func TestFixedRSAProviderCopiesPrivateKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	provider, err := NewFixedRSAProvider(RS256, privateKey, WithKeyIDGenerator(staticKID("copied-rsa")))
	if err != nil {
		t.Fatalf("NewFixedRSAProvider() error = %v", err)
	}
	privateKey.N.SetInt64(3)
	privateKey.D.SetInt64(1)
	privateKey.Primes[0].SetInt64(3)

	token, err := provider.Compose(WithSubject("subject"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() after caller key mutation error = %v", err)
	}
	reader, err := provider.Parse(token, WithExpectedSubject("subject"))
	if err != nil {
		t.Fatalf("Parse() after caller key mutation error = %v", err)
	}
	if reader.Kid() != "copied-rsa" {
		t.Fatalf("Kid() = %q", reader.Kid())
	}
}

func TestReservedHeadersClaimsAndInboundHeadersAreRejected(t *testing.T) {
	provider, err := NewFixedHMACProvider(HS256, bytesOf('a', 32), WithKeyIDGenerator(staticKID("kid")))
	if err != nil {
		t.Fatalf("NewFixedHMACProvider() error = %v", err)
	}
	if _, err := provider.Compose(WithHeader("zip", "DEF")); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("reserved header error = %v", err)
	}
	if _, err := provider.Compose(WithClaim("exp", "bad")); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("reserved claim error = %v", err)
	}

	for _, header := range []string{"zip", "crit", "jku", "jwk", "x5u", "x5c"} {
		t.Run("inbound "+header, func(t *testing.T) {
			token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{
				"sub": "subject",
				"iat": golangjwt.NewNumericDate(time.Now()),
			})
			token.Header["kid"] = "kid"
			token.Header[header] = "unsupported"
			signed, err := token.SignedString(bytesOf('a', 32))
			if err != nil {
				t.Fatalf("SignedString() error = %v", err)
			}
			if _, err := provider.Parse(signed); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Parse() error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestParseFailureCasesAndTryParse(t *testing.T) {
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	provider, err := NewFixedHMACProvider(HS256, bytesOf('a', 32),
		WithClock(func() time.Time { return now }),
		WithKeyIDGenerator(staticKID("kid")),
	)
	if err != nil {
		t.Fatalf("NewFixedHMACProvider() error = %v", err)
	}

	expired, err := provider.Compose(WithExpiresAt(now.Add(-time.Minute)))
	if err != nil {
		t.Fatalf("Compose(expired) error = %v", err)
	}
	if _, err := provider.Parse(expired, WithParseClock(func() time.Time { return now })); !errors.Is(err, ErrExpiredToken) || !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired error = %v", err)
	}
	if reader, ok := provider.TryParse(expired, WithParseClock(func() time.Time { return now })); ok || reader != nil {
		t.Fatalf("TryParse(expired) = %v %v", reader, ok)
	}

	notBefore, err := provider.Compose(WithNotBefore(now.Add(time.Minute)), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose(nbf) error = %v", err)
	}
	if _, err := provider.Parse(notBefore, WithParseClock(func() time.Time { return now })); !errors.Is(err, ErrNotYetValid) || !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("nbf error = %v", err)
	}
	if _, err := provider.Parse(notBefore, WithParseClock(func() time.Time { return now }), WithLeeway(2*time.Minute)); err != nil {
		t.Fatalf("leeway parse error = %v", err)
	}

	wrongKeyProvider, err := NewFixedHMACProvider(HS256, bytesOf('b', 32), WithKeyIDGenerator(staticKID("kid")))
	if err != nil {
		t.Fatalf("wrong provider error = %v", err)
	}
	valid, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose(valid) error = %v", err)
	}
	if reader, ok := provider.TryParse(valid, WithParseClock(func() time.Time { return now })); !ok || reader == nil {
		t.Fatalf("TryParse(valid) = %v %v", reader, ok)
	}
	if _, err := wrongKeyProvider.Parse(valid); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong key error = %v", err)
	}

	wrongAlg := golangjwt.NewWithClaims(golangjwt.SigningMethodHS384, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	wrongAlg.Header["kid"] = "kid"
	wrongAlgToken, err := wrongAlg.SignedString(bytesOf('a', 48))
	if err != nil {
		t.Fatalf("wrongAlg SignedString() error = %v", err)
	}
	if _, err := provider.Parse(wrongAlgToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong alg error = %v", err)
	}

	unknownKID := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{"iat": golangjwt.NewNumericDate(now)})
	unknownKID.Header["kid"] = "unknown"
	unknownKIDToken, err := unknownKID.SignedString(bytesOf('a', 32))
	if err != nil {
		t.Fatalf("unknownKID SignedString() error = %v", err)
	}
	unsupportedHeaderToken := signUnsupportedHeaderToken(t, "zip", "kid", bytesOf('a', 32))
	failureCases := []struct {
		name  string
		token string
		err   error
	}{
		{name: "expired", token: expired, err: ErrExpiredToken},
		{name: "not yet valid", token: notBefore, err: ErrNotYetValid},
		{name: "wrong alg", token: wrongAlgToken, err: ErrInvalidToken},
		{name: "wrong key", token: valid, err: ErrInvalidToken},
		{name: "unknown kid", token: unknownKIDToken, err: ErrInvalidToken},
		{name: "malformed", token: "not-a-token", err: ErrInvalidToken},
		{name: "unsupported header", token: unsupportedHeaderToken, err: ErrInvalidToken},
	}
	for _, tc := range failureCases {
		t.Run("try parse "+tc.name, func(t *testing.T) {
			parseProvider := provider
			if tc.name == "wrong key" {
				parseProvider = wrongKeyProvider
			}
			if reader, ok := parseProvider.TryParse(tc.token, WithParseClock(func() time.Time { return now })); ok || reader != nil {
				t.Fatalf("TryParse(%s) = %v %v", tc.name, reader, ok)
			}
			_, err := parseProvider.Parse(tc.token, WithParseClock(func() time.Time { return now }))
			if !errors.Is(err, tc.err) {
				t.Fatalf("Parse(%s) error = %v, want %v", tc.name, err, tc.err)
			}
			assertErrorDoesNotLeak(t, err, tc.token, "aaaaaaaa", "0123456789abcdef")
		})
	}

	if _, err := provider.Parse("not-a-token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("malformed error = %v", err)
	}
}

func TestMissingKIDBoundary(t *testing.T) {
	fixed, err := NewFixedHMACProvider(HS256, bytesOf('a', 32), WithKeyIDGenerator(staticKID("fixed")))
	if err != nil {
		t.Fatalf("fixed provider error = %v", err)
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{
		"iat": golangjwt.NewNumericDate(time.Now()),
	})
	signed, err := token.SignedString(bytesOf('a', 32))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := fixed.Parse(signed); err != nil {
		t.Fatalf("fixed Parse(no kid) error = %v", err)
	}

	rotating, err := NewHMACProvider(HS256, WithKeyIDGenerator(sequenceKID("rot")))
	if err != nil {
		t.Fatalf("rotating provider error = %v", err)
	}
	if _, err := rotating.Parse(signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("rotating Parse(no kid) error = %v", err)
	}
}

func TestRotatingProviderRotationExpiryAndCapacity(t *testing.T) {
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	provider, err := NewHMACProvider(HS256,
		WithClock(clock),
		WithKeyTTL(time.Hour),
		WithRepositoryCapacity(2),
		WithKeyIDGenerator(sequenceKID("key")),
		WithEntropy(repeatingReader('s')),
	)
	if err != nil {
		t.Fatalf("NewHMACProvider() error = %v", err)
	}
	oldToken, err := provider.Compose(WithExpiresAfter(2 * time.Hour))
	if err != nil {
		t.Fatalf("Compose(old) error = %v", err)
	}
	oldKey, err := provider.CurrentKeyChain()
	if err != nil {
		t.Fatalf("CurrentKeyChain() error = %v", err)
	}
	if _, err := provider.Rotate(); err != nil {
		t.Fatalf("Rotate() live key error = %v", err)
	}
	if current, _ := provider.CurrentKeyChain(); current.KID() != oldKey.KID() {
		t.Fatalf("Rotate() changed live key")
	}

	if _, err := provider.ForcedRotate(); err != nil {
		t.Fatalf("ForcedRotate() error = %v", err)
	}
	rotatedKey, err := provider.CurrentKeyChain()
	if err != nil {
		t.Fatalf("CurrentKeyChain(rotated) error = %v", err)
	}
	if rotatedKey.KID() == oldKey.KID() {
		t.Fatalf("ForcedRotate() did not change kid: %q", rotatedKey.KID())
	}
	newToken, err := provider.Compose(WithExpiresAfter(2 * time.Hour))
	if err != nil {
		t.Fatalf("Compose(new) error = %v", err)
	}
	newReader, err := provider.Parse(newToken, WithParseClock(clock))
	if err != nil {
		t.Fatalf("Parse(new) error = %v", err)
	}
	if newReader.Kid() != rotatedKey.KID() {
		t.Fatalf("new token kid = %q, want %q", newReader.Kid(), rotatedKey.KID())
	}
	if _, err := provider.Parse(oldToken, WithParseClock(clock)); err != nil {
		t.Fatalf("old retained key parse error = %v", err)
	}
	if _, err := provider.ForcedRotate(); err != nil {
		t.Fatalf("second ForcedRotate() error = %v", err)
	}
	if _, err := provider.Parse(oldToken, WithParseClock(clock)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("evicted old key parse error = %v", err)
	}

	expiring, err := provider.Compose(WithExpiresAfter(2 * time.Hour))
	if err != nil {
		t.Fatalf("Compose(expiring) error = %v", err)
	}
	now = now.Add(2 * time.Hour)
	if _, err := provider.Parse(expiring, WithParseClock(clock)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired key parse error = %v", err)
	}
}

func TestGeneratedHMACEntropyFailures(t *testing.T) {
	_, err := NewHMACProvider(HS256,
		WithKeyIDGenerator(staticKID("kid")),
		WithEntropy(io.LimitReader(repeatingReader('x'), 31)),
	)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("NewHMACProvider() error = %v, want ErrInvalidKey", err)
	}
}

func TestConcurrentComposeParseAndRotate(t *testing.T) {
	provider, err := NewHMACProvider(HS256,
		WithRepositoryCapacity(512),
		WithKeyIDGenerator(sequenceKID("stress")),
		WithEntropy(repeatingReader('z')),
	)
	if err != nil {
		t.Fatalf("NewHMACProvider() error = %v", err)
	}
	retained, err := provider.Compose(WithExpiresAfter(time.Hour), WithClaim("retained", "yes"))
	if err != nil {
		t.Fatalf("Compose(retained) error = %v", err)
	}
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 40,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t,
		func(context.Context) error {
			token, err := provider.Compose(WithExpiresAfter(time.Hour), WithClaim("n", "v"))
			if err != nil {
				return err
			}
			reader, err := provider.Parse(token)
			if err != nil {
				return err
			}
			if value, ok := reader.ClaimString("n"); !ok || value != "v" {
				return errorsNew("claim mismatch")
			}
			retainedReader, err := provider.Parse(retained)
			if err != nil {
				return err
			}
			if value, ok := retainedReader.ClaimString("retained"); !ok || value != "yes" {
				return errorsNew("retained claim mismatch")
			}
			return nil
		},
		func(context.Context) error {
			_, err := provider.ForcedRotate()
			return err
		},
	)
}

func TestConcurrentExpiryRotationKeepsReturnedTokensVerifiable(t *testing.T) {
	var nowNanos atomic.Int64
	base := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	nowNanos.Store(base.UnixNano())
	provider, err := NewHMACProvider(HS256,
		WithClock(func() time.Time { return time.Unix(0, nowNanos.Load()).UTC() }),
		WithKeyTTL(time.Nanosecond),
		WithRepositoryCapacity(2),
		WithKeyIDGenerator(sequenceKID("expiry")),
		WithEntropy(repeatingReader('e')),
	)
	if err != nil {
		t.Fatalf("NewHMACProvider() error = %v", err)
	}
	nowNanos.Store(base.Add(2 * time.Nanosecond).UnixNano())

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 30,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(context.Context) error {
		token, err := provider.Compose(WithExpiresAfter(time.Hour), WithClaim("phase", "post-expiry"))
		if err != nil {
			return err
		}
		runtime.Gosched()
		reader, err := provider.Parse(token)
		if err != nil {
			return err
		}
		if value, ok := reader.ClaimString("phase"); !ok || value != "post-expiry" {
			return errorsNew("post-expiry claim mismatch")
		}
		return nil
	})
}

func TestProviderZeroValueAndNilReceiverReturnErrors(t *testing.T) {
	var zero Provider
	if _, err := zero.Compose(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero Compose() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := zero.Parse("token"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero Parse() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := zero.CurrentKeyChain(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero CurrentKeyChain() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := zero.Rotate(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero Rotate() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := zero.ForcedRotate(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero ForcedRotate() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := zero.FindKeyChain("kid"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero FindKeyChain() error = %v, want ErrInvalidOptions", err)
	}

	var nilProvider *Provider
	if _, err := nilProvider.Compose(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil Compose() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := nilProvider.Parse("token"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil Parse() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := nilProvider.CurrentKeyChain(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil CurrentKeyChain() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := nilProvider.Rotate(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil Rotate() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := nilProvider.ForcedRotate(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil ForcedRotate() error = %v, want ErrInvalidOptions", err)
	}
	if _, err := nilProvider.FindKeyChain("kid"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil FindKeyChain() error = %v, want ErrInvalidOptions", err)
	}

	fixed, err := NewFixedHMACProvider(HS256, bytesOf('a', 32), WithKeyIDGenerator(staticKID("fixed")))
	if err != nil {
		t.Fatalf("fixed provider error = %v", err)
	}
	if _, err := fixed.ForcedRotate(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("fixed ForcedRotate() error = %v, want ErrInvalidOptions", err)
	}
}

func signUnsupportedHeaderToken(t *testing.T, header string, kid string, secret []byte) string {
	t.Helper()
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodHS256, golangjwt.MapClaims{
		"iat": golangjwt.NewNumericDate(time.Now()),
	})
	token.Header["kid"] = kid
	token.Header[header] = "unsupported"
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("SignedString(%s) error = %v", header, err)
	}
	return signed
}

func assertErrorDoesNotLeak(t *testing.T, err error, forbidden ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error")
	}
	message := err.Error()
	for _, value := range forbidden {
		if value != "" && strings.Contains(message, value) {
			t.Fatalf("error leaked %q in %q", value, message)
		}
	}
}

func bytesOf(value byte, count int) []byte {
	out := make([]byte, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func staticKID(kid string) func() (string, error) {
	return func() (string, error) { return kid, nil }
}

func sequenceKID(prefix string) func() (string, error) {
	var mu sync.Mutex
	var next int
	return func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		next++
		return prefix + "-" + time.Unix(int64(next), 0).UTC().Format("20060102150405"), nil
	}
}

type repeatingReader byte

func (r repeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}
