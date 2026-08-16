package jwks

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	rootjwt "github.com/bluetape4k/bluetape-go/jwt"
)

func TestNewRejectsUnsafeEndpointAndNilOption(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{name: "empty", endpoint: ""},
		{name: "unsupported scheme", endpoint: "ftp://example.com/jwks.json"},
		{name: "missing host", endpoint: "https:///jwks.json"},
		{name: "userinfo", endpoint: "https://user:pass@example.com/jwks.json"},
		{name: "fragment", endpoint: "https://example.com/jwks.json#keys"},
		{name: "remote http", endpoint: "http://example.com/jwks.json"},
		{name: "private literal", endpoint: "https://10.0.0.1/jwks.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.endpoint); !errors.Is(err, rootjwt.ErrInvalidOptions) {
				t.Fatalf("New(%q) error = %v, want ErrInvalidOptions", tt.endpoint, err)
			}
		})
	}

	if _, err := New("https://example.com/jwks.json", nil); !errors.Is(err, rootjwt.ErrInvalidOptions) {
		t.Fatalf("New(..., nil) error = %v, want ErrInvalidOptions", err)
	}
}

func TestOptionsRejectInvalidValuesAndSymmetricAlgorithms(t *testing.T) {
	client := &http.Client{}
	tests := []struct {
		name string
		opt  Option
	}{
		{name: "nil client", opt: WithHTTPClient(nil)},
		{name: "zero ttl", opt: WithCacheTTL(0)},
		{name: "negative cooldown", opt: WithRefreshCooldown(-time.Second)},
		{name: "zero fetch timeout", opt: WithFetchTimeout(0)},
		{name: "zero body size", opt: WithMaxBodySize(0)},
		{name: "empty allowlist", opt: WithAllowedAlgorithms()},
		{name: "duplicate allowlist", opt: WithAllowedAlgorithms(RS256, RS256)},
		{name: "unknown allowlist", opt: WithAllowedAlgorithms(Algorithm("NOPE"))},
		{name: "symmetric allowlist", opt: WithAllowedAlgorithms(Algorithm("HS256"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New("https://example.com/jwks.json", tt.opt); !errors.Is(err, rootjwt.ErrInvalidOptions) {
				t.Fatalf("New() error = %v, want ErrInvalidOptions", err)
			}
		})
	}
	if _, err := New("https://example.com/jwks.json", WithHTTPClient(client)); err != nil {
		t.Fatalf("New(valid custom client) error = %v", err)
	}
}

func TestAllowedAlgorithmsOnlyNarrowAcrossOptions(t *testing.T) {
	provider, err := New(
		"https://issuer.example.test/jwks",
		WithAllowedAlgorithms(RS256),
		WithAllowedAlgorithms(RS256, PS256),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Lookup(context.Background(), "kid", PS256); !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("Lookup(PS256) error = %v, want ErrUnsupportedAlgorithm", err)
	}
	if _, err := New(
		"https://issuer.example.test/jwks",
		WithAllowedAlgorithms(RS256),
		WithAllowedAlgorithms(ES256),
	); !errors.Is(err, rootjwt.ErrInvalidOptions) {
		t.Fatalf("disjoint allowlists error = %v, want ErrInvalidOptions", err)
	}
}

func TestTypedErrorsPreserveSentinelsAndContext(t *testing.T) {
	fetch := FetchError{Class: FetchClassTransport, Status: 503, Err: context.DeadlineExceeded}
	if !errors.Is(fetch, ErrFetch) || !errors.Is(fetch, context.DeadlineExceeded) {
		t.Fatalf("FetchError errors.Is() did not preserve sentinels: %v", fetch)
	}
	var gotFetch FetchError
	if !errors.As(fetch, &gotFetch) || gotFetch.Status != 503 {
		t.Fatalf("FetchError errors.As() = %#v", gotFetch)
	}
	set := SetError{Err: rootjwt.ErrInvalidKey}
	if !errors.Is(set, ErrMalformedSet) || !errors.Is(set, rootjwt.ErrInvalidKey) {
		t.Fatalf("SetError errors.Is() did not preserve sentinels: %v", set)
	}
	var gotSet SetError
	if !errors.As(set, &gotSet) {
		t.Fatalf("SetError errors.As() = %#v", gotSet)
	}
	for _, err := range []error{fetch, set} {
		if got := err.Error(); got == "" || containsAny(got, "https://", "token", "body", "JWK") {
			t.Fatalf("error leaked sensitive material: %q", got)
		}
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if len(needle) > 0 && stringContains(value, needle) {
			return true
		}
	}
	return false
}

func stringContains(value, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
