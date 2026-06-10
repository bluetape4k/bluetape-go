package id

import (
	"crypto/rand"
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

// UUIDOption configures UUID string generation.
type UUIDOption func(*uuidGenerator) error

// WithUUIDReader injects an entropy reader for deterministic tests.
func WithUUIDReader(reader io.Reader) UUIDOption {
	return func(g *uuidGenerator) error {
		if reader == nil {
			return OptionError{Option: "reader", Err: errorsNew("must not be nil")}
		}
		g.reader = reader
		return nil
	}
}

// WithUUIDTime injects a clock for UUID v7 deterministic tests.
//
// It is ignored by UUID v4 generators because UUID v4 has no timestamp field.
// Custom clocks must be safe for concurrent use when a generator is shared.
func WithUUIDTime(now func() time.Time) UUIDOption {
	return func(g *uuidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewUUIDV4Generator creates a UUID v4 string generator.
func NewUUIDV4Generator(options ...UUIDOption) (StringGenerator, error) {
	return newUUIDGenerator(uuidVersion4, options...)
}

// NewUUIDV7Generator creates a UUID v7 string generator.
func NewUUIDV7Generator(options ...UUIDOption) (StringGenerator, error) {
	return newUUIDGenerator(uuidVersion7, options...)
}

func newUUIDGenerator(version uuidVersion, options ...UUIDOption) (*uuidGenerator, error) {
	g := &uuidGenerator{
		version:    version,
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
		if g.reader == nil {
			uuid, err = googleuuid.NewRandom()
		} else {
			uuid, err = googleuuid.NewRandomFromReader(g.reader)
		}
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
		reader = rand.Reader
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

// NewUUIDV4 returns a UUID v4 canonical string.
func NewUUIDV4() (string, error) {
	g, err := NewUUIDV4Generator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// NewUUIDV7 returns a UUID v7 canonical string.
func NewUUIDV7() (string, error) {
	g, err := NewUUIDV7Generator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseUUID validates and canonicalizes a UUID string.
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
