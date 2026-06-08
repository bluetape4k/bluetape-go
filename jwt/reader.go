package jwt

import (
	"fmt"
	"time"

	golangjwt "github.com/golang-jwt/jwt/v5"
)

// Reader 는 검증된 JWT header와 claim을 읽는다.
type Reader struct {
	kid       string
	algorithm Algorithm
	headers   map[string]any
	claims    golangjwt.MapClaims
}

// Kid 는 검증된 token의 kid header를 반환한다.
func (r *Reader) Kid() string {
	if r == nil {
		return ""
	}
	return r.kid
}

// Algorithm 은 검증된 token의 alg header를 반환한다.
func (r *Reader) Algorithm() Algorithm {
	if r == nil {
		return ""
	}
	return r.algorithm
}

// Header 는 header 값을 반환한다.
func (r *Reader) Header(name string) (any, bool) {
	if r == nil {
		return nil, false
	}
	value, ok := r.headers[name]
	return copyValue(value), ok
}

// Claim 는 claim 값을 반환한다.
func (r *Reader) Claim(name string) (any, bool) {
	if r == nil {
		return nil, false
	}
	value, ok := r.claims[name]
	return copyValue(value), ok
}

// ClaimString 은 string claim 값을 반환한다.
func (r *Reader) ClaimString(name string) (string, bool) {
	value, ok := r.Claim(name)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

// ClaimTime 은 numeric-date claim 값을 time으로 반환한다.
func (r *Reader) ClaimTime(name string) (time.Time, bool) {
	if r == nil {
		return time.Time{}, false
	}
	date, err := r.claims.GetExpirationTime()
	if name == "exp" && err == nil && date != nil {
		return date.Time, true
	}
	date, err = r.claims.GetNotBefore()
	if name == "nbf" && err == nil && date != nil {
		return date.Time, true
	}
	date, err = r.claims.GetIssuedAt()
	if name == "iat" && err == nil && date != nil {
		return date.Time, true
	}
	return time.Time{}, false
}

// Issuer 는 iss claim을 반환한다.
func (r *Reader) Issuer() string {
	if r == nil {
		return ""
	}
	value, _ := r.claims.GetIssuer()
	return value
}

// Subject 는 sub claim을 반환한다.
func (r *Reader) Subject() string {
	if r == nil {
		return ""
	}
	value, _ := r.claims.GetSubject()
	return value
}

// Audience 는 aud claim을 반환한다.
func (r *Reader) Audience() []string {
	if r == nil {
		return nil
	}
	value, err := r.claims.GetAudience()
	if err != nil {
		return nil
	}
	return append([]string(nil), value...)
}

// ExpiresAt 은 exp claim을 반환한다.
func (r *Reader) ExpiresAt() (time.Time, bool) {
	return r.ClaimTime("exp")
}

// NotBefore 은 nbf claim을 반환한다.
func (r *Reader) NotBefore() (time.Time, bool) {
	return r.ClaimTime("nbf")
}

// IssuedAt 은 iat claim을 반환한다.
func (r *Reader) IssuedAt() (time.Time, bool) {
	return r.ClaimTime("iat")
}

// IsExpired 는 now 기준 만료 여부를 반환한다.
func (r *Reader) IsExpired(now time.Time) bool {
	expiresAt, ok := r.ExpiresAt()
	return ok && !now.Before(expiresAt)
}

// RemainingTTL 은 now 기준 남은 TTL을 반환한다.
func (r *Reader) RemainingTTL(now time.Time) time.Duration {
	expiresAt, ok := r.ExpiresAt()
	if !ok {
		return 0
	}
	remaining := expiresAt.Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func newReader(token *golangjwt.Token, claims golangjwt.MapClaims) (*Reader, error) {
	headers, err := copyHeaders(token.Header)
	if err != nil {
		return nil, err
	}
	kid, _ := headers["kid"].(string)
	alg, _ := headers["alg"].(string)
	return &Reader{
		kid:       kid,
		algorithm: Algorithm(alg),
		headers:   headers,
		claims:    copyClaims(claims),
	}, nil
}

func copyHeaders(headers map[string]any) (map[string]any, error) {
	copied := make(map[string]any, len(headers))
	for key, value := range headers {
		if _, reserved := reservedHeaders[key]; reserved && key != "alg" && key != "kid" {
			return nil, TokenError{Kind: ErrInvalidToken, Err: fmt.Errorf("unsupported header %q", key)}
		}
		copied[key] = copyValue(value)
	}
	return copied, nil
}

func copyClaims(claims golangjwt.MapClaims) golangjwt.MapClaims {
	copied := make(golangjwt.MapClaims, len(claims))
	for key, value := range claims {
		copied[key] = copyValue(value)
	}
	return copied
}

func copyValue(value any) any {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		return append([]any(nil), typed...)
	case map[string]any:
		copied := make(map[string]any, len(typed))
		for key, item := range typed {
			copied[key] = copyValue(item)
		}
		return copied
	default:
		return value
	}
}
