package leader

import "context"

// Elector coordinates leadership for one member in one election group.
type Elector interface {
	Campaign(ctx context.Context) error
	Resign(ctx context.Context) error
	IsLeader() bool
	Leader(ctx context.Context) (string, error)
}
