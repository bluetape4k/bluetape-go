package jwks

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	rootjwt "github.com/bluetape4k/bluetape-go/jwt"
	jose "github.com/go-jose/go-jose/v4"
	golangjwt "github.com/golang-jwt/jwt/v5"
)

func TestKeyFuncVerifiesRSAECDSAAndEdDSATokens(t *testing.T) {
	rsaPrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecdsaPrivate, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edPublic, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		kid        string
		algorithm  Algorithm
		method     golangjwt.SigningMethod
		privateKey any
		publicKey  any
	}{
		{name: "rsa", kid: "rsa", algorithm: RS256, method: golangjwt.SigningMethodRS256, privateKey: rsaPrivate, publicKey: &rsaPrivate.PublicKey},
		{name: "pss", kid: "pss", algorithm: PS256, method: golangjwt.SigningMethodPS256, privateKey: rsaPrivate, publicKey: &rsaPrivate.PublicKey},
		{name: "ecdsa", kid: "ec", algorithm: ES256, method: golangjwt.SigningMethodES256, privateKey: ecdsaPrivate, publicKey: &ecdsaPrivate.PublicKey},
		{name: "eddsa", kid: "ed", algorithm: EdDSA, method: golangjwt.SigningMethodEdDSA, privateKey: edPrivate, publicKey: edPublic},
	}
	keys := make([]jose.JSONWebKey, 0, len(tests))
	for _, tt := range tests {
		keys = append(keys, jose.JSONWebKey{Key: tt.publicKey, KeyID: tt.kid, Algorithm: string(tt.algorithm), Use: "sig"})
	}
	body, err := json.Marshal(jose.JSONWebKeySet{Keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	provider, err := New(server.URL, WithCacheTTL(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	keyFunc, err := provider.KeyFunc(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	parser := golangjwt.NewParser(
		golangjwt.WithValidMethods([]string{"RS256", "PS256", "ES256", "EdDSA"}),
		golangjwt.WithIssuer("issuer"),
		golangjwt.WithAudience("audience"),
		golangjwt.WithExpirationRequired(),
	)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := golangjwt.RegisteredClaims{
				Issuer:    "issuer",
				Audience:  golangjwt.ClaimStrings{"audience"},
				ExpiresAt: golangjwt.NewNumericDate(time.Now().Add(time.Minute)),
			}
			token := golangjwt.NewWithClaims(tt.method, claims)
			token.Header["kid"] = tt.kid
			signed, err := token.SignedString(tt.privateKey)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := parser.ParseWithClaims(signed, &golangjwt.RegisteredClaims{}, keyFunc)
			if err != nil {
				t.Fatalf("ParseWithClaims() error = %v", err)
			}
			if !parsed.Valid {
				t.Fatal("ParseWithClaims() returned invalid token")
			}
		})
	}
}

func TestKeyFuncRejectsUnsafeHeadersAndConstructionInputs(t *testing.T) {
	var requests atomic.Int64
	provider, err := New("https://issuer.example.test/jwks", WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("unexpected JWKS request")
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.KeyFunc(nilContext()); !errors.Is(err, rootjwt.ErrInvalidOptions) {
		t.Fatalf("KeyFunc(nil) error = %v", err)
	}
	var nilProvider *Provider
	if _, err := nilProvider.KeyFunc(context.Background()); !errors.Is(err, rootjwt.ErrInvalidOptions) {
		t.Fatalf("nil provider KeyFunc() error = %v", err)
	}
	keyFunc, err := provider.KeyFunc(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keyFunc(nil); !errors.Is(err, rootjwt.ErrInvalidToken) {
		t.Fatalf("KeyFunc(nil token) error = %v", err)
	}

	for _, header := range []string{"zip", "crit", "jku", "jwk", "x5u", "x5c"} {
		t.Run(header, func(t *testing.T) {
			token := &golangjwt.Token{Header: map[string]any{
				"alg":  "RS256",
				"kid":  "known",
				header: "blocked",
			}}
			if _, err := keyFunc(token); !errors.Is(err, rootjwt.ErrInvalidToken) {
				t.Fatalf("KeyFunc() error = %v, want ErrInvalidToken", err)
			}
		})
	}

	for name, kid := range map[string]string{
		"missing": "",
		"control": "bad\n kid",
		"long":    strings.Repeat("a", 129),
	} {
		t.Run(name, func(t *testing.T) {
			token := &golangjwt.Token{Header: map[string]any{"alg": "RS256", "kid": kid}}
			if _, err := keyFunc(token); !errors.Is(err, rootjwt.ErrInvalidToken) {
				t.Fatalf("KeyFunc() error = %v, want ErrInvalidToken", err)
			}
		})
	}
	for name, alg := range map[string]any{
		"missing":    nil,
		"non-string": 123,
		"symmetric":  "HS256",
		"unknown":    "NOPE",
	} {
		t.Run("alg-"+name, func(t *testing.T) {
			token := &golangjwt.Token{Header: map[string]any{"alg": alg, "kid": "known"}}
			if _, err := keyFunc(token); !errors.Is(err, rootjwt.ErrInvalidToken) && !errors.Is(err, ErrUnsupportedAlgorithm) {
				t.Fatalf("KeyFunc() error = %v", err)
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledKeyFunc, err := provider.KeyFunc(ctx)
	if err != nil {
		t.Fatal(err)
	}
	token := &golangjwt.Token{Header: map[string]any{"alg": "RS256", "kid": "known"}}
	if _, err := canceledKeyFunc(token); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled KeyFunc() error = %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("unsafe KeyFunc requests = %d, want 0", got)
	}
}

func nilContext() context.Context { return nil }

func TestKeyFuncAllowlistNarrowsAlgorithms(t *testing.T) {
	provider, err := New("https://issuer.example.test/jwks", WithAllowedAlgorithms(ES256), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("unexpected JWKS request")
	})}))
	if err != nil {
		t.Fatal(err)
	}
	keyFunc, err := provider.KeyFunc(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	token := &golangjwt.Token{Header: map[string]any{"alg": "RS256", "kid": "known"}}
	if _, err := keyFunc(token); !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("KeyFunc() error = %v, want ErrUnsupportedAlgorithm", err)
	}
}

func TestKeyFuncLeavesClaimsPolicyToParser(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &privateKey.PublicKey,
		KeyID:     "claims",
		Algorithm: string(RS256),
		Use:       "sig",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	provider, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	keyFunc, err := provider.KeyFunc(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodRS256, golangjwt.RegisteredClaims{
		Issuer:    "unexpected-issuer",
		ExpiresAt: golangjwt.NewNumericDate(time.Now().Add(time.Minute)),
	})
	token.Header["kid"] = "claims"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if parsed, err := golangjwt.ParseWithClaims(signed, &golangjwt.RegisteredClaims{}, keyFunc); err != nil || !parsed.Valid {
		t.Fatalf("signature-only parser error = %v, valid = %v", err, parsed != nil && parsed.Valid)
	}
	parser := golangjwt.NewParser(golangjwt.WithIssuer("expected-issuer"))
	if _, err := parser.ParseWithClaims(signed, &golangjwt.RegisteredClaims{}, keyFunc); err == nil {
		t.Fatal("issuer policy was not enforced by parser")
	}
}
