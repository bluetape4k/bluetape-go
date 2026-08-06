package sqlleader

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/bluetape4k/bluetape-go/leader/leadertest"
)

func TestPostgresElectorConformance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openPostgresDB(ctx, t)
	control := newPostgresConformanceControl(db)
	leadertest.Run(t, leadertest.Harness{
		New: func(_ testing.TB, opts leader.Options) (leader.Elector, error) {
			elector, err := New(db, opts)
			if err == nil {
				elector.testHook = control.hook(opts)
			}
			return elector, err
		},
		Control: control,
	})
}

type postgresConformanceControl struct {
	db *sql.DB
	mu sync.Mutex

	failures      map[string]map[leadertest.Operation]error
	probeFailures map[string]error
	counts        map[string]map[leadertest.Operation]int64
}

func newPostgresConformanceControl(db *sql.DB) *postgresConformanceControl {
	return &postgresConformanceControl{
		db:            db,
		failures:      make(map[string]map[leadertest.Operation]error),
		probeFailures: make(map[string]error),
		counts:        make(map[string]map[leadertest.Operation]int64),
	}
}

func (c *postgresConformanceControl) ReplaceOwner(ctx context.Context, opts leader.Options, owner string) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil || strings.TrimSpace(owner) == "" {
		return errors.New("postgres leader conformance: invalid control input")
	}
	_, err = c.db.ExecContext(ctx, `insert into public.bluetape_leader_leases
(leader_key,group_name,member_id,owner_token,lease_until,created_at,updated_at)
values($1,$2,'control',$3,pg_catalog.clock_timestamp()+$4::bigint*interval '1 microsecond',
pg_catalog.clock_timestamp(),pg_catalog.clock_timestamp())
on conflict(leader_key) do update set owner_token=excluded.owner_token,
lease_until=excluded.lease_until,updated_at=pg_catalog.clock_timestamp()`,
		postgresLeaderKey(normalized), normalized.Group, owner, durationMicros(normalized.Lease))
	return err
}

func (c *postgresConformanceControl) FailNext(ctx context.Context, opts leader.Options, operation leadertest.Operation, cause error) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil || cause == nil || !validPostgresOperation(operation) {
		return errors.New("postgres leader conformance: invalid failure injection")
	}
	key := postgresLeaderKey(normalized)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures[key] == nil {
		c.failures[key] = make(map[leadertest.Operation]error)
	}
	c.failures[key][operation] = cause
	return nil
}

func (c *postgresConformanceControl) Owner(ctx context.Context, opts leader.Options) (string, error) {
	if ctx == nil {
		return "", leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return "", errors.New("postgres leader conformance: invalid options")
	}
	var owner string
	err = c.db.QueryRowContext(ctx, `select owner_token from public.bluetape_leader_leases
where leader_key=$1 and lease_until>pg_catalog.clock_timestamp()`, postgresLeaderKey(normalized)).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return owner, err
}

func (c *postgresConformanceControl) OperationCount(opts leader.Options, operation leadertest.Operation) int64 {
	normalized, err := opts.Normalize()
	if err != nil || !validPostgresOperation(operation) {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[postgresLeaderKey(normalized)][operation]
}

func (c *postgresConformanceControl) hook(opts leader.Options) func(string, string) error {
	key := postgresLeaderKey(opts)
	return func(rawOperation, phase string) error {
		operation := leadertest.Operation(rawOperation)
		if !validPostgresOperation(operation) {
			return errors.New("postgres leader conformance: invalid mutation operation")
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if phase == "reconcile" {
			err := c.probeFailures[key]
			delete(c.probeFailures, key)
			return err
		}
		if phase == "attempt" {
			if c.counts[key] == nil {
				c.counts[key] = make(map[leadertest.Operation]int64)
			}
			c.counts[key][operation]++
			return nil
		}
		if phase != "after" {
			return nil
		}
		if c.counts[key] == nil {
			c.counts[key] = make(map[leadertest.Operation]int64)
		}
		if operation != leadertest.OperationCampaign {
			c.counts[key][operation]++
		}
		err := c.failures[key][operation]
		delete(c.failures[key], operation)
		if err != nil && operation == leadertest.OperationCampaign {
			c.probeFailures[key] = err
		}
		return err
	}
}

func postgresLeaderKey(opts leader.Options) string {
	normalized, err := opts.Normalize()
	if err != nil {
		return ""
	}
	return normalized.KeyPrefix + ":" + normalized.Group
}

func validPostgresOperation(operation leadertest.Operation) bool {
	switch operation {
	case leadertest.OperationCampaign, leadertest.OperationRenew, leadertest.OperationResign:
		return true
	default:
		return false
	}
}
