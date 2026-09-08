package geocoding

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidCoordinate 는 유효하지 않은 WGS84 좌표를 나타낸다.
	ErrInvalidCoordinate = errors.New("geocoding: invalid coordinate")
	// ErrInvalidOptions 는 endpoint 또는 요청 option이 유효하지 않음을 나타낸다.
	ErrInvalidOptions = errors.New("geocoding: invalid options")
	// ErrNoResult 는 provider가 좌표의 결과를 찾지 못했음을 나타낸다.
	ErrNoResult = errors.New("geocoding: no result")
	// ErrProvider 는 provider transport 또는 HTTP 오류를 나타낸다.
	ErrProvider = errors.New("geocoding: provider error")
	// ErrRateLimited 는 provider 또는 caller rate limiter가 요청을 거부했음을 나타낸다.
	ErrRateLimited = errors.New("geocoding: rate limited")
	// ErrTimeout 는 caller 또는 HTTP timeout을 나타낸다.
	ErrTimeout = errors.New("geocoding: timeout")
	// ErrParse 는 provider 응답을 해석할 수 없음을 나타낸다.
	ErrParse = errors.New("geocoding: parse error")
	// ErrResponseTooLarge 는 bounded response limit을 초과했음을 나타낸다.
	ErrResponseTooLarge = errors.New("geocoding: response too large")
	// ErrCache 는 caller-owned cache hook 오류를 나타낸다.
	ErrCache = errors.New("geocoding: cache error")
)

// Error 는 public error 문자열에 URL, payload, credential을 노출하지 않는 분류 오류다.
type Error struct {
	Kind       error
	StatusCode int
	Cause      error
}

// Error 는 안정된 kind와 HTTP status만 반환한다.
func (e *Error) Error() string {
	if e == nil || e.Kind == nil {
		return ErrProvider.Error()
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("%v: status=%d", e.Kind, e.StatusCode)
	}
	return e.Kind.Error()
}

// Unwrap 는 내부 원인을 보존하되 문자열에는 포함하지 않는다.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is 는 sentinel 분류와 errors.Is 호환성을 제공한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Cause, target)
}

func classified(kind error, status int, cause error) error {
	return &Error{Kind: kind, StatusCode: status, Cause: cause}
}
