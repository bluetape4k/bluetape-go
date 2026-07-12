package sqlleader

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/bluetape4k/bluetape-go/leader"
)

// Elector is a PostgreSQL-backed single leader elector.
type Elector struct {
	db    *sql.DB
	opts  leader.Options
	key   string
	token string

	mu          sync.RWMutex
	owned       bool
	campaigning bool
	cleanup     bool
	generation  uint64
	cancel      context.CancelFunc
	done        chan struct{}
	testHook    func(operation, phase string) error
}

// New creates a PostgreSQL-backed elector over the caller-owned database pool.
//
// The database must route every operation to the same writable primary. The
// elector never executes migrations and never closes db.
func New(db *sql.DB, opts leader.Options) (*Elector, error) {
	if db == nil {
		return nil, errors.New("postgres leader database must not be nil")
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return nil, err
	}
	if normalized.RenewInterval >= normalized.Lease {
		return nil, errors.New("postgres leader renew interval must be less than lease")
	}
	token, err := randomToken(normalized.MemberID)
	if err != nil {
		return nil, err
	}
	return &Elector{
		db:    db,
		opts:  normalized,
		key:   normalized.KeyPrefix + ":" + normalized.Group,
		token: token,
	}, nil
}

func randomToken(memberID string) (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return memberID + ":" + hex.EncodeToString(data[:]), nil
}
