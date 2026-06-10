package id

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

const (
	ksuidMillisEpoch        int64 = 1_400_000_000_000
	ksuidMillisTimestampLen       = 8
	ksuidMillisPayloadLen         = 12
	ksuidMillisTotalBytes         = ksuidMillisTimestampLen + ksuidMillisPayloadLen
	ksuidMillisEncodedLen         = 27
)

type ksuidMillisGenerator struct {
	entropy io.Reader
	now     func() time.Time
}

// KSUIDMillisOption configures Kotlin-compatible millisecond KSUID generation.
type KSUIDMillisOption func(*ksuidMillisGenerator) error

// WithKSUIDMillisEntropy injects an entropy reader. Production defaults use crypto/rand.
// Custom readers must be safe for concurrent use when the generator is shared.
func WithKSUIDMillisEntropy(entropy io.Reader) KSUIDMillisOption {
	return func(g *ksuidMillisGenerator) error {
		if entropy == nil {
			return OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
		}
		g.entropy = entropy
		return nil
	}
}

// WithKSUIDMillisTime injects a clock for deterministic tests. Custom clocks
// must be safe for concurrent use when the generator is shared.
func WithKSUIDMillisTime(now func() time.Time) KSUIDMillisOption {
	return func(g *ksuidMillisGenerator) error {
		if now == nil {
			return OptionError{Option: "now", Err: errorsNew("must not be nil")}
		}
		g.now = now
		return nil
	}
}

// NewKSUIDMillisGenerator creates a Kotlin-compatible millisecond KSUID string generator.
func NewKSUIDMillisGenerator(options ...KSUIDMillisOption) (StringGenerator, error) {
	return newKSUIDMillisGenerator(defaultEntropyReader(), time.Now, options...)
}

func newKSUIDMillisGenerator(entropy io.Reader, now func() time.Time, options ...KSUIDMillisOption) (*ksuidMillisGenerator, error) {
	g := &ksuidMillisGenerator{entropy: entropy, now: now}
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

func (g *ksuidMillisGenerator) NextString() (string, error) {
	if g == nil {
		return "", OptionError{Option: "generator", Err: errorsNew("must not be nil")}
	}
	if g.entropy == nil {
		return "", OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
	}
	if g.now == nil {
		return "", OptionError{Option: "now", Err: errorsNew("must not be nil")}
	}

	var raw [ksuidMillisTotalBytes]byte
	binary.BigEndian.PutUint64(raw[:ksuidMillisTimestampLen], uint64(g.now().UnixMilli()-ksuidMillisEpoch))
	if _, err := io.ReadFull(g.entropy, raw[ksuidMillisTimestampLen:]); err != nil {
		return "", EntropyError{Kind: "ksuid-millis", Err: err}
	}
	var buffer [ksuidMillisEncodedLen]byte
	encoded := encodeKSUIDMillisBase62Prefix(buffer[:0], raw[:], ksuidMillisEncodedLen)
	return encoded, nil
}

// NewKSUIDMillis returns a Kotlin-compatible millisecond KSUID string.
func NewKSUIDMillis() (string, error) {
	g, err := NewKSUIDMillisGenerator()
	if err != nil {
		return "", err
	}
	return g.NextString()
}

// ParseKSUIDMillis validates a Kotlin-compatible millisecond KSUID string.
//
// Bare 27-character KSUID strings are not self-describing. This validates the
// Kotlin-compatible millis shape only; callers must know they are handling the
// millis family, not the Segment-compatible seconds family.
func ParseKSUIDMillis(value string) (string, error) {
	if _, err := decodeKSUIDMillis(value); err != nil {
		return "", ParseError{Kind: "ksuid-millis", Value: value, Err: err}
	}
	return value, nil
}

// KSUIDMillisTime extracts the timestamp encoded in a Kotlin-compatible millisecond KSUID string.
//
// Bare 27-character KSUID strings are not self-describing. Call this only for
// caller-known millis strings; Segment seconds strings may parse but produce the
// wrong family interpretation.
func KSUIDMillisTime(value string) (time.Time, error) {
	raw, err := decodeKSUIDMillis(value)
	if err != nil {
		return time.Time{}, ParseError{Kind: "ksuid-millis", Value: value, Err: err}
	}
	offset := int64(binary.BigEndian.Uint64(raw[:ksuidMillisTimestampLen]))
	return time.UnixMilli(offset + ksuidMillisEpoch).UTC(), nil
}

func decodeKSUIDMillis(value string) ([]byte, error) {
	if len(value) != ksuidMillisEncodedLen {
		return nil, fmt.Errorf("valid KSUID millis strings are %d characters", ksuidMillisEncodedLen)
	}
	return decodeKSUIDMillisBase62(value, ksuidMillisTotalBytes)
}

const (
	ksuidMillisCompactMask = 0x1e
	ksuidMillisMask5Bits   = 0x1f
)

var ksuidMillisEncodeTable = [...]byte{
	'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
	'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
	'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
}

func encodeKSUIDMillisBase62(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	output := encodeKSUIDMillisBase62Prefix(make([]byte, 0, len(data)*8/5+1), data, 0)
	return output
}

func encodeKSUIDMillisBase62Prefix(dst []byte, data []byte, limit int) string {
	input := bitInput{bytes: data, bitLength: len(data) * 8}
	for input.hasMore() && (limit == 0 || len(dst) < limit) {
		rawBits := input.readBits(6)
		bits := rawBits
		if rawBits&ksuidMillisCompactMask == ksuidMillisCompactMask {
			bits = rawBits & ksuidMillisMask5Bits
			input.seekBit(-1)
		}
		dst = append(dst, ksuidMillisEncodeTable[bits])
	}
	return string(dst)
}

func decodeKSUIDMillisBase62(value string, expectedBytes int) ([]byte, error) {
	if value == "" {
		return []byte{}, nil
	}
	if expectedBytes < 0 {
		return nil, OptionError{Option: "expectedBytes", Err: errorsNew("must not be negative")}
	}

	output := newBitOutput(len(value) * 6)
	for index := range value {
		bits, err := ksuidMillisDecodedBits(value[index])
		if err != nil {
			return nil, err
		}
		bitsCount := 6
		switch {
		case bits&ksuidMillisCompactMask == ksuidMillisCompactMask:
			bitsCount = 5
		case index == len(value)-1:
			bitsCount = output.bitsCountUpToByte()
		}
		output.writeBits(bitsCount, bits)
	}
	decoded := output.toArray()
	if len(decoded) == expectedBytes {
		return decoded, nil
	}
	result := make([]byte, expectedBytes)
	copy(result, decoded)
	return result, nil
}

func ksuidMillisDecodedBits(char byte) (int, error) {
	switch {
	case char >= 'A' && char <= 'Z':
		return int(char - 'A'), nil
	case char >= 'a' && char <= 'z':
		return int(char-'a') + 26, nil
	case char >= '0' && char <= '9':
		return int(char-'0') + 52, nil
	default:
		return 0, fmt.Errorf("invalid KSUID millis Base62 character %q", char)
	}
}

type bitInput struct {
	bytes     []byte
	bitLength int
	offset    int
}

func (i *bitInput) hasMore() bool {
	return i.offset < i.bitLength
}

func (i *bitInput) seekBit(pos int) {
	i.offset += pos
}

func (i *bitInput) readBits(bitsCount int) int {
	bitNum := i.offset % 8
	byteNum := i.offset / 8

	firstRead := min(8-bitNum, bitsCount)
	secondRead := bitsCount - firstRead

	result := (int(i.bytes[byteNum]) & (((1 << firstRead) - 1) << bitNum)) >> bitNum
	if secondRead > 0 && len(i.bytes) > byteNum+1 {
		result |= (int(i.bytes[byteNum+1]) & ((1 << secondRead) - 1)) << firstRead
	}
	i.offset += bitsCount
	return result
}

type bitOutput struct {
	bytes  []byte
	offset int
}

func newBitOutput(capacity int) *bitOutput {
	return &bitOutput{bytes: make([]byte, (capacity+7)/8)}
}

func (o *bitOutput) currentBit() int {
	return o.offset % 8
}

func (o *bitOutput) currentLength() int {
	return o.offset / 8
}

func (o *bitOutput) bitsCountUpToByte() int {
	if bit := o.currentBit(); bit != 0 {
		return 8 - bit
	}
	return 0
}

func (o *bitOutput) writeBits(bitsCount int, bits int) {
	if bitsCount == 0 {
		return
	}
	bitNum := o.currentBit()
	byteNum := o.currentLength()

	firstWrite := min(8-bitNum, bitsCount)
	secondWrite := bitsCount - firstWrite

	o.bytes[byteNum] |= byte((bits & ((1 << firstWrite) - 1)) << bitNum)
	if secondWrite > 0 {
		o.bytes[byteNum+1] |= byte((bits >> firstWrite) & ((1 << secondWrite) - 1))
	}
	o.offset += bitsCount
}

func (o *bitOutput) toArray() []byte {
	size := o.currentLength()
	if o.currentBit() != 0 {
		size++
	}
	result := make([]byte, size)
	copy(result, o.bytes[:size])
	return result
}
