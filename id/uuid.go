package id

import (
	"errors"
	"io"
	"sync"
	"time"

	googleuuid "github.com/google/uuid"
)

type uuidVersion int

const (
	uuidVersion4       uuidVersion = 4
	uuidVersion7       uuidVersion = 7
	uuidV7MaxUnixMilli             = int64(1<<48 - 1)
)

type uuidGenerator struct {
	version uuidVersion
	reader  io.Reader
	now     func() time.Time

	mu         sync.Mutex
	lastV7Tick int64
}

// UUIDOption func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type UUIDOption func(*uuidGenerator) error

// WithUUIDReader WithUUIDReader 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - reader: WithUUIDReader 동작에 필요한 reader 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithUUIDReader(reader io.Reader) UUIDOption {
	return func(g *uuidGenerator) error {
		if reader == nil {
			return OptionError{Option: "reader", Err: errorsNew("must not be nil")}
		}
		g.reader = reader
		return nil
	}
}

// WithUUIDTime WithUUIDTime 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - now: WithUUIDTime 동작에 필요한 now 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithUUIDTime(now func() time.Time) UUIDOption {
	return func(g *uuidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewUUIDV4Generator NewUUIDV4Generator 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - options: NewUUIDV4Generator 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewUUIDV4Generator(options ...UUIDOption) (StringGenerator, error) {
	return newUUIDGenerator(uuidVersion4, options...)
}

// NewUUIDV7Generator NewUUIDV7Generator 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - options: NewUUIDV7Generator 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewUUIDV7Generator(options ...UUIDOption) (StringGenerator, error) {
	return newUUIDGenerator(uuidVersion7, options...)
}

func newUUIDGenerator(version uuidVersion, options ...UUIDOption) (*uuidGenerator, error) {
	g := &uuidGenerator{
		version:    version,
		reader:     defaultEntropyReader(),
		now:        time.Now,
		lastV7Tick: -1,
	}
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

func (g *uuidGenerator) NextString() (string, error) {
	if g == nil {
		return "", OptionError{Option: "generator", Err: errorsNew("must not be nil")}
	}
	var (
		uuid googleuuid.UUID
		err  error
	)
	switch g.version {
	case uuidVersion4:
		uuid, err = googleuuid.NewRandomFromReader(g.reader)
	case uuidVersion7:
		uuid, err = g.nextV7()
	default:
		return "", OptionError{Option: "version", Err: ErrUnsupportedVersion}
	}
	if err != nil {
		if errors.Is(err, ErrInvalidOptions) {
			return "", err
		}
		return "", EntropyError{Kind: "uuid", Err: err}
	}
	return uuid.String(), nil
}

func (g *uuidGenerator) nextV7() (googleuuid.UUID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	tick, err := uuidV7Tick(g.now())
	if err != nil {
		return googleuuid.Nil, err
	}
	if tick <= g.lastV7Tick {
		tick = g.lastV7Tick + 1
	}
	if tick > (uuidV7MaxUnixMilli<<12)|0x0fff {
		return googleuuid.Nil, OptionError{Option: "time", Err: errorsNew("outside UUID v7 timestamp range")}
	}

	var uuid googleuuid.UUID
	reader := g.reader
	if reader == nil {
		reader = defaultEntropyReader()
	}
	if _, err := io.ReadFull(reader, uuid[:]); err != nil {
		return googleuuid.Nil, err
	}
	g.lastV7Tick = tick

	milli := tick >> 12
	seq := tick & 0x0fff

	uuid[0] = byte(milli >> 40)
	uuid[1] = byte(milli >> 32)
	uuid[2] = byte(milli >> 24)
	uuid[3] = byte(milli >> 16)
	uuid[4] = byte(milli >> 8)
	uuid[5] = byte(milli)
	uuid[6] = 0x70 | byte(seq>>8)
	uuid[7] = byte(seq)
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return uuid, nil
}

func uuidV7Tick(now time.Time) (int64, error) {
	milli := now.UnixMilli()
	if milli < 0 {
		return 0, OptionError{Option: "time", Err: errorsNew("before Unix epoch")}
	}
	if milli > uuidV7MaxUnixMilli {
		return 0, OptionError{Option: "time", Err: errorsNew("outside UUID v7 timestamp range")}
	}
	fraction := int64(now.Nanosecond()%int(time.Millisecond)) >> 8
	return (milli << 12) | (fraction & 0x0fff), nil
}

// NewUUIDV4 NewUUIDV4 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewUUIDV4() (string, error) {
	g, err := NewUUIDV4Generator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// NewUUIDV7 NewUUIDV7 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewUUIDV7() (string, error) {
	g, err := NewUUIDV7Generator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseUUID ParseUUID 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: ParseUUID가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func ParseUUID(value string) (string, error) {
	parsed, err := googleuuid.Parse(value)
	if err != nil {
		return "", ParseError{Kind: "uuid", Value: value, Err: err}
	}
	canonical := parsed.String()
	if value != canonical {
		return "", ParseError{Kind: "uuid", Value: value, Err: errorsNew("must be canonical UUID string")}
	}
	return canonical, nil
}

func errorsNew(message string) error {
	return simpleError(message)
}

type simpleError string

func (e simpleError) Error() string { return string(e) }
