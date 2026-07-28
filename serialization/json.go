package serialization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// JSONOption는 func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type JSONOption func(*JSONSerializerOptions)

// JSONSerializerOptions는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type JSONSerializerOptions struct {
	DisallowUnknownFields bool
}

// JSONSerializer는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type JSONSerializer[T any] struct {
	options JSONSerializerOptions
}

// NewJSONSerializer는 NewJSONSerializer 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - options: NewJSONSerializer 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func NewJSONSerializer[T any](options ...JSONOption) JSONSerializer[T] {
	serializer := JSONSerializer[T]{}
	for _, option := range options {
		if option != nil {
			option(&serializer.options)
		}
	}
	return serializer
}

// WithDisallowUnknownFields는 WithDisallowUnknownFields 공개 API의 동작을 수행한다.
func WithDisallowUnknownFields() JSONOption {
	return func(options *JSONSerializerOptions) {
		options.DisallowUnknownFields = true
	}
}

// Format는 Format 공개 API의 동작을 수행한다.
func (s JSONSerializer[T]) Format() string {
	return "json"
}

// Marshal는 Marshal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Marshal 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (s JSONSerializer[T]) Marshal(value T) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return data, nil
}

// Unmarshal는 Unmarshal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - data: Unmarshal가 읽거나 복사하는 data 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
