package jwt

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidOptions provider 또는 option 설정 오류를 나타낸다.
	ErrInvalidOptions = errors.New("jwt: invalid options")
	// ErrInvalidToken 은 token 형식, 서명, header, claim 검증 오류를 나타낸다.
	ErrInvalidToken = errors.New("jwt: invalid token")
	// ErrInvalidKey key material 또는 key 설정 오류를 나타낸다.
	ErrInvalidKey = errors.New("jwt: invalid key")
	// ErrKeyNotFound kid로 검증 key를 찾지 못했음을 나타낸다.
	ErrKeyNotFound = errors.New("jwt: key not found")
	// ErrExpiredToken 은 token 만료 오류를 나타낸다.
	ErrExpiredToken = errors.New("jwt: expired token")
	// ErrNotYetValid token이 아직 유효하지 않음을 나타낸다.
	ErrNotYetValid = errors.New("jwt: token not yet valid")
)

// OptionError 잘못된 option 이름과 원인을 보존한다.
type OptionError struct {
	Option string
	Err    error
}

func (e OptionError) Error() string {
	if e.Option == "" {
		return fmt.Sprintf("%v: %v", ErrInvalidOptions, e.Err)
	}
	return fmt.Sprintf("%v: %s: %v", ErrInvalidOptions, e.Option, e.Err)
}

func (e OptionError) Unwrap() error { return e.Err }

// Is ErrInvalidOptions 또는 감싼 원인과의 일치를 보고한다.
func (e OptionError) Is(target error) bool {
	return target == ErrInvalidOptions || errors.Is(e.Err, target)
}

// KeyError key 관련 오류를 sentinel과 함께 감싼다.
type KeyError struct {
	Kind error
	KID  string
	Err  error
}

func (e KeyError) Error() string {
	kind := e.Kind
	if kind == nil {
		kind = ErrInvalidKey
	}
	if e.KID == "" {
		return fmt.Sprintf("%v: %v", kind, e.Err)
	}
	return fmt.Sprintf("%v: kid=%q: %v", kind, e.KID, e.Err)
}

func (e KeyError) Unwrap() error { return e.Err }

// Is key sentinel 또는 감싼 원인과의 일치를 보고한다.
func (e KeyError) Is(target error) bool {
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Err, target)
}

// TokenError token 문자열을 노출하지 않고 검증 오류를 감싼다.
type TokenError struct {
	Kind error
	Err  error
}

func (e TokenError) Error() string {
	kind := e.Kind
	if kind == nil {
		kind = ErrInvalidToken
	}
	if e.Err == nil {
		return kind.Error()
	}
	return fmt.Sprintf("%v: %v", kind, e.Err)
}

func (e TokenError) Unwrap() error { return e.Err }

// Is token sentinel 또는 감싼 원인과의 일치를 보고한다.
func (e TokenError) Is(target error) bool {
	if target == ErrInvalidToken && (e.Kind == ErrExpiredToken || e.Kind == ErrNotYetValid) {
		return true
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Err, target)
}

func errorsNew(text string) error {
	return errors.New(text)
}
