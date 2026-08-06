package id

import (
	"io"
	"time"

	segmentioksuid "github.com/segmentio/ksuid"
)

const (
	ksuidEpochStamp         int64 = 1_400_000_000
	ksuidMaxTimestampOffset int64 = 1<<32 - 1
	ksuidPayloadLength            = 16
)

type ksuidGenerator struct {
	entropy io.Reader
	now     func() time.Time
}

// KSUIDOption func 공개 타입이다.
type KSUIDOption func(*ksuidGenerator) error

// WithKSUIDEntropy KSUIDEntropy 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - entropy: WithKSUIDEntropy에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithKSUIDEntropy(entropy io.Reader) KSUIDOption {
	return func(g *ksuidGenerator) error {
		if entropy == nil {
			return OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
		}
		g.entropy = entropy
		return nil
	}
}

// WithKSUIDTime KSUIDTime 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - now: WithKSUIDTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithKSUIDTime(now func() time.Time) KSUIDOption {
	return func(g *ksuidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewKSUIDGenerator KSUIDGenerator 인스턴스를 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewKSUIDGenerator(options ...KSUIDOption) (StringGenerator, error) {
	return newKSUIDGenerator(defaultEntropyReader(), time.Now, options...)
}

func newKSUIDGenerator(entropy io.Reader, now func() time.Time, options ...KSUIDOption) (*ksuidGenerator, error) {
	g := &ksuidGenerator{entropy: entropy, now: now}
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

func (g *ksuidGenerator) NextString() (string, error) {
	if g == nil {
		return "", OptionError{Option: "generator", Err: errorsNew("must not be nil")}
	}
	if g.entropy == nil {
		return "", OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
	}
	if g.now == nil {
		return "", OptionError{Option: "now", Err: errorsNew("must not be nil")}
	}

	now := g.now()
	if err := validateKSUIDTime(now); err != nil {
		return "", err
	}

	payload := make([]byte, ksuidPayloadLength)
	if _, err := io.ReadFull(g.entropy, payload); err != nil {
		return "", EntropyError{Kind: "ksuid", Err: err}
	}
	value, err := segmentioksuid.FromParts(now, payload)
	if err != nil {
		return "", EntropyError{Kind: "ksuid", Err: err}
	}
	return value.String(), nil
}

func validateKSUIDTime(value time.Time) error {
	offset := value.Unix() - ksuidEpochStamp
	if offset < 0 {
		return OptionError{Option: "time", Err: errorsNew("before KSUID epoch")}
	}
	if offset > ksuidMaxTimestampOffset {
		return OptionError{Option: "time", Err: errorsNew("outside KSUID timestamp range")}
	}
	return nil
}

// NewKSUID KSUID 인스턴스를 생성한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewKSUID() (string, error) {
	g, err := NewKSUIDGenerator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseKSUID 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - value: ParseKSUID가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ParseKSUID(value string) (string, error) {
	parsed, err := segmentioksuid.Parse(value)
	if err != nil {
		return "", ParseError{Kind: "ksuid", Value: value, Err: err}
	}
	canonical := parsed.String()
	if value != canonical {
		return "", ParseError{Kind: "ksuid", Value: value, Err: errorsNew("must be canonical KSUID string")}
	}
	return canonical, nil
}

// KSUIDTime KSUID에 포함된 시각을 반환한다.
//
// 매개변수:
//   - value: KSUIDTime가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func KSUIDTime(value string) (time.Time, error) {
	parsed, err := segmentioksuid.Parse(value)
	if err != nil {
		return time.Time{}, ParseError{Kind: "ksuid", Value: value, Err: err}
	}
	if value != parsed.String() {
		return time.Time{}, ParseError{Kind: "ksuid", Value: value, Err: errorsNew("must be canonical KSUID string")}
	}
	return parsed.Time().UTC(), nil
}
