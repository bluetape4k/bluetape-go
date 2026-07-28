package jwt

import (
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"
)

var reservedHeaders = map[string]struct{}{
	"alg":  {},
	"kid":  {},
	"zip":  {},
	"crit": {},
	"jku":  {},
	"jwk":  {},
	"x5u":  {},
	"x5c":  {},
}

var reservedClaims = map[string]struct{}{
	"iss": {},
	"sub": {},
	"aud": {},
	"exp": {},
	"nbf": {},
	"iat": {},
	"jti": {},
}

type composeConfig struct {
	headers map[string]any
	claims  golangjwt.MapClaims
}

// ComposeOption 은 JWT 생성 option이다.
type ComposeOption func(*composeConfig) error

// WithHeader 안전한 custom header를 추가한다.
func WithHeader(name string, value any) ComposeOption {
	return func(cfg *composeConfig) error {
		if _, exists := reservedHeaders[name]; exists {
			return OptionError{Option: "header", Err: errorsNew("reserved header")}
		}
		cfg.headers[name] = value
		return nil
	}
}

// WithClaim 은 안전한 custom claim을 추가한다.
func WithClaim(name string, value any) ComposeOption {
	return func(cfg *composeConfig) error {
		if _, exists := reservedClaims[name]; exists {
			return OptionError{Option: "claim", Err: errorsNew("reserved claim")}
		}
		cfg.claims[name] = value
		return nil
	}
}

// WithIssuer iss claim을 지정한다.
func WithIssuer(issuer string) ComposeOption {
	return func(cfg *composeConfig) error {
		cfg.claims["iss"] = issuer
		return nil
	}
}

// WithSubject sub claim을 지정한다.
func WithSubject(subject string) ComposeOption {
	return func(cfg *composeConfig) error {
		cfg.claims["sub"] = subject
		return nil
	}
}

// WithAudience aud claim을 지정한다.
func WithAudience(audience ...string) ComposeOption {
	return func(cfg *composeConfig) error {
		copied := append([]string(nil), audience...)
		cfg.claims["aud"] = copied
		return nil
	}
}

// WithIssuedAt 은 iat claim을 지정한다.
func WithIssuedAt(issuedAt time.Time) ComposeOption {
	return func(cfg *composeConfig) error {
		cfg.claims["iat"] = golangjwt.NewNumericDate(issuedAt)
		return nil
	}
}

// WithNotBefore nbf claim을 지정한다.
func WithNotBefore(notBefore time.Time) ComposeOption {
	return func(cfg *composeConfig) error {
		cfg.claims["nbf"] = golangjwt.NewNumericDate(notBefore)
		return nil
	}
}

// WithExpiresAt 은 exp claim을 지정한다.
func WithExpiresAt(expiresAt time.Time) ComposeOption {
	return func(cfg *composeConfig) error {
		cfg.claims["exp"] = golangjwt.NewNumericDate(expiresAt)
		return nil
	}
}

// WithExpiresAfter provider clock 기준 exp claim을 지정한다.
func WithExpiresAfter(ttl time.Duration) ComposeOption {
	return func(cfg *composeConfig) error {
		if ttl <= 0 {
			return OptionError{Option: "expires_after", Err: errorsNew("must be positive")}
		}
		cfg.claims["expires_after"] = ttl
		return nil
	}
}

// WithJWTID jti claim을 지정한다.
func WithJWTID(id string) ComposeOption {
	return func(cfg *composeConfig) error {
		cfg.claims["jti"] = id
		return nil
	}
}

func newComposeConfig() composeConfig {
	return composeConfig{
		headers: make(map[string]any),
		claims:  make(golangjwt.MapClaims),
	}
}

func (cfg composeConfig) build(now time.Time) (map[string]any, golangjwt.MapClaims) {
	headers := make(map[string]any, len(cfg.headers)+1)
	headers["typ"] = "JWT"
	for key, value := range cfg.headers {
		headers[key] = value
	}
	claims := make(golangjwt.MapClaims, len(cfg.claims)+1)
	for key, value := range cfg.claims {
		claims[key] = value
	}
	if ttl, ok := claims["expires_after"].(time.Duration); ok {
		claims["exp"] = golangjwt.NewNumericDate(now.Add(ttl))
		delete(claims, "expires_after")
	}
	if _, exists := claims["iat"]; !exists {
		claims["iat"] = golangjwt.NewNumericDate(now)
	}
	return headers, claims
}
