package leader

import "errors"

var (
	// ErrAlreadyLeader is returned when an elector already owns leadership.
	ErrAlreadyLeader = errors.New("leader: already leader")

	// ErrNotLeader is returned when leadership is owned by another member.
	ErrNotLeader = errors.New("leader: not leader")
)
