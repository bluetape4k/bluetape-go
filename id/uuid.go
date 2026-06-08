package id

import (
	"io"

	googleuuid "github.com/google/uuid"
)

type uuidVersion int

const (
	uuidVersion4 uuidVersion = 4
	uuidVersion7 uuidVersion = 7
)

type uuidGenerator struct {
	version uuidVersion
	reader  io.Reader
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

// NewUUIDV4Generator creates a UUID v4 string generator.
func NewUUIDV4Generator(options ...UUIDOption) (StringGenerator, error) {
	return newUUIDGenerator(uuidVersion4, options...)
}

// NewUUIDV7Generator creates a UUID v7 string generator.
func NewUUIDV7Generator(options ...UUIDOption) (StringGenerator, error) {
	return newUUIDGenerator(uuidVersion7, options...)
}

func newUUIDGenerator(version uuidVersion, options ...UUIDOption) (*uuidGenerator, error) {
	g := &uuidGenerator{version: version}
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
		if g.reader == nil {
			uuid, err = googleuuid.NewV7()
		} else {
			uuid, err = googleuuid.NewV7FromReader(g.reader)
		}
	default:
		return "", OptionError{Option: "version", Err: ErrUnsupportedVersion}
	}
	if err != nil {
		return "", EntropyError{Kind: "uuid", Err: err}
	}
	return uuid.String(), nil
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
