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

// ConfigError struct 공개 타입이며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ConfigError struct {
	Field string
	Err   error
}

// Error Error 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
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

// Is Is 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - target: Is에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (e ConfigError) Is(target error) bool {
	return target == ErrInvalidConfig || errors.Is(e.Err, target)
}
