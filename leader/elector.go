package leader

import "context"

// Elector 한 group 안의 한 member leadership을 조정한다.
type Elector interface {
	// Campaign 은 leadership 획득을 시도한다.
	//
	// 다른 member가 leader이면 leadership을 얻거나 ctx가 끝날 때까지 대기한다.
	// 이미 이 elector가 leader이면 [ErrAlreadyLeader], 같은 elector의 campaign이
	// 진행 중이면 [ErrCampaignInProgress], 이전 정리가 남아 있으면
	// [ErrCleanupPending]을 반환한다. [ErrCommitUnknown]이면 같은 elector에서 bounded
	// [Elector.Resign]을 재시도한 뒤 provider별 cleanup proof/expiry contract를 따라야
	// 한다. etcd provider는 성공한 revoke 또는 linearizable exact-key reconciliation이
	// 필요하며 TTL 경과만으로 cleanup을 증명하지 않는다.
	Campaign(ctx context.Context) error

	// Resign 은 현재 보유한 leadership을 해제한다.
	//
	// 이미 leader가 아니면 성공으로 처리한다. 따라서 반복 호출해도 안전하다.
	Resign(ctx context.Context) error

	// IsLeader 이 elector가 아직 leader라고 판단하는지 알려준다.
	IsLeader() bool

	// Leader backend가 기록한 현재 leader token을 반환한다.
	//
	// leader가 없으면 빈 문자열과 nil error를 반환한다.
	Leader(ctx context.Context) (string, error)
}
