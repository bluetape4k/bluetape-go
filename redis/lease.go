package btredis

import (
	"fmt"
	"log/slog"
	"strings"
)

// Lease is an immutable Redis key and owner-token pair.
type Lease struct {
	key   string
	token OwnerToken
}

// NewLease returns a lease after validating the key and owner token.
func NewLease(key string, token OwnerToken) (Lease, error) {
	lease := Lease{key: key, token: token}
	if err := lease.Validate(); err != nil {
		return Lease{}, err
	}
	return lease, nil
}

// Key returns the exact Redis key.
func (l Lease) Key() string {
	return l.key
}

// RedactedKeyID returns the lease key correlation identifier.
func (l Lease) RedactedKeyID() string {
	return RedactedKeyID(l.key)
}

// Token returns the owner token.
func (l Lease) Token() OwnerToken {
	return l.token
}

// String returns a redacted display value.
func (l Lease) String() string {
	return "redis-lease:" + l.RedactedKeyID()
}

// GoString returns a redacted debug display value.
func (l Lease) GoString() string {
	return l.String()
}

// LogValue returns a redacted structured logging value.
func (l Lease) LogValue() slog.Value {
	return slog.StringValue(l.String())
}

// Validate verifies that the lease has a non-blank key and valid token.
func (l Lease) Validate() error {
	if strings.TrimSpace(l.key) == "" {
		return fmt.Errorf("%w: invalid lease key", ErrInvalidKey)
	}
	if err := l.token.Validate(); err != nil {
		return err
	}
	return nil
}
