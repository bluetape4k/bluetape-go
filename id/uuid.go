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
type UUIDOption func(*uuidGenerator) error

// WithUUIDReader UUIDReader 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - reader: WithUUIDReader에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithUUIDReader(reader io.Reader) UUIDOption {
	return func(g *uuidGenerator) error {
		if reader == nil {
			return OptionError{Option: "reader", Err: errorsNew("must not be nil")}
		}
		g.reader = reader
		return nil
	}
}

// WithUUIDTime UUIDTime 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - now: WithUUIDTime에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithUUIDTime(now func() time.Time) UUIDOption {
	return func(g *uuidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewUUIDV4Generator UUIDV4Generator 인스턴스를 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewUUIDV4Generator(options ...UUIDOption) (StringGenerator, error) {
	return newUUIDGenerator(uuidVersion4, options...)
}

// NewUUIDV7Generator UUIDV7Generator 인스턴스를 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// NewUUIDV4 UUIDV4 인스턴스를 생성한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewUUIDV4() (string, error) {
	g, err := NewUUIDV4Generator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// NewUUIDV7 UUIDV7 인스턴스를 생성한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewUUIDV7() (string, error) {
	g, err := NewUUIDV7Generator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseUUID 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - value: ParseUUID가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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
