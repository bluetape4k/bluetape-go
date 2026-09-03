package auth

const redactedToken = "[REDACTED]"

// Token - RDS IAM authentication token을 감싼 immutable 값이다.
//
// fmt.Stringer와 fmt.GoStringer는 항상 redacted marker를 반환한다. database
// driver에 전달할 raw token은 Text 또는 Bytes를 명시적으로 호출한다.
type Token struct {
	data    string
	present bool
}

func newToken(value string) Token {
	return Token{data: value, present: true}
}

// Text - database driver에 전달할 raw token을 반환한다.
func (t Token) Text() string {
	return t.data
}

// Bytes - raw token의 독립 byte 복사본을 반환한다.
func (t Token) Bytes() []byte {
	if !t.present {
		return nil
	}
	return append([]byte{}, t.data...)
}

// IsSet - zero Token과 SDK가 반환한 token을 구분한다.
func (t Token) IsSet() bool {
	return t.present
}

// Len - token의 byte 길이를 반환한다.
func (t Token) Len() int {
	return len(t.data)
}

// String는 token을 숨긴다.
func (t Token) String() string {
	return redactedToken
}

// GoString - %#v 포맷에서도 token을 숨긴다.
func (t Token) GoString() string {
	return redactedToken
}
