package jwks

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

func TestLookupFetchesCachesAndRotatesSnapshot(t *testing.T) {
	first := newRSAJWK(t, "first", RS256)
	second := newRSAJWK(t, "second", RS256)
	var body atomic.Value
	body.Store(marshalJWKSet(t, first))
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(body.Load().([]byte))
	}))
	defer server.Close()

	provider, err := New(server.URL, WithCacheTTL(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	provider.cfg.now = func() time.Time { return now }
	if _, err := provider.Lookup(context.Background(), "first", RS256); err != nil {
		t.Fatalf("first Lookup() error = %v", err)
	}
	if _, err := provider.Lookup(context.Background(), "first", RS256); err != nil {
		t.Fatalf("warm Lookup() error = %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests after warm hit = %d, want 1", got)
	}

	body.Store(marshalJWKSet(t, second))
	now = now.Add(time.Minute)
	if _, err := provider.Lookup(context.Background(), "second", RS256); err != nil {
		t.Fatalf("rotated Lookup() error = %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests after rotation = %d, want 2", got)
	}
}

func TestLookupRejectsAlgorithmMismatchAfterColdFetch(t *testing.T) {
	key := newRSAJWK(t, "rsa", RS256)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(marshalJWKSet(t, key))
	}))
	defer server.Close()
	provider, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Lookup(context.Background(), "rsa", ES256); !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("Lookup() error = %v, want ErrUnsupportedAlgorithm", err)
	}
}

func TestLookupUnknownKidRefreshesOncePerCooldown(t *testing.T) {
	first := newRSAJWK(t, "first", RS256)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(marshalJWKSet(t, first))
	}))
	defer server.Close()
	provider, err := New(server.URL, WithRefreshCooldown(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Lookup(context.Background(), "missing", RS256); err == nil {
		t.Fatal("Lookup(missing) error = nil")
	}
	if _, err := provider.Lookup(context.Background(), "missing", RS256); err == nil {
		t.Fatal("Lookup(missing second) error = nil")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestRefreshBypassesCooldown(t *testing.T) {
	key := newRSAJWK(t, "key", RS256)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(marshalJWKSet(t, key))
	}))
	defer server.Close()
	provider, err := New(server.URL, WithRefreshCooldown(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() second error = %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestRefreshLeaderCancellationAllowsTakeoverAndIgnoresLateResult(t *testing.T) {
	oldKey := newRSAJWK(t, "old", RS256)
	newKey := newRSAJWK(t, "new", RS256)
	firstStarted := make(chan struct{})
	firstRelease := make(chan struct{})
	var requests atomic.Int64
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		call := requests.Add(1)
		if call == 1 {
			close(firstStarted)
			<-firstRelease
			return jsonResponse(marshalJWKSet(t, oldKey)), nil
		}
		return jsonResponse(marshalJWKSet(t, newKey)), nil
	})}
	provider, err := New("https://issuer.example.test/jwks", WithHTTPClient(client), WithRefreshCooldown(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderDone := make(chan error, 1)
	go func() {
		_, lookupErr := provider.Lookup(leaderCtx, "old", RS256)
		leaderDone <- lookupErr
	}()
	<-firstStarted
	cancelLeader()
	if err := <-leaderDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader Lookup() error = %v, want context.Canceled", err)
	}

	if _, err := provider.Lookup(context.Background(), "new", RS256); err != nil {
		t.Fatalf("takeover Lookup() error = %v", err)
	}
	close(firstRelease)
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}

	if _, err := provider.Lookup(context.Background(), "old", RS256); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("late leader published old key: error = %v, want ErrKeyNotFound", err)
	}
}

func TestLookupConcurrentMissesShareOneRefresh(t *testing.T) {
	key := newRSAJWK(t, "shared", RS256)
	started := make(chan struct{})
	release := make(chan struct{})
	var requests atomic.Int64
	var startOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		startOnce.Do(func() { close(started) })
		<-release
		_, _ = w.Write(marshalJWKSet(t, key))
	}))
	defer server.Close()
	provider, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	const callers = 8
	ready := make(chan struct{}, callers)
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			ready <- struct{}{}
			_, lookupErr := provider.Lookup(context.Background(), "shared", RS256)
			results <- lookupErr
		}()
	}
	for i := 0; i < callers; i++ {
		<-ready
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("single-flight leader did not start")
	}
	close(release)
	for i := 0; i < callers; i++ {
		if err := <-results; err != nil {
			t.Fatalf("concurrent Lookup() error = %v", err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestLookupWaiterCancellationDoesNotCancelLeader(t *testing.T) {
	key := newRSAJWK(t, "shared", RS256)
	started := make(chan struct{})
	release := make(chan struct{})
	var requests atomic.Int64
	var startOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		startOnce.Do(func() { close(started) })
		<-release
		_, _ = w.Write(marshalJWKSet(t, key))
	}))
	defer server.Close()
	provider, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	leaderDone := make(chan error, 1)
	go func() {
		_, lookupErr := provider.Lookup(context.Background(), "shared", RS256)
		leaderDone <- lookupErr
	}()
	<-started
	waiterContext, cancelWaiter := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelWaiter()
	if _, err := provider.Lookup(waiterContext, "shared", RS256); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter Lookup() error = %v, want context deadline", err)
	}
	close(release)
	if err := <-leaderDone; err != nil {
		t.Fatalf("leader Lookup() error = %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestWarmLookupIsNotBlockedByExplicitRefresh(t *testing.T) {
	key := newRSAJWK(t, "warm", RS256)
	refreshStarted := make(chan struct{})
	refreshRelease := make(chan struct{})
	var requests atomic.Int64
	var refreshOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := requests.Add(1)
		if call == 2 {
			refreshOnce.Do(func() { close(refreshStarted) })
			<-refreshRelease
		}
		_, _ = w.Write(marshalJWKSet(t, key))
	}))
	defer server.Close()
	provider, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Lookup(context.Background(), "warm", RS256); err != nil {
		t.Fatalf("initial Lookup() error = %v", err)
	}
	refreshDone := make(chan error, 1)
	go func() { refreshDone <- provider.Refresh(context.Background()) }()
	select {
	case <-refreshStarted:
	case <-time.After(time.Second):
		t.Fatal("explicit refresh did not start")
	}
	warmDone := make(chan error, 1)
	go func() {
		_, lookupErr := provider.Lookup(context.Background(), "warm", RS256)
		warmDone <- lookupErr
	}()
	select {
	case err := <-warmDone:
		if err != nil {
			t.Fatalf("warm Lookup() error = %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("warm Lookup() was blocked by refresh")
	}
	close(refreshRelease)
	if err := <-refreshDone; err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
}

func TestLookupReturnsDefensivePublicKeyCopies(t *testing.T) {
	rsaPrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecPrivate, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(marshalJWKSet(t,
			jose.JSONWebKey{Key: &rsaPrivate.PublicKey, KeyID: "rsa", Algorithm: string(RS256), Use: "sig"},
			jose.JSONWebKey{Key: &ecPrivate.PublicKey, KeyID: "ec", Algorithm: string(ES256), Use: "sig"},
			jose.JSONWebKey{Key: edPublic, KeyID: "ed", Algorithm: string(EdDSA), Use: "sig"},
		))
	}))
	defer server.Close()
	provider, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	rsaValue, err := provider.Lookup(context.Background(), "rsa", RS256)
	if err != nil {
		t.Fatal(err)
	}
	ecValue, err := provider.Lookup(context.Background(), "ec", ES256)
	if err != nil {
		t.Fatal(err)
	}
	edValue, err := provider.Lookup(context.Background(), "ed", EdDSA)
	if err != nil {
		t.Fatal(err)
	}
	rsaCopy := rsaValue.(*rsa.PublicKey)
	rsaCopy.N.SetInt64(3)
	rsaCopy.E = 3
	ecCopy := ecValue.(*ecdsa.PublicKey)
	tamperedEC, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	*ecCopy = tamperedEC.PublicKey
	edCopy := edValue.(ed25519.PublicKey)
	edCopy[0] ^= 0xff

	rsaAgain, err := provider.Lookup(context.Background(), "rsa", RS256)
	if err != nil {
		t.Fatal(err)
	}
	ecAgain, err := provider.Lookup(context.Background(), "ec", ES256)
	if err != nil {
		t.Fatal(err)
	}
	edAgain, err := provider.Lookup(context.Background(), "ed", EdDSA)
	if err != nil {
		t.Fatal(err)
	}
	if rsaAgain.(*rsa.PublicKey).N.Cmp(rsaPrivate.N) != 0 || rsaAgain.(*rsa.PublicKey).E != rsaPrivate.E {
		t.Fatal("RSA snapshot was mutated through returned key")
	}
	if !ecAgain.(*ecdsa.PublicKey).Equal(&ecPrivate.PublicKey) {
		t.Fatal("ECDSA snapshot was mutated through returned key")
	}
	if string(edAgain.(ed25519.PublicKey)) != string(edPublic) {
		t.Fatal("Ed25519 snapshot was mutated through returned key")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func newRSAJWK(t *testing.T, kid string, algorithm Algorithm) jose.JSONWebKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return jose.JSONWebKey{Key: &key.PublicKey, KeyID: kid, Algorithm: string(algorithm), Use: "sig"}
}

func marshalJWKSet(t *testing.T, keys ...jose.JSONWebKey) []byte {
	t.Helper()
	body, err := json.Marshal(jose.JSONWebKeySet{Keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
