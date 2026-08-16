package jwks

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	rootjwt "github.com/bluetape4k/bluetape-go/jwt"
	jose "github.com/go-jose/go-jose/v4"
)

func TestParseKeySetAcceptsAsymmetricPublicKeys(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	edKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &rsaKey.PublicKey, KeyID: "rsa", Algorithm: string(RS256), Use: "sig"},
		{Key: &ecKey.PublicKey, KeyID: "ec", Algorithm: string(ES256), Use: "sig"},
		{Key: edKey, KeyID: "ed", Algorithm: string(EdDSA), Use: "sig"},
	}}
	body, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := parseKeySet(body)
	if err != nil {
		t.Fatalf("parseKeySet() error = %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}
	if _, ok := keys["rsa"].key.(*rsa.PublicKey); !ok {
		t.Fatalf("rsa key type = %T", keys["rsa"].key)
	}
	if _, ok := keys["ec"].key.(*ecdsa.PublicKey); !ok {
		t.Fatalf("ec key type = %T", keys["ec"].key)
	}
	if _, ok := keys["ed"].key.(ed25519.PublicKey); !ok {
		t.Fatalf("ed key type = %T", keys["ed"].key)
	}
}

func TestParseKeySetRejectsWeakOrAmbiguousKeys(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []error
	}{
		{name: "empty set", body: `{"keys":[]}`, want: []error{ErrMalformedSet}},
		{name: "oct", body: `{"keys":[{"kty":"oct","kid":"k","k":"c2VjcmV0"}]}`, want: []error{ErrMalformedSet, rootjwt.ErrInvalidKey}},
		{name: "missing kid", body: `{"keys":[{"kty":"RSA","n":"AQ","e":"AQAB"}]}`, want: []error{ErrMalformedSet}},
		{name: "duplicate kid", body: `{"keys":[{"kty":"RSA","kid":"k","n":"AQ","e":"AQAB"},{"kty":"RSA","kid":"k","n":"AQ","e":"AQAB"}]}`, want: []error{ErrMalformedSet}},
		{name: "wrong use", body: `{"keys":[{"kty":"RSA","kid":"k","use":"enc","n":"AQ","e":"AQAB"}]}`, want: []error{ErrMalformedSet}},
		{name: "bad kid", body: `{"keys":[{"kty":"RSA","kid":"bad\u0001","n":"AQ","e":"AQAB"}]}`, want: []error{ErrMalformedSet}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseKeySet([]byte(tt.body))
			if err == nil {
				t.Fatal("parseKeySet() error = nil")
			}
			for _, sentinel := range tt.want {
				if !errors.Is(err, sentinel) {
					t.Errorf("error = %v, errors.Is(%v) = false", err, sentinel)
				}
			}
		})
	}
}

func TestParseKeySetRejectsMalformedEd25519AndRSAExponent(t *testing.T) {
	badEd := base64.RawURLEncoding.EncodeToString(make([]byte, 31))
	badRSAExponent := base64.RawURLEncoding.EncodeToString([]byte{2})
	for name, body := range map[string]string{
		"ed25519 length": `{"keys":[{"kty":"OKP","crv":"Ed25519","kid":"ed","x":"` + badEd + `"}]}`,
		"rsa exponent":   `{"keys":[{"kty":"RSA","kid":"rsa","n":"` + base64.RawURLEncoding.EncodeToString(make([]byte, 256)) + `","e":"` + badRSAExponent + `"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseKeySet([]byte(body)); !errors.Is(err, ErrMalformedSet) {
				t.Fatalf("parseKeySet() error = %v, want ErrMalformedSet", err)
			}
		})
	}
}

func TestParseKeySetRejectsOversizedAndUnsupportedKeyOperations(t *testing.T) {
	longKID := strings.Repeat("a", 129)
	key := `{"kty":"RSA","kid":"%s","n":"%s","e":"AQAB","key_ops":["sign"]}`
	n := base64.RawURLEncoding.EncodeToString(new(big.Int).Lsh(big.NewInt(1), 2047).Bytes())
	for name, body := range map[string]string{
		"long kid":  `{"keys":[` + fmt.Sprintf(key, longKID, n) + `]}`,
		"sign op":   `{"keys":[` + fmt.Sprintf(key, "rsa", n) + `]}`,
		"many keys": `{"keys":[` + strings.TrimSuffix(strings.Repeat(`{"kty":"RSA","kid":"rsa","n":"`+n+`","e":"AQAB"},`, 257), ",") + `]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseKeySet([]byte(body)); !errors.Is(err, ErrMalformedSet) {
				t.Fatalf("parseKeySet() error = %v, want ErrMalformedSet", err)
			}
		})
	}
}

func TestParseKeySetAcceptsCertificateURLMetadataWithoutFetching(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &key.PublicKey,
		KeyID:     "rsa",
		Algorithm: string(RS256),
		Use:       "sig",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	document["keys"].([]any)[0].(map[string]any)["x5u"] = "https://certs.example.test/key.pem"
	body, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseKeySet(body)
	if err != nil {
		t.Fatalf("parseKeySet() error = %v", err)
	}
	if _, ok := parsed["rsa"]; !ok {
		t.Fatal("certificate URL metadata removed the public key")
	}
}
