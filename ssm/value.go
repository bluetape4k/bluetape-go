package ssm

const redactedValue = "[REDACTED]"

// Value - SSM parameter value를 감싼 immutable 값이다.
//
// fmt.Stringer와 fmt.GoStringer는 항상 redacted marker를 반환한다. raw 값이
// 필요할 때만 Bytes 또는 Text를 명시적으로 호출한다.
type Value struct {
	data    []byte
	present bool
}

func newValue(value string) Value {
	return Value{data: append([]byte{}, value...), present: true}
}

func (v Value) clone() Value {
	if !v.present {
		return Value{}
	}
	return Value{data: append([]byte{}, v.data...), present: true}
}

// Bytes - raw parameter value의 독립 복사본을 반환한다.
func (v Value) Bytes() []byte {
	if !v.present {
		return nil
	}
	return append([]byte{}, v.data...)
}

// Text - parameter value를 반환한다.
func (v Value) Text() string {
	return string(v.data)
}

// IsBinary - SSM 값이 binary인지 반환한다. SSM Parameter Store 값은 항상
// 문자열이므로 이 메서드는 API pair 호환성을 위해 false를 반환한다.
func (v Value) IsBinary() bool {
	return false
}

// IsSet - zero Value와 empty-but-present parameter를 구분한다.
func (v Value) IsSet() bool {
	return v.present
}

// Len - raw parameter value의 byte 길이를 반환한다.
func (v Value) Len() int {
	return len(v.data)
}

// String는 비밀값을 숨긴다.
func (v Value) String() string {
	return redactedValue
}

// GoString - %#v 포맷에서도 비밀값을 숨긴다.
func (v Value) GoString() string {
	return redactedValue
}
