package leader

import (
	"time"

	"github.com/bluetape4k/bluetape-go/core"
)

const (
	defaultLease         = 10 * time.Second
	defaultRenewInterval = 3 * time.Second
	defaultKeyPrefix     = "bluetape:leader"
)

// Options configures one leader election participant.
type Options struct {
	Group         string
	MemberID      string
	Lease         time.Duration
	RenewInterval time.Duration
	KeyPrefix     string
}

// Normalize validates options and fills stable defaults.
func (o Options) Normalize() (Options, error) {
	if err := core.RequireNotBlank("group", o.Group); err != nil {
		return Options{}, err
	}
	if err := core.RequireNotBlank("memberID", o.MemberID); err != nil {
		return Options{}, err
	}

	if o.Lease <= 0 {
		o.Lease = defaultLease
	}
	if o.RenewInterval <= 0 {
		o.RenewInterval = defaultRenewInterval
	}
	if o.KeyPrefix == "" {
		o.KeyPrefix = defaultKeyPrefix
	}
	return o, nil
}
