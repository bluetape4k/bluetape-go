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

// KSUIDOption는 func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type KSUIDOption func(*ksuidGenerator) error

// WithKSUIDEntropy는 WithKSUIDEntropy 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - entropy: WithKSUIDEntropy 동작에 필요한 entropy 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithKSUIDEntropy(entropy io.Reader) KSUIDOption {
	return func(g *ksuidGenerator) error {
		if entropy == nil {
			return OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
		}
		g.entropy = entropy
		return nil
	}
}

// WithKSUIDTime는 WithKSUIDTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - now: WithKSUIDTime 동작에 필요한 now 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithKSUIDTime(now func() time.Time) KSUIDOption {
	return func(g *ksuidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewKSUIDGenerator는 NewKSUIDGenerator 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - options: NewKSUIDGenerator 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// NewKSUID는 NewKSUID 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewKSUID() (string, error) {
	g, err := NewKSUIDGenerator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseKSUID는 ParseKSUID 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ParseKSUID가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// KSUIDTime는 KSUIDTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: KSUIDTime가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
