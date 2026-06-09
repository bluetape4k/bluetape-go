package id

import (
	"crypto/rand"
	"io"
	"time"

	okulid "github.com/oklog/ulid/v2"
)

type ulidGenerator struct {
	entropy io.Reader
	now     func() time.Time
}

// ULIDOption configures ULID string generation.
type ULIDOption func(*ulidGenerator) error

// WithULIDEntropy injects an entropy reader. Production defaults use crypto/rand.
func WithULIDEntropy(entropy io.Reader) ULIDOption {
	return func(g *ulidGenerator) error {
		if entropy == nil {
			return OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
		}
		g.entropy = entropy
		return nil
	}
}

// WithULIDTime injects a clock for deterministic tests.
func WithULIDTime(now func() time.Time) ULIDOption {
	return func(g *ulidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewULIDGenerator creates a random ULID string generator.
func NewULIDGenerator(options ...ULIDOption) (StringGenerator, error) {
	return newULIDGenerator(rand.Reader, time.Now, options...)
}

// NewMonotonicULIDGenerator creates a concurrency-safe monotonic ULID generator.
func NewMonotonicULIDGenerator(options ...ULIDOption) (StringGenerator, error) {
	g, err := newULIDGenerator(rand.Reader, time.Now, options...)
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
	return value.String(), nil
}

// NewULID returns a random canonical ULID string.
func NewULID() (string, error) {
	g, err := NewULIDGenerator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseULID canonicalizes a ULID string using strict Crockford Base32 parsing.
func ParseULID(value string) (string, error) {
	parsed, err := okulid.ParseStrict(value)
	if err != nil {
		return "", ParseError{Kind: "ulid", Value: value, Err: err}
	}
	return parsed.String(), nil
}

// ULIDTime extracts the timestamp encoded in a canonical ULID string.
func ULIDTime(value string) (time.Time, error) {
	parsed, err := okulid.ParseStrict(value)
	if err != nil {
		return time.Time{}, ParseError{Kind: "ulid", Value: value, Err: err}
	}
	return time.UnixMilli(int64(parsed.Time())).UTC(), nil
}
