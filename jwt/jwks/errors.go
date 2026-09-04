package jwks

import (
	"errors"
	"fmt"

	rootjwt "github.com/bluetape4k/bluetape-go/jwt"
)

var (
	// ErrFetch 는 JWKS endpoint fetch 실패를 나타낸다.
	ErrFetch = errors.New("jwks: fetch failed")
	// ErrMalformedSet 은 JWKS JSON 또는 key policy 위반을 나타낸다.
	ErrMalformedSet = errors.New("jwks: malformed key set")
	// ErrUnsupportedAlgorithm 은 허용하지 않는 서명 알고리즘을 나타낸다.
	ErrUnsupportedAlgorithm = errors.New("jwks: unsupported algorithm")

	// ErrInvalidOptions 는 root jwt option sentinel의 alias다.
	ErrInvalidOptions = rootjwt.ErrInvalidOptions
	// ErrInvalidKey 는 root jwt key sentinel의 alias다.
	ErrInvalidKey = rootjwt.ErrInvalidKey
	// ErrKeyNotFound 는 root jwt kid sentinel의 alias다.
	ErrKeyNotFound = rootjwt.ErrKeyNotFound
)

// FetchClass 는 fetch 실패의 low-cardinality 분류다.
type FetchClass string

const (
	// FetchClassTransport 는 transport 또는 context 실패다.
	FetchClassTransport FetchClass = "transport"
	// FetchClassStatus 는 HTTP status 실패다.
	FetchClassStatus FetchClass = "status"
	// FetchClassBody 는 response body 제한 실패다.
	FetchClassBody FetchClass = "body"
	// FetchClassDecode 는 JSON decode 실패다.
	FetchClassDecode FetchClass = "decode"
)

// FetchError 는 endpoint와 원격 payload를 노출하지 않는 fetch 오류다.
type FetchError struct {
	Class  FetchClass
	Status int
	Err    error
}

func (e FetchError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("%v: class=%s status=%d", ErrFetch, e.Class, e.Status)
	}
	if e.Class != "" {
		return fmt.Sprintf("%v: class=%s", ErrFetch, e.Class)
	}
	return ErrFetch.Error()
}

// Unwrap는 sanitization을 통과한 context sentinel만 보존한다.
func (e FetchError) Unwrap() error { return e.Err }

// Is 는 ErrFetch와 sanitization을 통과한 원인 sentinel을 보존한다.
func (e FetchError) Is(target error) bool {
	return target == ErrFetch || errors.Is(e.Err, target)
}

// SetError 는 JWKS key set 오류를 raw key material 없이 감싼다.
type SetError struct {
	Err error
}

func (e SetError) Error() string { return ErrMalformedSet.Error() }

// Unwrap는 내부 root key 오류를 보존한다.
func (e SetError) Unwrap() error { return e.Err }

// Is 는 ErrMalformedSet, ErrInvalidKey와 감싼 원인을 보존한다.
func (e SetError) Is(target error) bool {
	return target == ErrMalformedSet || target == rootjwt.ErrInvalidKey || errors.Is(e.Err, target)
}
