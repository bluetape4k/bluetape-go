package sqlleader

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

const acquireQuery = `insert into public.bluetape_leader_leases (
    leader_key, group_name, member_id, owner_token,
    lease_until, created_at, updated_at
) values (
    $1, $2, $3, $4,
    pg_catalog.clock_timestamp() + $5::bigint * interval '1 microsecond',
    pg_catalog.clock_timestamp(), pg_catalog.clock_timestamp()
)
on conflict (leader_key) do update set
    group_name = excluded.group_name,
    member_id = excluded.member_id,
    owner_token = excluded.owner_token,
    lease_until = pg_catalog.clock_timestamp() + $5::bigint * interval '1 microsecond',
    updated_at = pg_catalog.clock_timestamp()
where public.bluetape_leader_leases.lease_until <= pg_catalog.clock_timestamp()
   or public.bluetape_leader_leases.owner_token = excluded.owner_token
returning owner_token, lease_until`

const renewQuery = `update public.bluetape_leader_leases
set lease_until = pg_catalog.clock_timestamp() + $3::bigint * interval '1 microsecond',
    updated_at = pg_catalog.clock_timestamp()
where leader_key = $1 and owner_token = $2
  and lease_until > pg_catalog.clock_timestamp()
returning lease_until`

const deleteQuery = `delete from public.bluetape_leader_leases
where leader_key = $1 and owner_token = $2`

const lookupQuery = `select owner_token from public.bluetape_leader_leases
where leader_key = $1 and lease_until > pg_catalog.clock_timestamp()`

func (e *Elector) tryAcquire(ctx context.Context) (bool, error) {
	var owner string
	var leaseUntil time.Time
	err := e.db.QueryRowContext(
		ctx,
		acquireQuery,
		e.key,
		e.opts.Group,
		e.opts.MemberID,
		e.token,
		durationMicros(e.opts.Lease),
	).Scan(&owner, &leaseUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, leader.NewOperationError("postgres", "campaign", err)
	}
	return owner == e.token, nil
}

func (e *Elector) renew(ctx context.Context) (bool, error) {
	var leaseUntil time.Time
	err := e.db.QueryRowContext(
		ctx,
		renewQuery,
		e.key,
		e.token,
		durationMicros(e.opts.Lease),
	).Scan(&leaseUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, leader.NewOperationError("postgres", "renew", err)
	}
	return true, nil
}

func (e *Elector) deleteOwner(ctx context.Context) error {
	if _, err := e.db.ExecContext(ctx, deleteQuery, e.key, e.token); err != nil {
		return leader.NewOperationError("postgres", "resign", err)
	}
	return nil
}

func (e *Elector) lookupOwner(ctx context.Context) (string, error) {
	var owner string
	err := e.db.QueryRowContext(ctx, lookupQuery, e.key).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", leader.NewOperationError("postgres", "lookup", err)
	}
	return owner, nil
}

func durationMicros(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	micros := int64(duration / time.Microsecond)
	if duration%time.Microsecond != 0 {
		micros++
	}
	return micros
}
