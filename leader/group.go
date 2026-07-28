package leader

import (
	"context"

	"github.com/bluetape4k/bluetape-go/core"
)

const defaultGroupKeyPrefix = "bluetape:leader-group"

// GroupOptions multi-leader election 참가자를 설정한다.
type GroupOptions struct {
	Options
	MaxLeaders int
}

// Normalize group 옵션을 검증하고 기본값을 채운다.
func (o GroupOptions) Normalize() (GroupOptions, error) {
	if o.KeyPrefix == "" {
		o.KeyPrefix = defaultGroupKeyPrefix
	}

	normalized, err := o.Options.Normalize()
	if err != nil {
		return GroupOptions{}, err
	}
	if err := core.RequirePositive("maxLeaders", o.MaxLeaders); err != nil {
		return GroupOptions{}, err
	}

	return GroupOptions{
		Options:    normalized,
		MaxLeaders: o.MaxLeaders,
	}, nil
}

// GroupElector 한 group 안에서 제한된 수의 leader slot을 조정한다.
type GroupElector interface {
	// Campaign 은 빈 leader slot을 획득할 때까지 대기한다.
	Campaign(ctx context.Context) error

	// Resign 은 현재 보유한 leader slot을 해제한다.
	Resign(ctx context.Context) error

	// IsLeader 이 elector가 아직 slot을 보유한다고 판단하는지 알려준다.
	IsLeader() bool

	// ActiveCount 현재 살아 있는 leader slot 수를 반환한다.
	ActiveCount(ctx context.Context) (int, error)

	// AvailableSlots 추가로 획득할 수 있는 leader slot 수를 반환한다.
	AvailableSlots(ctx context.Context) (int, error)
}
