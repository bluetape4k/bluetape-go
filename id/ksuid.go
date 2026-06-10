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

// KSUIDOption configures standard seconds-precision KSUID string generation.
type KSUIDOption func(*ksuidGenerator) error

// WithKSUIDEntropy injects an entropy reader. Production defaults use crypto/rand.
// Custom readers must be safe for concurrent use when the generator is shared.
func WithKSUIDEntropy(entropy io.Reader) KSUIDOption {
	return func(g *ksuidGenerator) error {
		if entropy == nil {
			return OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
		}
		g.entropy = entropy
		return nil
	}
}

// WithKSUIDTime injects a clock for deterministic tests. Custom clocks must be
// safe for concurrent use when the generator is shared.
func WithKSUIDTime(now func() time.Time) KSUIDOption {
	return func(g *ksuidGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewKSUIDGenerator creates a standard seconds-precision KSUID string generator.
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

// NewKSUID returns a standard seconds-precision KSUID canonical string.
func NewKSUID() (string, error) {
	g, err := NewKSUIDGenerator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseKSUID validates and canonicalizes a standard seconds-precision KSUID string.
//
// Bare 27-character KSUID strings are not self-describing. This validates the
// Segment-compatible shape only; callers must know they are handling the
// seconds family, not the Kotlin-compatible millisecond family.
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

// KSUIDTime extracts the timestamp encoded in a standard seconds-precision KSUID string.
//
// Bare 27-character KSUID strings are not self-describing. Call this only for
// caller-known Segment seconds strings; millis strings may parse but produce the
// wrong family interpretation.
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
