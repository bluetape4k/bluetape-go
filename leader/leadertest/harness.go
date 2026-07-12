package leadertest

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/leader"
)

// Operation identifies a leader mutation boundary observed by a test Control.
type Operation string

const (
	// OperationCampaign identifies acquisition attempts.
	OperationCampaign Operation = "campaign"
	// OperationRenew identifies lease renewal attempts.
	OperationRenew Operation = "renew"
	// OperationResign identifies compare-and-delete attempts.
	OperationResign Operation = "resign"
)

// Factory constructs one elector attached to the harness backend namespace.
type Factory func(testing.TB, leader.Options) (leader.Elector, error)

// Control exposes deterministic test-only backend observation and fault injection.
type Control interface {
	ReplaceOwner(context.Context, leader.Options, string) error
	FailNext(context.Context, leader.Options, Operation, error) error
	Owner(context.Context, leader.Options) (string, error)
	OperationCount(leader.Options, Operation) int64
}

// Harness supplies a mandatory provider factory and backend control.
type Harness struct {
	New     Factory
	Control Control
}

func validateHarness(h Harness) error {
	if h.New == nil {
		return errors.New("leadertest: nil factory")
	}
	if h.Control == nil {
		return errors.New("leadertest: nil control")
	}
	return nil
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func validOperation(operation Operation) bool {
	switch operation {
	case OperationCampaign, OperationRenew, OperationResign:
		return true
	default:
		return false
	}
}
