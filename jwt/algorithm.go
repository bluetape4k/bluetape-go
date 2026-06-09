package jwt

import (
	"fmt"

	golangjwt "github.com/golang-jwt/jwt/v5"
)

// Algorithm 은 JWT 서명 알고리즘 이름이다.
type Algorithm string

const (
	// HS256 은 HMAC SHA-256 서명 알고리즘이다.
	HS256 Algorithm = "HS256"
	// HS384 은 HMAC SHA-384 서명 알고리즘이다.
	HS384 Algorithm = "HS384"
	// HS512 은 HMAC SHA-512 서명 알고리즘이다.
	HS512 Algorithm = "HS512"
	// RS256 은 RSA PKCS#1 v1.5 SHA-256 서명 알고리즘이다.
	RS256 Algorithm = "RS256"
	// RS384 은 RSA PKCS#1 v1.5 SHA-384 서명 알고리즘이다.
	RS384 Algorithm = "RS384"
	// RS512 은 RSA PKCS#1 v1.5 SHA-512 서명 알고리즘이다.
	RS512 Algorithm = "RS512"
	// PS256 은 RSA-PSS SHA-256 서명 알고리즘이다.
	PS256 Algorithm = "PS256"
	// PS384 은 RSA-PSS SHA-384 서명 알고리즘이다.
	PS384 Algorithm = "PS384"
	// PS512 은 RSA-PSS SHA-512 서명 알고리즘이다.
	PS512 Algorithm = "PS512"
)

func (a Algorithm) signingMethod() (golangjwt.SigningMethod, error) {
	switch a {
	case HS256:
		return golangjwt.SigningMethodHS256, nil
	case HS384:
		return golangjwt.SigningMethodHS384, nil
	case HS512:
		return golangjwt.SigningMethodHS512, nil
	case RS256:
		return golangjwt.SigningMethodRS256, nil
	case RS384:
		return golangjwt.SigningMethodRS384, nil
	case RS512:
		return golangjwt.SigningMethodRS512, nil
	case PS256:
		return golangjwt.SigningMethodPS256, nil
	case PS384:
		return golangjwt.SigningMethodPS384, nil
	case PS512:
		return golangjwt.SigningMethodPS512, nil
	default:
		return nil, OptionError{Option: "algorithm", Err: fmt.Errorf("unsupported algorithm %q", a)}
	}
}

func (a Algorithm) hmacSecretLength() (int, bool) {
	switch a {
	case HS256:
		return 32, true
	case HS384:
		return 48, true
	case HS512:
		return 64, true
	default:
		return 0, false
	}
}

func (a Algorithm) isRSA() bool {
	switch a {
	case RS256, RS384, RS512, PS256, PS384, PS512:
		return true
	default:
		return false
	}
}

func validateHMACSecret(algorithm Algorithm, secret []byte) error {
	minimum, ok := algorithm.hmacSecretLength()
	if !ok {
		return OptionError{Option: "algorithm", Err: fmt.Errorf("unsupported hmac algorithm %q", algorithm)}
	}
	if len(secret) < minimum {
		return KeyError{Kind: ErrInvalidKey, Err: fmt.Errorf("hmac secret must be at least %d bytes", minimum)}
	}
	return nil
}
