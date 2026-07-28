package serialization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// JSONOption func 공개 타입이다.
type JSONOption func(*JSONSerializerOptions)

// JSONSerializerOptions 패키지에서 공개하는 구조체다.
type JSONSerializerOptions struct {
	DisallowUnknownFields bool
}

// JSONSerializer 패키지에서 공개하는 구조체다.
type JSONSerializer[T any] struct {
	options JSONSerializerOptions
}

// NewJSONSerializer JSONSerializer 인스턴스를 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
func NewJSONSerializer[T any](options ...JSONOption) JSONSerializer[T] {
	serializer := JSONSerializer[T]{}
	for _, option := range options {
		if option != nil {
			option(&serializer.options)
		}
	}
	return serializer
}

// WithDisallowUnknownFields DisallowUnknownFields 설정을 적용한 옵션을 반환한다.
func WithDisallowUnknownFields() JSONOption {
	return func(options *JSONSerializerOptions) {
		options.DisallowUnknownFields = true
	}
}

// Format 값을 지정한 형식의 문자열로 변환한다.
func (s JSONSerializer[T]) Format() string {
	return "json"
}

// Marshal 값을 직렬화된 바이트로 변환한다.
//
// 매개변수:
//   - value: Marshal에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (s JSONSerializer[T]) Marshal(value T) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return data, nil
}

// Unmarshal 직렬화된 데이터를 대상 값으로 복원한다.
//
// 매개변수:
//   - data: Unmarshal가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (s JSONSerializer[T]) Unmarshal(data []byte) (T, error) {
	var value T
	if len(data) == 0 {
		return value, fmt.Errorf("unmarshal json: input must not be empty")
	}

	if !s.options.DisallowUnknownFields {
		if err := json.Unmarshal(data, &value); err != nil {
			return value, fmt.Errorf("unmarshal json: %w", err)
		}
		return value, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("unmarshal json: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return value, fmt.Errorf("unmarshal json: trailing data")
	}
	return value, nil
}
