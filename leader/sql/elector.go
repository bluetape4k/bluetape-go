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

// Elector는 PostgreSQL-backed single leader elector다.
type Elector struct {
	db    *sql.DB
	opts  leader.Options
	key   string
	token string

	mu          sync.RWMutex
	owned       bool
	campaigning bool
	cleanup     bool
	resigning   int
	resolved    bool
	generation  uint64
	cancel      context.CancelFunc
	done        chan struct{}
	testHook    func(operation, phase string) error
}

// New는 호출자가 소유한 database pool 위에 PostgreSQL-backed elector를 생성한다.
//
// database는 모든 operation을 동일한 writable primary로 route해야 한다. elector는 migration을 실행하지 않고
// db를 닫지 않는다. option normalization 뒤 RenewInterval은 Lease보다 작아야 한다.
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
