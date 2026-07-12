package sqlleader

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSchemaSQLHasExpectedShape(t *testing.T) {
	for _, required := range []string{
		"public.bluetape_leader_leases",
		"leader_key text primary key",
		"owner_token text not null",
		"lease_until timestamptz not null",
	} {
		if !strings.Contains(strings.ToLower(SchemaSQL), required) {
			t.Fatalf("SchemaSQL missing %q", required)
		}
	}
}

func TestQueriesUseQualifiedServerClock(t *testing.T) {
	for name, query := range map[string]string{
		"acquire": acquireQuery,
		"renew":   renewQuery,
		"lookup":  lookupQuery,
	} {
		if !strings.Contains(query, "public.bluetape_leader_leases") {
			t.Fatalf("%s query is not schema-qualified", name)
		}
		if !strings.Contains(query, "pg_catalog.clock_timestamp()") {
			t.Fatalf("%s query does not use qualified server clock", name)
		}
		if strings.Contains(strings.ReplaceAll(query, "pg_catalog.clock_timestamp()", ""), "clock_timestamp()") {
			t.Fatalf("%s query contains unqualified server clock", name)
		}
	}
}

func TestDurationMicrosCeilsPositive(t *testing.T) {
	for _, tt := range []struct {
		duration time.Duration
		want     int64
	}{
		{time.Nanosecond, 1},
		{time.Microsecond, 1},
		{1001 * time.Nanosecond, 2},
		{24 * time.Hour, int64(24 * time.Hour / time.Microsecond)},
	} {
		if got := durationMicros(tt.duration); got != tt.want {
			t.Fatalf("durationMicros(%s) = %d, want %d", tt.duration, got, tt.want)
		}
	}
}

func TestLeaseStatements(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openPostgresDB(ctx, t)

	t.Run("acquire-observe", func(t *testing.T) { testAcquireObserve(ctx, t, db) })
	t.Run("concurrent-single-winner", func(t *testing.T) { testConcurrentSingleWinner(ctx, t, db) })
	t.Run("active-contention", func(t *testing.T) { testActiveContention(ctx, t, db) })
	t.Run("expiry-takeover", func(t *testing.T) { testExpiryTakeover(ctx, t, db) })
	t.Run("stale-token-delete", func(t *testing.T) { testStaleTokenDelete(ctx, t, db) })
	t.Run("schema-idempotent", func(t *testing.T) { testSchemaIdempotent(ctx, t, db) })
	t.Run("hostile-schema-detected", func(t *testing.T) { testHostileSchemaDetected(ctx, t, db) })
	t.Run("expired-cleanup-safety", func(t *testing.T) { testExpiredCleanupSafety(ctx, t, db) })
}

func testAcquireObserve(ctx context.Context, t *testing.T, db *sql.DB) {
	elector := newTestElector(t, db, "acquire-observe", "member-1", time.Second)
	acquired, err := elector.tryAcquire(ctx)
	if err != nil || !acquired {
		t.Fatalf("tryAcquire() acquired=%v err=%v", acquired, err)
	}
	owner, err := elector.lookupOwner(ctx)
	if err != nil || owner != elector.token {
		t.Fatalf("lookupOwner() owner=%q want=%q err=%v", owner, elector.token, err)
	}
}

func testConcurrentSingleWinner(ctx context.Context, t *testing.T, db *sql.DB) {
	const contenders = 16
	start := make(chan struct{})
	results := make(chan bool, contenders)
	errs := make(chan error, contenders)
	var wg sync.WaitGroup
	for i := range contenders {
		elector := newTestElector(t, db, "concurrent-single-winner", fmt.Sprintf("member-%d", i), time.Second)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			acquired, err := elector.tryAcquire(ctx)
			results <- acquired
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("tryAcquire() error: %v", err)
		}
	}
	winners := 0
	for acquired := range results {
		if acquired {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("winners=%d, want 1", winners)
	}
}

func testActiveContention(ctx context.Context, t *testing.T, db *sql.DB) {
	owner := newTestElector(t, db, "active-contention", "owner", time.Second)
	contender := newTestElector(t, db, "active-contention", "contender", time.Second)
	if acquired, err := owner.tryAcquire(ctx); err != nil || !acquired {
		t.Fatalf("owner tryAcquire() acquired=%v err=%v", acquired, err)
	}
	if acquired, err := contender.tryAcquire(ctx); err != nil || acquired {
		t.Fatalf("contender tryAcquire() acquired=%v err=%v", acquired, err)
	}
}

func testExpiryTakeover(ctx context.Context, t *testing.T, db *sql.DB) {
	owner := newTestElector(t, db, "expiry-takeover", "owner", 150*time.Millisecond)
	contender := newTestElector(t, db, "expiry-takeover", "contender", 150*time.Millisecond)
	if acquired, err := owner.tryAcquire(ctx); err != nil || !acquired {
		t.Fatalf("owner tryAcquire() acquired=%v err=%v", acquired, err)
	}
	time.Sleep(180 * time.Millisecond)
	if acquired, err := contender.tryAcquire(ctx); err != nil || !acquired {
		t.Fatalf("takeover tryAcquire() acquired=%v err=%v", acquired, err)
	}
}

func testStaleTokenDelete(ctx context.Context, t *testing.T, db *sql.DB) {
	stale := newTestElector(t, db, "stale-token-delete", "member", time.Second)
	if acquired, err := stale.tryAcquire(ctx); err != nil || !acquired {
		t.Fatalf("stale tryAcquire() acquired=%v err=%v", acquired, err)
	}
	replacement := "replacement-owner"
	if _, err := db.ExecContext(ctx, `update public.bluetape_leader_leases
set owner_token=$2, lease_until=pg_catalog.clock_timestamp()+interval '1 second'
where leader_key=$1`, stale.key, replacement); err != nil {
		t.Fatalf("replace owner: %v", err)
	}
	if err := stale.deleteOwner(ctx); err != nil {
		t.Fatalf("deleteOwner: %v", err)
	}
	owner, err := stale.lookupOwner(ctx)
	if err != nil || owner != replacement {
		t.Fatalf("replacement owner=%q want=%q err=%v", owner, replacement, err)
	}
}

func testSchemaIdempotent(ctx context.Context, t *testing.T, db *sql.DB) {
	if _, err := db.ExecContext(ctx, SchemaSQL); err != nil {
		t.Fatalf("second SchemaSQL execution: %v", err)
	}
}

func testHostileSchemaDetected(ctx context.Context, t *testing.T, db *sql.DB) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	statements := []string{
		`drop table public.bluetape_leader_leases`,
		`create table public.bluetape_leader_leases (
leader_key text not null, group_name text not null, member_id text not null,
owner_token text not null, lease_until timestamptz not null,
created_at timestamptz not null, updated_at timestamptz not null)`,
		`alter table public.bluetape_leader_leases enable row level security`,
		`create function public.bluetape_leader_test_trigger() returns trigger
language plpgsql as 'begin return new; end'`,
		`create trigger bluetape_leader_test_trigger before insert on public.bluetape_leader_leases
for each row execute function public.bluetape_leader_test_trigger()`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("hostile schema setup: %v", err)
		}
	}
	valid, err := schemaContractValid(ctx, tx)
	if err != nil {
		t.Fatalf("schema contract query: %v", err)
	}
	if valid {
		t.Fatal("hostile compatible-shape schema passed contract gate")
	}
}

func testExpiredCleanupSafety(ctx context.Context, t *testing.T, db *sql.DB) {
	for _, row := range []struct {
		key    string
		expiry string
	}{
		{"cleanup-live", "+ interval '1 hour'"},
		{"cleanup-recent", "- interval '1 hour'"},
		{"cleanup-old", "- interval '2 days'"},
	} {
		query := `insert into public.bluetape_leader_leases
(leader_key, group_name, member_id, owner_token, lease_until, created_at, updated_at)
values ($1,$1,'control','control',pg_catalog.clock_timestamp()` + row.expiry + `,
pg_catalog.clock_timestamp(),pg_catalog.clock_timestamp())`
		if _, err := db.ExecContext(ctx, query, row.key); err != nil {
			t.Fatalf("insert cleanup row %s: %v", row.key, err)
		}
	}
	if _, err := db.ExecContext(ctx, `delete from public.bluetape_leader_leases
where lease_until < pg_catalog.clock_timestamp() - interval '1 day'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	for key, want := range map[string]bool{"cleanup-live": true, "cleanup-recent": true, "cleanup-old": false} {
		var exists bool
		if err := db.QueryRowContext(ctx, `select exists(select 1 from public.bluetape_leader_leases where leader_key=$1)`, key).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists != want {
			t.Fatalf("row %s exists=%v want=%v", key, exists, want)
		}
	}

	opts := leader.Options{Group: "recent", MemberID: "takeover", KeyPrefix: "cleanup", Lease: time.Second, RenewInterval: 100 * time.Millisecond}
	elector, err := New(db, opts)
	if err != nil {
		t.Fatal(err)
	}
	if acquired, err := elector.tryAcquire(ctx); err != nil || !acquired {
		t.Fatalf("takeover after cleanup acquired=%v err=%v", acquired, err)
	}
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func schemaContractValid(ctx context.Context, db queryer) (bool, error) {
	var relkind string
	var rls, forceRLS bool
	err := db.QueryRowContext(ctx, `select c.relkind::text, c.relrowsecurity, c.relforcerowsecurity
from pg_catalog.pg_class c join pg_catalog.pg_namespace n on n.oid=c.relnamespace
where n.nspname='public' and c.relname='bluetape_leader_leases'`).Scan(&relkind, &rls, &forceRLS)
	if err != nil {
		return false, err
	}
	var primaryKey string
	err = db.QueryRowContext(ctx, `select coalesce(array_agg(a.attname order by k.ordinality), array[]::name[])
from pg_catalog.pg_constraint con
cross join lateral unnest(con.conkey) with ordinality as k(attnum, ordinality)
join pg_catalog.pg_attribute a on a.attrelid=con.conrelid and a.attnum=k.attnum
where con.conrelid='public.bluetape_leader_leases'::regclass and con.contype='p'`).Scan(&primaryKey)
	if err != nil {
		return false, err
	}
	var triggers int
	err = db.QueryRowContext(ctx, `select count(*) from pg_catalog.pg_trigger
where tgrelid='public.bluetape_leader_leases'::regclass and not tgisinternal`).Scan(&triggers)
	if err != nil {
		return false, err
	}
	return relkind == "r" && !rls && !forceRLS && primaryKey == "{leader_key}" && triggers == 0, nil
}

func newTestElector(t *testing.T, db *sql.DB, group, member string, lease time.Duration) *Elector {
	t.Helper()
	elector, err := New(db, leader.Options{
		Group:         group,
		MemberID:      member,
		Lease:         lease,
		RenewInterval: lease / 3,
		KeyPrefix:     "sqlleader-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return elector
}

func openPostgresDB(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()
	dsn := postgrestestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, SchemaSQL); err != nil {
		t.Fatal(err)
	}
	return db
}
