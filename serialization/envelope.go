package serialization

import (
	"encoding/binary"
	"fmt"
)

const (
	envelopeMagic = "BTGS"
	envelopeV1    = 1
)

// VersionedSerializer는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type VersionedSerializer[T any] struct {
	serializer NamedSerializer[T]
	version    uint16
}

// NewVersionedSerializer는 NewVersionedSerializer 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - serializer: NewVersionedSerializer 동작에 필요한 serializer 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - version: NewVersionedSerializer 동작에 필요한 version 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Format는 Format 공개 API의 동작을 수행한다.
func (s VersionedSerializer[T]) Format() string {
	return s.serializer.Format()
}

// Version는 Version 공개 API의 동작을 수행한다.
func (s VersionedSerializer[T]) Version() uint16 {
	return s.version
}

// Marshal는 Marshal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: Marshal 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Unmarshal는 Unmarshal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - data: Unmarshal가 읽거나 복사하는 data 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
