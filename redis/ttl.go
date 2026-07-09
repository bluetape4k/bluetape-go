package btredis

import (
	"fmt"
	"time"
)

// ValidateTTL verifies that ttl is positive and at least one millisecond.
func ValidateTTL(name string, ttl time.Duration) error {
	if ttl < time.Millisecond {
		return fmt.Errorf("%w: invalid %s duration", ErrInvalidTTL, ttlName(name))
	}
	return nil
}

// TTLMillis returns ttl converted to Redis millisecond precision.
func TTLMillis(name string, ttl time.Duration) (int64, error) {
	if err := ValidateTTL(name, ttl); err != nil {
		return 0, err
	}
	return ttl.Milliseconds(), nil
}

func ttlName(name string) string {
	if validLabel(name) {
		return name
	}
	return "ttl"
}
