package encrypt

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidKey AES key material이 없거나 지원하지 않는 크기일 때 반환된다.
	ErrInvalidKey = errors.New("encrypt: invalid key")
	// ErrMalformedCiphertext ciphertext envelope를 parse할 수 없을 때 반환된다.
	ErrMalformedCiphertext = errors.New("encrypt: malformed ciphertext")
	// ErrAuthenticationFailed 변조, 잘못된 key, 또는 다른 associated data로 인증이 실패했을 때 반환된다.
	ErrAuthenticationFailed = errors.New("encrypt: authentication failed")
	// ErrInvalidOptions encryptor option이 유효하지 않을 때 반환된다.
	ErrInvalidOptions = errors.New("encrypt: invalid options")
)

// Error encrypt sentinel identity와 선택적 원인을 보존하되 Error() 문자열에 plaintext,
// ciphertext, key byte, associated data를 노출하지 않는다.
type Error struct {
	Kind      error
	Operation string
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ErrInvalidOptions.Error()
	}
	kind := e.Kind
	if kind == nil {
		kind = ErrInvalidOptions
	}
	if e.Operation != "" {
		return fmt.Sprintf("%v: %s", kind, e.Operation)
	}
	return kind.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is encrypt sentinel error와 wrapping된 원인에 대한 errors.Is matching을 지원한다.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind || errors.Is(e.Kind, target) || errors.Is(e.Cause, target)
}

func errorWith(kind error, operation string, cause error) *Error {
	return &Error{Kind: kind, Operation: operation, Cause: cause}
}
