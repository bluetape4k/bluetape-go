package id

import (
	"io"
	"time"

	okulid "github.com/oklog/ulid/v2"
)

type ulidGenerator struct {
	entropy io.Reader
	now     func() time.Time
}

// ULIDOption func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ULIDOption func(*ulidGenerator) error

// WithULIDEntropy WithULIDEntropy 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - entropy: WithULIDEntropy 동작에 필요한 entropy 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithULIDEntropy(entropy io.Reader) ULIDOption {
	return func(g *ulidGenerator) error {
		if entropy == nil {
			return OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
		}
		g.entropy = entropy
		return nil
	}
}

// WithULIDTime WithULIDTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - now: WithULIDTime 동작에 필요한 now 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithULIDTime(now func() time.Time) ULIDOption {
	return func(g *ulidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewULIDGenerator NewULIDGenerator 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - options: NewULIDGenerator 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewULIDGenerator(options ...ULIDOption) (StringGenerator, error) {
	return newULIDGenerator(defaultEntropyReader(), time.Now, options...)
}

// NewMonotonicULIDGenerator NewMonotonicULIDGenerator 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - options: NewMonotonicULIDGenerator 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewMonotonicULIDGenerator(options ...ULIDOption) (StringGenerator, error) {
	g, err := newULIDGenerator(defaultEntropyReader(), time.Now, options...)
	if err != nil {
		return nil, err
	}
	g.entropy = &okulid.LockedMonotonicReader{MonotonicReader: okulid.Monotonic(g.entropy, 0)}
	return g, nil
}

func newULIDGenerator(entropy io.Reader, now func() time.Time, options ...ULIDOption) (*ulidGenerator, error) {
	g := &ulidGenerator{entropy: entropy, now: now}
	for _, option := range options {
		if option == nil {
			return nil, OptionError{Option: "option", Err: errorsNew("must not be nil")}
		}
		if err := option(g); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (g *ulidGenerator) NextString() (string, error) {
	if g == nil {
		return "", OptionError{Option: "generator", Err: errorsNew("must not be nil")}
	}
	if g.entropy == nil {
		return "", OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
	}
	if g.now == nil {
		return "", OptionError{Option: "now", Err: errorsNew("must not be nil")}
	}
	value, err := okulid.New(okulid.Timestamp(g.now()), g.entropy)
	if err != nil {
		return "", EntropyError{Kind: "ulid", Err: err}
	}
	var encoded [okulid.EncodedSize]byte
	if err := value.MarshalTextTo(encoded[:]); err != nil {
		return "", EntropyError{Kind: "ulid", Err: err}
	}
	return string(encoded[:]), nil
}

// NewULID NewULID 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewULID() (string, error) {
	g, err := NewULIDGenerator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseULID ParseULID 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ParseULID가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseULID(value string) (string, error) {
	parsed, err := okulid.ParseStrict(value)
	if err != nil {
		return "", ParseError{Kind: "ulid", Value: value, Err: err}
	}
	return parsed.String(), nil
}

// ULIDTime ULIDTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ULIDTime가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ULIDTime(value string) (time.Time, error) {
	parsed, err := okulid.ParseStrict(value)
	if err != nil {
		return time.Time{}, ParseError{Kind: "ulid", Value: value, Err: err}
	}
	return time.UnixMilli(int64(parsed.Time())).UTC(), nil
}
