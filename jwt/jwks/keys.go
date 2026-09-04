package jwks

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"

	rootjwt "github.com/bluetape4k/bluetape-go/jwt"
	jose "github.com/go-jose/go-jose/v4"
)

const maxKeyCount = 256

type keyRecord struct {
	key       any
	algorithm Algorithm
}

type rawKeySet struct {
	Keys []json.RawMessage `json:"keys"`
}

type rawKeyMetadata struct {
	Kty    string   `json:"kty"`
	Kid    string   `json:"kid"`
	Crv    string   `json:"crv"`
	Alg    string   `json:"alg"`
	Use    string   `json:"use"`
	KeyOps []string `json:"key_ops"`
	K      string   `json:"k"`
	D      string   `json:"d"`
	P      string   `json:"p"`
	Q      string   `json:"q"`
	Dp     string   `json:"dp"`
	Dq     string   `json:"dq"`
	Qi     string   `json:"qi"`
	X      string   `json:"x"`
	Y      string   `json:"y"`
	N      string   `json:"n"`
	E      string   `json:"e"`
}

func parseKeySet(data []byte) (map[string]keyRecord, error) {
	var raw rawKeySet
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, SetError{Err: err}
	}
	if len(raw.Keys) == 0 || len(raw.Keys) > maxKeyCount {
		return nil, SetError{Err: errors.New("invalid key count")}
	}
	for _, rawKey := range raw.Keys {
		if err := validateRawKey(rawKey); err != nil {
			return nil, err
		}
	}

	var set jose.JSONWebKeySet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, SetError{Err: err}
	}
	keys := make(map[string]keyRecord, len(set.Keys))
	for _, key := range set.Keys {
		if !key.IsPublic() || !key.Valid() {
			return nil, SetError{Err: rootjwt.ErrInvalidKey}
		}
		if key.KeyID == "" || !validKID(key.KeyID) {
			return nil, SetError{Err: rootjwt.ErrInvalidKey}
		}
		if _, exists := keys[key.KeyID]; exists {
			return nil, SetError{Err: errors.New("duplicate key id")}
		}
		if key.Use != "" && key.Use != "sig" {
			return nil, SetError{Err: rootjwt.ErrInvalidKey}
		}
		algorithm := Algorithm(key.Algorithm)
		if algorithm != "" && !isSupportedAlgorithm(algorithm) {
			return nil, SetError{Err: ErrUnsupportedAlgorithm}
		}
		if err := validatePublicKey(algorithm, key.Key); err != nil {
			return nil, err
		}
		keys[key.KeyID] = keyRecord{key: key.Key, algorithm: algorithm}
	}
	return keys, nil
}

func validateRawKey(data []byte) error {
	var raw rawKeyMetadata
	if err := json.Unmarshal(data, &raw); err != nil {
		return SetError{Err: err}
	}
	if !validKID(raw.Kid) {
		return SetError{Err: rootjwt.ErrInvalidKey}
	}
	if raw.Use != "" && raw.Use != "sig" {
		return SetError{Err: rootjwt.ErrInvalidKey}
	}
	if len(raw.KeyOps) > 0 {
		if len(raw.KeyOps) != 1 || raw.KeyOps[0] != "verify" {
			return SetError{Err: rootjwt.ErrInvalidKey}
		}
	}
	if raw.K != "" || raw.D != "" || raw.P != "" || raw.Q != "" || raw.Dp != "" || raw.Dq != "" || raw.Qi != "" {
		return SetError{Err: rootjwt.ErrInvalidKey}
	}
	if raw.Alg != "" && !isSupportedAlgorithm(Algorithm(raw.Alg)) {
		return SetError{Err: ErrUnsupportedAlgorithm}
	}
	switch raw.Kty {
	case "RSA":
		if err := validateRSAExponent(raw.E); err != nil {
			return err
		}
	case "OKP":
		if raw.Crv == "Ed25519" {
			decoded, err := decodeRawURL(raw.X)
			if err != nil || len(decoded) != ed25519.PublicKeySize {
				return SetError{Err: rootjwt.ErrInvalidKey}
			}
		}
	}
	return nil
}

func validateRSAExponent(encoded string) error {
	decoded, err := decodeRawURL(encoded)
	if err != nil || len(decoded) == 0 {
		return SetError{Err: rootjwt.ErrInvalidKey}
	}
	exponent := new(big.Int).SetBytes(decoded)
	if exponent.Cmp(big.NewInt(3)) < 0 || exponent.Bit(0) == 0 || exponent.BitLen() > strconv.IntSize-1 {
		return SetError{Err: rootjwt.ErrInvalidKey}
	}
	return nil
}

func validatePublicKey(algorithm Algorithm, key any) error {
	switch typed := key.(type) {
	case *rsa.PublicKey:
		if typed == nil || typed.N == nil || typed.N.BitLen() < 2048 || typed.E < 3 || typed.E%2 == 0 {
			return SetError{Err: rootjwt.ErrInvalidKey}
		}
		if algorithm != "" && !isRSAAlgorithm(algorithm) {
			return SetError{Err: ErrUnsupportedAlgorithm}
		}
	case *ecdsa.PublicKey:
		if typed == nil || typed.Curve == nil {
			return SetError{Err: rootjwt.ErrInvalidKey}
		}
		encoded, err := typed.Bytes()
		if err != nil {
			return SetError{Err: rootjwt.ErrInvalidKey}
		}
		if _, err := ecdsa.ParseUncompressedPublicKey(typed.Curve, encoded); err != nil {
			return SetError{Err: rootjwt.ErrInvalidKey}
		}
		expected := curveAlgorithm(typed.Curve)
		if expected == "" || (algorithm != "" && algorithm != expected) {
			return SetError{Err: rootjwt.ErrInvalidKey}
		}
	case ed25519.PublicKey:
		if len(typed) != ed25519.PublicKeySize || (algorithm != "" && algorithm != EdDSA) {
			return SetError{Err: rootjwt.ErrInvalidKey}
		}
	default:
		return SetError{Err: rootjwt.ErrInvalidKey}
	}
	return nil
}

func curveAlgorithm(curve elliptic.Curve) Algorithm {
	switch curve {
	case elliptic.P256():
		return ES256
	case elliptic.P384():
		return ES384
	case elliptic.P521():
		return ES512
	default:
		return ""
	}
}

func isRSAAlgorithm(algorithm Algorithm) bool {
	switch algorithm {
	case RS256, RS384, RS512, PS256, PS384, PS512:
		return true
	default:
		return false
	}
}

func validKID(kid string) bool {
	if kid == "" || len(kid) > 128 {
		return false
	}
	for i := 0; i < len(kid); i++ {
		if kid[i] < 0x21 || kid[i] > 0x7e {
			return false
		}
	}
	return true
}

func decodeRawURL(value string) ([]byte, error) {
	if value == "" {
		return nil, errors.New("missing value")
	}
	return base64.RawURLEncoding.DecodeString(value)
}

func cloneKey(key any) any {
	switch typed := key.(type) {
	case *rsa.PublicKey:
		if typed == nil {
			return (*rsa.PublicKey)(nil)
		}
		return &rsa.PublicKey{N: new(big.Int).Set(typed.N), E: typed.E}
	case *ecdsa.PublicKey:
		if typed == nil {
			return (*ecdsa.PublicKey)(nil)
		}
		encoded, err := typed.Bytes()
		if err != nil {
			return nil
		}
		cloned, err := ecdsa.ParseUncompressedPublicKey(typed.Curve, encoded)
		if err != nil {
			return nil
		}
		return cloned
	case ed25519.PublicKey:
		return append(ed25519.PublicKey(nil), typed...)
	default:
		return nil
	}
}
