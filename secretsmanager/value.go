package secretsmanager

const redactedValue = "[REDACTED]"

// Value - Secrets Manager의 string 또는 binary secret을 감싼 immutable 값이다.
//
// fmt.Stringer와 fmt.GoStringer는 항상 redacted marker를 반환한다. raw 값이
// 필요할 때만 Bytes 또는 Text를 명시적으로 호출한다.
type Value struct {
	data    []byte
	binary  bool
	present bool
}

func newTextValue(value string) Value {
	return Value{data: append([]byte{}, value...), present: true}
}

func newBinaryValue(value []byte) Value {
	return Value{data: append([]byte{}, value...), binary: true, present: true}
}

func (v Value) clone() Value {
	if !v.present {
		return Value{}
	}
	return Value{data: append([]byte{}, v.data...), binary: v.binary, present: true}
}

// Bytes - raw value의 독립 복사본을 반환한다. 반환된 slice를 수정해도
// provider나 cache에 저장된 값은 변경되지 않는다.
func (v Value) Bytes() []byte {
	if !v.present {
		return nil
	}
	return append([]byte{}, v.data...)
}

// Text - value를 string으로 반환한다. binary secret은 호출자가 IsBinary로
// 구분한 뒤 Bytes를 사용하는 것이 안전하다.
func (v Value) Text() string {
	if v.binary {
		return ""
	}
	return string(v.data)
}

// IsBinary - 값이 Secrets Manager SecretBinary에서 왔는지 반환한다.
func (v Value) IsBinary() bool {
	return v.present && v.binary
}

// IsSet - zero Value와 empty-but-present secret을 구분한다.
func (v Value) IsSet() bool {
	return v.present
}

// Len - raw value의 byte 길이를 반환한다.
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
