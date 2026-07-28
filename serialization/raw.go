package serialization

import (
	"fmt"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/core"
)

// BytesSerializer 패키지에서 공개하는 구조체다.
type BytesSerializer struct{}

// Format 값을 지정한 형식의 문자열로 변환한다.
func (BytesSerializer) Format() string {
	return "bytes"
}

// Marshal 값을 직렬화된 바이트로 변환한다.
//
// 매개변수:
//   - value: Marshal가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (BytesSerializer) Marshal(value []byte) ([]byte, error) {
	copied := make([]byte, len(value))
	copy(copied, value)
	return copied, nil
}

// Unmarshal 직렬화된 데이터를 대상 값으로 복원한다.
//
// 매개변수:
//   - data: Unmarshal가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (BytesSerializer) Unmarshal(data []byte) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("unmarshal bytes: input must not be nil")
	}
	copied := make([]byte, len(data))
	copy(copied, data)
	return copied, nil
}

// StringSerializer 패키지에서 공개하는 구조체다.
type StringSerializer struct{}

// Format 값을 지정한 형식의 문자열로 변환한다.
func (StringSerializer) Format() string {
	return "string"
}

// Marshal 값을 직렬화된 바이트로 변환한다.
//
// 매개변수:
//   - value: Marshal가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (StringSerializer) Marshal(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("marshal string: %w", core.ErrInvalidUTF8)
	}
	return []byte(value), nil
}

// Unmarshal 직렬화된 데이터를 대상 값으로 복원한다.
//
// 매개변수:
//   - data: Unmarshal가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (StringSerializer) Unmarshal(data []byte) (string, error) {
	if data == nil {
		return "", fmt.Errorf("unmarshal string: input must not be nil")
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("unmarshal string: %w", core.ErrInvalidUTF8)
	}
	return string(data), nil
}
