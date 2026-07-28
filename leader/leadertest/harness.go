package leadertest

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/leader"
)

// Operation은 test Control이 관찰하는 leader mutation boundary를 식별한다.
type Operation string

const (
	// OperationCampaign은 acquisition attempt를 식별한다.
	OperationCampaign Operation = "campaign"
	// OperationRenew lease renewal attempt를 식별한다.
	OperationRenew Operation = "renew"
	// OperationResign은 compare-and-delete attempt를 식별한다.
	OperationResign Operation = "resign"
)

// Factory harness backend namespace에 연결된 elector 하나를 생성한다.
type Factory func(testing.TB, leader.Options) (leader.Elector, error)

// Control은 deterministic test-only backend observation과 fault injection을 노출한다.
type Control interface {
	ReplaceOwner(context.Context, leader.Options, string) error
	FailNext(context.Context, leader.Options, Operation, error) error
	Owner(context.Context, leader.Options) (string, error)
	OperationCount(leader.Options, Operation) int64
}

// Harness 필수 provider factory와 backend control을 제공한다.
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
