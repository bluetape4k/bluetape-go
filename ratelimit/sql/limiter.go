package sqlratelimit

import (
	"database/sql"
	"errors"
)

// Limiter is a PostgreSQL-backed token-bucket limiter.
type Limiter struct {
	db       *sql.DB
	opts     options
	testHook func(operation string, phase testPhase, key string) error
}

type testPhase string

const (
	phaseBeforeLinearize testPhase = "before-linearize"
	phaseAfterLinearize  testPhase = "after-linearize"
)

// New creates a limiter with a caller-owned database pool.
// New performs no database I/O and never closes db.
func New(db *sql.DB, opts Options) (*Limiter, error) {
	if db == nil {
		return nil, errors.New("postgres rate limiter database must not be nil")
	}
	normalized, err := opts.normalize()
	if err != nil {
		return nil, err
	}
	return &Limiter{db: db, opts: normalized}, nil
}
