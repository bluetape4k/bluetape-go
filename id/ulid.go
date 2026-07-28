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
type ULIDOption func(*ulidGenerator) error

// WithULIDEntropy ULIDEntropy 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - entropy: WithULIDEntropy에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithULIDEntropy(entropy io.Reader) ULIDOption {
	return func(g *ulidGenerator) error {
		if entropy == nil {
			return OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
		}
		g.entropy = entropy
		return nil
	}
}

// WithULIDTime ULIDTime 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - now: WithULIDTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithULIDTime(now func() time.Time) ULIDOption {
	return func(g *ulidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewULIDGenerator ULIDGenerator 인스턴스를 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewULIDGenerator(options ...ULIDOption) (StringGenerator, error) {
	return newULIDGenerator(defaultEntropyReader(), time.Now, options...)
}

// NewMonotonicULIDGenerator MonotonicULIDGenerator 인스턴스를 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// NewULID ULID 인스턴스를 생성한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewULID() (string, error) {
	g, err := NewULIDGenerator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseULID 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - value: ParseULID가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseULID(value string) (string, error) {
	parsed, err := okulid.ParseStrict(value)
	if err != nil {
		return "", ParseError{Kind: "ulid", Value: value, Err: err}
	}
	return parsed.String(), nil
}

// ULIDTime ULID에 포함된 시각을 반환한다.
//
// 매개변수:
//   - value: ULIDTime가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ULIDTime(value string) (time.Time, error) {
	parsed, err := okulid.ParseStrict(value)
	if err != nil {
		return time.Time{}, ParseError{Kind: "ulid", Value: value, Err: err}
	}
	return time.UnixMilli(int64(parsed.Time())).UTC(), nil
}
