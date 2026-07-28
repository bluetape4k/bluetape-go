package probabilistic

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidConfig Bloom filter 설정이 유효하지 않을 때 반환됩니다.
	ErrInvalidConfig = errors.New("probabilistic: invalid config")
	// ErrIncompatibleFilter 병합 대상 Bloom filter의 설정이나 hasher가 다를 때 반환됩니다.
	ErrIncompatibleFilter = errors.New("probabilistic: incompatible filter")
	// ErrNilFilter nil Bloom filter를 병합하려 할 때 반환됩니다.
	ErrNilFilter = errors.New("probabilistic: nil filter")
	// ErrNilHasher nil hasher 함수로 Bloom filter를 만들 때 반환됩니다.
	ErrNilHasher = errors.New("probabilistic: nil hasher")
	// ErrEmptyHasherKey 빈 hasher compatibility key를 사용할 때 반환됩니다.
	ErrEmptyHasherKey = errors.New("probabilistic: empty hasher key")
)

// ConfigError 유효하지 않은 설정 필드와 원인을 보존합니다.
type ConfigError struct {
	Field string
	Err   error
}

// Error 설정 오류 메시지를 반환합니다.
func (e ConfigError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%v: %v", ErrInvalidConfig, e.Err)
	}
	return fmt.Sprintf("%v: %s: %v", ErrInvalidConfig, e.Field, e.Err)
}

// Unwrap Unwrap 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, compatibility 불일치, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (e ConfigError) Unwrap() error {
	if e.Err == nil {
		return ErrInvalidConfig
	}
	return e.Err
}

// Is ConfigError가 ErrInvalidConfig 또는 원인 오류와 match되도록 합니다.
func (e ConfigError) Is(target error) bool {
	return target == ErrInvalidConfig || errors.Is(e.Err, target)
}
