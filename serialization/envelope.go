package serialization

import (
	"encoding/binary"
	"fmt"
)

const (
	envelopeMagic = "BTGS"
	envelopeV1    = 1
)

// VersionedSerializer 패키지에서 공개하는 구조체다.
type VersionedSerializer[T any] struct {
	serializer NamedSerializer[T]
	version    uint16
}

// NewVersionedSerializer VersionedSerializer 인스턴스를 생성한다.
//
// 매개변수:
//   - serializer: NewVersionedSerializer에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - version: NewVersionedSerializer에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewVersionedSerializer[T any](serializer NamedSerializer[T], version uint16) (VersionedSerializer[T], error) {
	if serializer == nil {
		return VersionedSerializer[T]{}, fmt.Errorf("serializer must not be nil")
	}
	if serializer.Format() == "" {
		return VersionedSerializer[T]{}, fmt.Errorf("serializer format must not be empty")
	}
	if len(serializer.Format()) > 255 {
		return VersionedSerializer[T]{}, fmt.Errorf("serializer format must fit in 255 bytes")
	}
	if version == 0 {
		return VersionedSerializer[T]{}, fmt.Errorf("serializer version must be positive")
	}

	return VersionedSerializer[T]{
		serializer: serializer,
		version:    version,
	}, nil
}

// Format 값을 지정한 형식의 문자열로 변환한다.
func (s VersionedSerializer[T]) Format() string {
	return s.serializer.Format()
}

// Version 직렬화 envelope의 version 값을 반환한다.
func (s VersionedSerializer[T]) Version() uint16 {
	return s.version
}

// Marshal 값을 직렬화된 바이트로 변환한다.
//
// 매개변수:
//   - value: Marshal에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (s VersionedSerializer[T]) Marshal(value T) ([]byte, error) {
	payload, err := s.serializer.Marshal(value)
	if err != nil {
		return nil, err
	}

	format := []byte(s.serializer.Format())
	headerLen := len(envelopeMagic) + 2 + 1 + len(format)
	result := make([]byte, headerLen+len(payload))
	copy(result, envelopeMagic)
	binary.BigEndian.PutUint16(result[4:6], s.version)
	result[6] = byte(len(format))
	copy(result[7:], format)
	copy(result[headerLen:], payload)
	return result, nil
}

// Unmarshal 직렬화된 데이터를 대상 값으로 복원한다.
//
// 매개변수:
//   - data: Unmarshal가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (s VersionedSerializer[T]) Unmarshal(data []byte) (T, error) {
	var zero T
	payload, err := s.payload(data)
	if err != nil {
		return zero, err
	}
	return s.serializer.Unmarshal(payload)
}

func (s VersionedSerializer[T]) payload(data []byte) ([]byte, error) {
	if len(data) < 7 {
		return nil, fmt.Errorf("%w: header too short", ErrInvalidEnvelope)
	}
	if string(data[:4]) != envelopeMagic {
		return nil, fmt.Errorf("%w: bad magic", ErrInvalidEnvelope)
	}

	version := binary.BigEndian.Uint16(data[4:6])
	if version > s.version {
		return nil, fmt.Errorf("%w: got %d want <= %d", ErrUnsupportedVersion, version, s.version)
	}

	formatLen := int(data[6])
	headerLen := 7 + formatLen
	if len(data) < headerLen {
		return nil, fmt.Errorf("%w: format truncated", ErrInvalidEnvelope)
	}
	format := string(data[7:headerLen])
	if format != s.serializer.Format() {
		return nil, fmt.Errorf("%w: got %q want %q", ErrFormatMismatch, format, s.serializer.Format())
	}
	return data[headerLen:], nil
}
