package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"io"
	"time"
)

const (
	defaultKeyTTL         = 365 * 24 * time.Hour
	defaultRepositorySize = 10
	minRepositorySize     = 2
	maxRepositorySize     = 1000
	defaultRSAKeyBits     = 2048
	defaultKIDBytes       = 16
	maxKIDBytes           = 128
)

// KeyChain 은 하나의 JWT 서명/검증 key와 kid metadata를 나타낸다.
type KeyChain struct {
	kid             string
	algorithm       Algorithm
	createdAt       time.Time
	expiresAt       time.Time
	signingKey      any
	verificationKey any
}

// KID 는 JWT header의 kid 값을 반환한다.
func (k *KeyChain) KID() string {
	if k == nil {
		return ""
	}
	return k.kid
}

// Algorithm 은 KeyChain의 서명 알고리즘을 반환한다.
func (k *KeyChain) Algorithm() Algorithm {
	if k == nil {
		return ""
	}
	return k.algorithm
}

// CreatedAt 은 KeyChain 생성 시각을 반환한다.
func (k *KeyChain) CreatedAt() time.Time {
	if k == nil {
		return time.Time{}
	}
	return k.createdAt
}

// ExpiresAt 은 KeyChain 만료 시각을 반환한다.
func (k *KeyChain) ExpiresAt() time.Time {
	if k == nil {
		return time.Time{}
	}
	return k.expiresAt
}

// Expired 는 now 기준으로 KeyChain 만료 여부를 반환한다.
func (k *KeyChain) Expired(now time.Time) bool {
	if k == nil {
		return true
	}
	return !k.expiresAt.IsZero() && !now.Before(k.expiresAt)
}

func (k *KeyChain) signingMaterial() any {
	if secret, ok := k.signingKey.([]byte); ok {
		return append([]byte(nil), secret...)
	}
	return k.signingKey
}

func (k *KeyChain) verificationMaterial() any {
	if secret, ok := k.verificationKey.([]byte); ok {
		return append([]byte(nil), secret...)
	}
	return k.verificationKey
}

func newHMACKeyChain(kid string, algorithm Algorithm, secret []byte, createdAt time.Time, ttl time.Duration) (*KeyChain, error) {
	if err := validateHMACSecret(algorithm, secret); err != nil {
		return nil, err
	}
	if err := validateKID(kid); err != nil {
		return nil, err
	}
	copied := append([]byte(nil), secret...)
	return &KeyChain{
		kid:             kid,
		algorithm:       algorithm,
		createdAt:       createdAt,
		expiresAt:       createdAt.Add(ttl),
		signingKey:      copied,
		verificationKey: copied,
	}, nil
}

func newRSAKeyChain(kid string, algorithm Algorithm, privateKey *rsa.PrivateKey, createdAt time.Time, ttl time.Duration) (*KeyChain, error) {
	if !algorithm.isRSA() {
		return nil, OptionError{Option: "algorithm", Err: errorsNew("algorithm must be rsa")}
	}
	if privateKey == nil {
		return nil, KeyError{Kind: ErrInvalidKey, Err: errorsNew("rsa private key must not be nil")}
	}
	if privateKey.N == nil || privateKey.N.BitLen() < defaultRSAKeyBits {
		return nil, KeyError{Kind: ErrInvalidKey, Err: errorsNew("rsa private key must be at least 2048 bits")}
	}
	if err := privateKey.Validate(); err != nil {
		return nil, KeyError{Kind: ErrInvalidKey, Err: err}
	}
	copied, err := cloneRSAPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	if err := validateKID(kid); err != nil {
		return nil, err
	}
	return &KeyChain{
		kid:             kid,
		algorithm:       algorithm,
		createdAt:       createdAt,
		expiresAt:       createdAt.Add(ttl),
		signingKey:      copied,
		verificationKey: &copied.PublicKey,
	}, nil
}

func cloneRSAPrivateKey(privateKey *rsa.PrivateKey) (*rsa.PrivateKey, error) {
	der := x509.MarshalPKCS1PrivateKey(privateKey)
	copied, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return nil, KeyError{Kind: ErrInvalidKey, Err: err}
	}
	return copied, nil
}

func generateKID(entropy io.Reader) (string, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	buf := make([]byte, defaultKIDBytes)
	if _, err := io.ReadFull(entropy, buf); err != nil {
		return "", KeyError{Kind: ErrInvalidKey, Err: err}
	}
	return hex.EncodeToString(buf), nil
}

func validateKID(kid string) error {
	if kid == "" {
		return KeyError{Kind: ErrInvalidKey, Err: errorsNew("kid must not be empty")}
	}
	if len([]byte(kid)) > maxKIDBytes {
		return KeyError{Kind: ErrInvalidKey, Err: errorsNew("kid is too long")}
	}
	for _, r := range kid {
		if r < 0x21 || r > 0x7e {
			return KeyError{Kind: ErrInvalidKey, Err: errorsNew("kid must contain only printable ASCII without spaces")}
		}
	}
	return nil
}

func validateLookupKID(kid string) error {
	if kid == "" {
		return KeyError{Kind: ErrKeyNotFound, Err: errorsNew("kid is required")}
	}
	if err := validateKID(kid); err != nil {
		return KeyError{Kind: ErrKeyNotFound, Err: err}
	}
	return nil
}

func generateHMACSecret(algorithm Algorithm, entropy io.Reader) ([]byte, error) {
	length, ok := algorithm.hmacSecretLength()
	if !ok {
		return nil, OptionError{Option: "algorithm", Err: errorsNew("algorithm must be hmac")}
	}
	secret := make([]byte, length)
	if _, err := io.ReadFull(entropy, secret); err != nil {
		return nil, KeyError{Kind: ErrInvalidKey, Err: err}
	}
	return secret, nil
}
