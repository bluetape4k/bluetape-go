package leader

import "context"

// Elector 는 한 group 안의 한 member leadership을 조정한다.
type Elector interface {
	// Campaign 은 leadership 획득을 시도한다.
	//
	// 이미 이 elector가 leader이면 [ErrAlreadyLeader]를 반환한다. 다른 member가
	// leader이면 [ErrNotLeader]를 반환한다. backend 오류와 context 취소 오류는
	// errors.Is로 원인을 확인할 수 있게 감싸서 반환할 수 있다.
	Campaign(ctx context.Context) error

	// Resign 은 현재 보유한 leadership을 해제한다.
	//
	// 이미 leader가 아니면 성공으로 처리한다. 따라서 반복 호출해도 안전하다.
	Resign(ctx context.Context) error

	// IsLeader 는 이 elector가 아직 leader라고 판단하는지 알려준다.
	IsLeader() bool

	// Leader 는 backend가 기록한 현재 leader token을 반환한다.
	//
	// leader가 없으면 빈 문자열과 nil error를 반환한다.
	Leader(ctx context.Context) (string, error)
}
