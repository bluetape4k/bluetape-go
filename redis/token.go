package btredis

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

var tokenPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ErrInvalidOwnerToken is returned when an owner token is empty or non-canonical.
var ErrInvalidOwnerToken = errors.New("redis: invalid owner token")

// OwnerToken is an opaque Redis lease credential.
type OwnerToken struct {
	value string
}

// NewOwnerToken returns a 256-bit random token encoded as lowercase hex.
func NewOwnerToken() (OwnerToken, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return OwnerToken{}, fmt.Errorf("redis owner token: %w", err)
	}
	return OwnerToken{value: hex.EncodeToString(data[:])}, nil
}

// ParseOwnerToken parses a canonical lowercase 256-bit hex owner token.
func ParseOwnerToken(value string) (OwnerToken, error) {
	token := OwnerToken{value: value}
	if err := token.Validate(); err != nil {
		return OwnerToken{}, err
	}
	return token, nil
}

// String returns a redacted display value.
func (t OwnerToken) String() string {
	if t.value == "" {
		return "redis-owner-token:<empty>"
	}
	return "redis-owner-token:<redacted>"
}

// GoString returns a redacted debug display value.
func (t OwnerToken) GoString() string {
	return t.String()
}

// LogValue returns a redacted structured logging value.
func (t OwnerToken) LogValue() slog.Value {
	return slog.StringValue(t.String())
}

// RedisValue returns the sensitive Redis comparison value.
func (t OwnerToken) RedisValue() string {
	return t.value
}

// Validate verifies that the token is canonical and non-empty.
func (t OwnerToken) Validate() error {
	if strings.TrimSpace(t.value) == "" || !tokenPattern.MatchString(t.value) {
		return fmt.Errorf("%w: expected 64 lowercase hex characters", ErrInvalidOwnerToken)
	}
	return nil
}
