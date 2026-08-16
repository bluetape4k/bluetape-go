package jwks_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	rootjwt "github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/jwt/jwks"
	jose "github.com/go-jose/go-jose/v4"
	golangjwt "github.com/golang-jwt/jwt/v5"
)

func ExampleProvider_KeyFunc() {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	body, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &privateKey.PublicKey,
		KeyID:     "example",
		Algorithm: string(jwks.RS256),
		Use:       "sig",
	}}})
	if err != nil {
		panic(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	// root jwt 알고리즘을 하위 provider allowlist로 넘길 때는 명시적으로 변환한다.
	rootAlgorithm := rootjwt.RS256
	provider, err := jwks.New(server.URL,
		jwks.WithAllowedAlgorithms(jwks.Algorithm(rootAlgorithm)),
	)
	if err != nil {
		panic(err)
	}
	if err := provider.Refresh(context.Background()); err != nil {
		panic(err)
	}
	keyFunc, err := provider.KeyFunc(context.Background())
	if err != nil {
		panic(err)
	}
	token := golangjwt.NewWithClaims(golangjwt.SigningMethodRS256, golangjwt.RegisteredClaims{
		Issuer:    "issuer",
		Audience:  golangjwt.ClaimStrings{"audience"},
		ExpiresAt: golangjwt.NewNumericDate(time.Now().Add(time.Minute)),
	})
	token.Header["kid"] = "example"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		panic(err)
	}
	parser := golangjwt.NewParser(
		golangjwt.WithValidMethods([]string{string(jwks.RS256)}),
		golangjwt.WithIssuer("issuer"),
		golangjwt.WithAudience("audience"),
		golangjwt.WithExpirationRequired(),
	)
	parsed, err := parser.ParseWithClaims(signed, &golangjwt.RegisteredClaims{}, keyFunc)
	if err != nil || !parsed.Valid {
		panic(fmt.Sprintf("JWT verification failed: %v", err))
	}
	fmt.Println("verified")
	// Output: verified
}
