package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

const (
	redisKeyVersion    = 1
	redisKeyFamilyHMAC = "hmac"
	redisKeyFamilyRSA  = "rsa"
)

type keyChainDTO struct {
	Version   int       `json:"version"`
	KID       string    `json:"kid"`
	Algorithm string    `json:"algorithm"`
	Family    string    `json:"family"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	HMAC      string    `json:"hmac,omitempty"`
	RSA       string    `json:"rsa,omitempty"`
}

func encodeRedisKeyChain(key *KeyChain) ([]byte, error) {
	if key == nil {
		return nil, KeyError{Kind: ErrInvalidKey, Err: errorsNew("key must not be nil")}
	}
	dto := keyChainDTO{
		Version:   redisKeyVersion,
		KID:       key.KID(),
		Algorithm: string(key.Algorithm()),
		CreatedAt: key.CreatedAt(),
		ExpiresAt: key.ExpiresAt(),
	}
	if _, ok := key.Algorithm().hmacSecretLength(); ok {
		secret, ok := key.signingMaterial().([]byte)
		if !ok {
			return nil, KeyError{Kind: ErrInvalidKey, KID: key.KID(), Err: errorsNew("hmac key material has invalid type")}
		}
		dto.Family = redisKeyFamilyHMAC
		dto.HMAC = base64.StdEncoding.EncodeToString(secret)
	} else if key.Algorithm().isRSA() {
		privateKey, ok := key.signingMaterial().(*rsa.PrivateKey)
		if !ok {
			return nil, KeyError{Kind: ErrInvalidKey, KID: key.KID(), Err: errorsNew("rsa key material has invalid type")}
		}
		dto.Family = redisKeyFamilyRSA
		dto.RSA = base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(privateKey))
	} else {
		return nil, OptionError{Option: "algorithm", Err: fmt.Errorf("unsupported algorithm %q", key.Algorithm())}
	}
	payload, err := json.Marshal(dto)
	if err != nil {
		return nil, KeyError{Kind: ErrInvalidKey, KID: key.KID(), Err: err}
	}
	return payload, nil
}

func decodeRedisKeyChain(payload []byte, maxKeyBytes int) (*KeyChain, error) {
	if maxKeyBytes <= 0 {
		maxKeyBytes = defaultRedisMaxKeyBytes
	}
	if len(payload) > maxKeyBytes {
		return nil, KeyError{Kind: ErrInvalidKey, Err: errorsNew("redis key payload exceeds max key bytes")}
	}
	var dto keyChainDTO
	if err := json.Unmarshal(payload, &dto); err != nil {
		return nil, KeyError{Kind: ErrInvalidKey, Err: errorsNew("redis key payload is not valid json")}
	}
	if dto.Version != redisKeyVersion {
		return nil, KeyError{Kind: ErrInvalidKey, KID: dto.KID, Err: errorsNew("unsupported redis key version")}
	}
	if dto.KID == "" {
		return nil, KeyError{Kind: ErrInvalidKey, Err: errorsNew("kid must not be empty")}
	}
	algorithm := Algorithm(dto.Algorithm)
	ttl := dto.ExpiresAt.Sub(dto.CreatedAt)
	switch dto.Family {
	case redisKeyFamilyHMAC:
		if _, ok := algorithm.hmacSecretLength(); !ok {
			return nil, KeyError{Kind: ErrInvalidKey, KID: dto.KID, Err: errorsNew("algorithm family mismatch")}
		}
		secret, err := base64.StdEncoding.DecodeString(dto.HMAC)
		if err != nil {
			return nil, KeyError{Kind: ErrInvalidKey, KID: dto.KID, Err: errorsNew("hmac key material is not valid base64")}
		}
		return newHMACKeyChain(dto.KID, algorithm, secret, dto.CreatedAt, ttl)
	case redisKeyFamilyRSA:
		if !algorithm.isRSA() {
			return nil, KeyError{Kind: ErrInvalidKey, KID: dto.KID, Err: errorsNew("algorithm family mismatch")}
		}
		der, err := base64.StdEncoding.DecodeString(dto.RSA)
		if err != nil {
			return nil, KeyError{Kind: ErrInvalidKey, KID: dto.KID, Err: errorsNew("rsa key material is not valid base64")}
		}
		privateKey, err := x509.ParsePKCS1PrivateKey(der)
		if err != nil {
			return nil, KeyError{Kind: ErrInvalidKey, KID: dto.KID, Err: errorsNew("rsa key material is not valid pkcs1")}
		}
		return newRSAKeyChain(dto.KID, algorithm, privateKey, dto.CreatedAt, ttl)
	default:
		return nil, KeyError{Kind: ErrInvalidKey, KID: dto.KID, Err: errorsNew("unsupported key family")}
	}
}
