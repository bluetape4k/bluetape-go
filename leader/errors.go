package leader

import "errors"

var (
	// ErrAlreadyLeader 는 elector가 이미 leader일 때 반환된다.
	ErrAlreadyLeader = errors.New("leader: already leader")

	// ErrNotLeader 는 다른 member가 leader일 때 반환된다.
	ErrNotLeader = errors.New("leader: not leader")
)
