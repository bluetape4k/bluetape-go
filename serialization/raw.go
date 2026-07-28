package serialization

import (
	"fmt"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/core"
)

// BytesSerializer는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type BytesSerializer struct{}

// Format는 Format 공개 API의 동작을 수행한다.
func (BytesSerializer) Format() string {
	return "bytes"
}

// Marshal는 Marshal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Marshal가 읽거나 복사하는 value 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (BytesSerializer) Marshal(value []byte) ([]byte, error) {
	copied := make([]byte, len(value))
	copy(copied, value)
	return copied, nil
}

// Unmarshal는 Unmarshal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - data: Unmarshal가 읽거나 복사하는 data 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (BytesSerializer) Unmarshal(data []byte) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("unmarshal bytes: input must not be nil")
	}
	copied := make([]byte, len(data))
	copy(copied, data)
	return copied, nil
}

// StringSerializer는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type StringSerializer struct{}

// Format는 Format 공개 API의 동작을 수행한다.
func (StringSerializer) Format() string {
	return "string"
}

// Marshal는 Marshal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Marshal가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (StringSerializer) Marshal(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("marshal string: %w", core.ErrInvalidUTF8)
	}
	return []byte(value), nil
}

// Unmarshal는 Unmarshal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - data: Unmarshal가 읽거나 복사하는 data 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (StringSerializer) Unmarshal(data []byte) (string, error) {
	if data == nil {
		return "", fmt.Errorf("unmarshal string: input must not be nil")
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("unmarshal string: %w", core.ErrInvalidUTF8)
	}
	return string(data), nil
}
