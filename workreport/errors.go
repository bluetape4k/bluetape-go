package workreport

import (
	"errors"
	"fmt"
)

// ErrUnknownFailurePolicy is returned when aggregation receives an unsupported
// failure policy value.
var ErrUnknownFailurePolicy = errors.New("unknown failure policy")

// FailurePolicyError describes an invalid failure policy.
type FailurePolicyError struct {
	Policy FailurePolicy
}

func (e FailurePolicyError) Error() string {
	return fmt.Sprintf("%v: %d", ErrUnknownFailurePolicy, e.Policy)
}

// Is allows errors.Is checks against ErrUnknownFailurePolicy.
func (e FailurePolicyError) Is(target error) bool {
	return target == ErrUnknownFailurePolicy
}
