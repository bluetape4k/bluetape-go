package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEncodeDecodeHMACKeyChain(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	secret := bytesOf('h', 32)
	key, err := newHMACKeyChain("hmac-1", HS256, secret, now, time.Hour)
	if err != nil {
		t.Fatalf("newHMACKeyChain() error = %v", err)
	}

	payload, err := encodeRedisKeyChain(key)
	if err != nil {
		t.Fatalf("encodeRedisKeyChain() error = %v", err)
	}
	decoded, err := decodeRedisKeyChain(payload, defaultRedisMaxKeyBytes)
	if err != nil {
		t.Fatalf("decodeRedisKeyChain() error = %v", err)
	}
	if decoded.KID() != "hmac-1" || decoded.Algorithm() != HS256 {
		t.Fatalf("decoded = %q/%q", decoded.KID(), decoded.Algorithm())
	}
	if !decoded.CreatedAt().Equal(now) || !decoded.ExpiresAt().Equal(now.Add(time.Hour)) {
		t.Fatalf("decoded times = %v/%v", decoded.CreatedAt(), decoded.ExpiresAt())
	}
	if !reflect.DeepEqual(decoded.signingMaterial(), secret) {
		t.Fatalf("decoded signing material mismatch")
	}
}

func TestEncodeDecodeRSAKeyChain(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	key, err := newRSAKeyChain("rsa-1", RS256, privateKey, now, time.Hour)
	if err != nil {
		t.Fatalf("newRSAKeyChain() error = %v", err)
	}

	payload, err := encodeRedisKeyChain(key)
	if err != nil {
		t.Fatalf("encodeRedisKeyChain() error = %v", err)
	}
	decoded, err := decodeRedisKeyChain(payload, defaultRedisMaxKeyBytes)
	if err != nil {
		t.Fatalf("decodeRedisKeyChain() error = %v", err)
	}
	if decoded.KID() != "rsa-1" || decoded.Algorithm() != RS256 {
		t.Fatalf("decoded = %q/%q", decoded.KID(), decoded.Algorithm())
	}
	if _, ok := decoded.signingMaterial().(*rsa.PrivateKey); !ok {
		t.Fatalf("decoded signing material type = %T", decoded.signingMaterial())
	}
}

func TestDecodeRejectsOversizedPayloadBeforeJSON(t *testing.T) {
	payload := []byte(`{"version":1}`)
	_, err := decodeRedisKeyChain(payload, len(payload)-1)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("decodeRedisKeyChain() error = %v, want ErrInvalidKey", err)
	}
}

func TestDecodeRejectsUnknownVersion(t *testing.T) {
	payload := marshalRedisDTO(t, keyChainDTO{Version: 2, KID: "kid", Algorithm: string(HS256), Family: redisKeyFamilyHMAC, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), HMAC: base64.StdEncoding.EncodeToString(bytesOf('h', 32))})
	_, err := decodeRedisKeyChain(payload, defaultRedisMaxKeyBytes)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("decodeRedisKeyChain() error = %v, want ErrInvalidKey", err)
	}
}

func TestDecodeRejectsInvalidKID(t *testing.T) {
	payload := marshalRedisDTO(t, keyChainDTO{Version: redisKeyVersion, KID: "", Algorithm: string(HS256), Family: redisKeyFamilyHMAC, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), HMAC: base64.StdEncoding.EncodeToString(bytesOf('h', 32))})
	_, err := decodeRedisKeyChain(payload, defaultRedisMaxKeyBytes)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("decodeRedisKeyChain() error = %v, want ErrInvalidKey", err)
	}
}

func TestDecodeRejectsAlgorithmFamilyMismatch(t *testing.T) {
	tests := []keyChainDTO{
		{Version: redisKeyVersion, KID: "mismatch-1", Algorithm: string(RS256), Family: redisKeyFamilyHMAC, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), HMAC: base64.StdEncoding.EncodeToString(bytesOf('h', 32))},
		{Version: redisKeyVersion, KID: "mismatch-2", Algorithm: string(HS256), Family: redisKeyFamilyRSA, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), RSA: base64.StdEncoding.EncodeToString([]byte("not-rsa"))},
	}
	for _, dto := range tests {
		t.Run(dto.KID, func(t *testing.T) {
			_, err := decodeRedisKeyChain(marshalRedisDTO(t, dto), defaultRedisMaxKeyBytes)
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("decodeRedisKeyChain() error = %v, want ErrInvalidKey", err)
			}
		})
	}
}

func TestDecodeRejectsShortHMACMaterial(t *testing.T) {
	payload := marshalRedisDTO(t, keyChainDTO{Version: redisKeyVersion, KID: "short", Algorithm: string(HS256), Family: redisKeyFamilyHMAC, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), HMAC: base64.StdEncoding.EncodeToString(bytesOf('h', 31))})
	_, err := decodeRedisKeyChain(payload, defaultRedisMaxKeyBytes)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("decodeRedisKeyChain() error = %v, want ErrInvalidKey", err)
	}
}

func TestDecodeRejectsInvalidRSAMaterial(t *testing.T) {
	payload := marshalRedisDTO(t, keyChainDTO{Version: redisKeyVersion, KID: "bad-rsa", Algorithm: string(RS256), Family: redisKeyFamilyRSA, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), RSA: base64.StdEncoding.EncodeToString([]byte("not-rsa"))})
	_, err := decodeRedisKeyChain(payload, defaultRedisMaxKeyBytes)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("decodeRedisKeyChain() error = %v, want ErrInvalidKey", err)
	}
}

func TestDTOErrorsDoNotLeakKeyMaterial(t *testing.T) {
	secret := strings.Repeat("super-secret-material", 2)
	payload := marshalRedisDTO(t, keyChainDTO{Version: redisKeyVersion, KID: "leak", Algorithm: string(HS512), Family: redisKeyFamilyHMAC, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), HMAC: base64.StdEncoding.EncodeToString([]byte(secret))})
	_, err := decodeRedisKeyChain(payload, defaultRedisMaxKeyBytes)
	if err == nil {
		t.Fatalf("decodeRedisKeyChain() expected error")
	}
	assertErrorDoesNotLeak(t, err, secret, base64.StdEncoding.EncodeToString([]byte(secret)))
}

func TestRedisDTORequiresPackagePrivateReconstruction(t *testing.T) {
	key, err := newHMACKeyChain("private", HS256, bytesOf('p', 32), time.Now(), time.Hour)
	if err != nil {
		t.Fatalf("newHMACKeyChain() error = %v", err)
	}
	payload, err := encodeRedisKeyChain(key)
	if err != nil {
		t.Fatalf("encodeRedisKeyChain() error = %v", err)
	}
	decoded, err := decodeRedisKeyChain(payload, defaultRedisMaxKeyBytes)
	if err != nil {
		t.Fatalf("decodeRedisKeyChain() error = %v", err)
	}
	if decoded == key {
		t.Fatalf("decodeRedisKeyChain() returned original key pointer")
	}
	if !reflect.DeepEqual(decoded.signingMaterial(), key.signingMaterial()) {
		t.Fatalf("decoded key material mismatch")
	}
}

func marshalRedisDTO(t *testing.T, dto keyChainDTO) []byte {
	t.Helper()
	payload, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return payload
}
