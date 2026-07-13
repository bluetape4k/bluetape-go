package sqlratelimit

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
	"testing"
	"time"

	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRuntimeRoleLeastPrivilege(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	dsn := postgrestestcontainer.Start(ctx, t)
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	conn, err := admin.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	setup := []string{
		`create role ratelimit_migration_owner nologin`,
		`create role ratelimit_runtime login password 'runtime-pass'`,
		`create role ratelimit_probe nologin`,
		`alter schema public owner to ratelimit_migration_owner`,
		`revoke create on schema public from public`,
		`grant usage,create on schema public to ratelimit_migration_owner`,
		`set role ratelimit_migration_owner`,
		`set lock_timeout='5s'`,
		`set statement_timeout='10s'`,
		SchemaSQL,
		`reset role`,
		`revoke create on schema public from ratelimit_migration_owner`,
		`grant usage on schema public to ratelimit_runtime`,
		`grant select,insert,update,delete on public.bluetape_ratelimit_buckets to ratelimit_runtime`,
	}
	for _, statement := range setup {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			t.Fatalf("security setup %q: %v", statement, err)
		}
	}

	runtimeDB, err := sql.Open("pgx", rateRoleDSN(t, dsn, "ratelimit_runtime", "runtime-pass"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtimeDB.Close() })
	limiter, err := New(runtimeDB, Options{Namespace: "least-privilege", RatePerSecond: 10, Burst: 2, IdleTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := limiter.Allow(ctx, "key", 1); err != nil || !result.Allowed {
		t.Fatalf("runtime Allow = %+v, %v", result, err)
	}
	if _, err := limiter.Cleanup(ctx, 10); err != nil {
		t.Fatalf("runtime Cleanup: %v", err)
	}
	for _, statement := range []string{
		`create table public.runtime_forbidden(id integer)`,
		`alter table public.bluetape_ratelimit_buckets add column forbidden integer`,
		`truncate table public.bluetape_ratelimit_buckets`,
	} {
		if _, err := runtimeDB.ExecContext(ctx, statement); err == nil {
			t.Fatalf("runtime unexpectedly executed %q", statement)
		}
	}
	_, _ = runtimeDB.ExecContext(ctx, `grant select on public.bluetape_ratelimit_buckets to ratelimit_probe`)
	var probeSelect bool
	if err := admin.QueryRowContext(ctx, `select has_table_privilege('ratelimit_probe','public.bluetape_ratelimit_buckets','select')`).Scan(&probeSelect); err != nil {
		t.Fatal(err)
	}
	if probeSelect {
		t.Fatal("runtime role delegated table privileges")
	}
	assertRateLimitCatalog(ctx, t, admin)
}

func TestHostileExistingSchemaFailsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	_, db := openRateLimitPostgres(ctx, t)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(ctx, `drop table public.bluetape_ratelimit_buckets;
create table public.bluetape_ratelimit_buckets(namespace bytea,bucket_key bytea)`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, SchemaSQL); err == nil {
		t.Fatal("SchemaSQL accepted hostile relation")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func assertRateLimitCatalog(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	var owner, relkind, primaryKey string
	var rls, forceRLS, indexValid, indexReady bool
	var triggers int
	err := db.QueryRowContext(ctx, `select owner.rolname,c.relkind::text,c.relrowsecurity,c.relforcerowsecurity,
coalesce((select string_agg(a.attname,',' order by k.ordinality)
 from pg_catalog.pg_constraint con
 cross join lateral unnest(con.conkey) with ordinality as k(attnum,ordinality)
 join pg_catalog.pg_attribute a on a.attrelid=con.conrelid and a.attnum=k.attnum
 where con.conrelid=c.oid and con.contype='p'),''),
coalesce(i.indisvalid,false),coalesce(i.indisready,false),
(select count(*) from pg_catalog.pg_trigger t where t.tgrelid=c.oid and not t.tgisinternal)
from pg_catalog.pg_class c
join pg_catalog.pg_namespace n on n.oid=c.relnamespace
join pg_catalog.pg_roles owner on owner.oid=c.relowner
left join pg_catalog.pg_class ic on ic.relnamespace=n.oid and ic.relname='bluetape_ratelimit_buckets_expires_at_idx'
left join pg_catalog.pg_index i on i.indexrelid=ic.oid
where n.nspname='public' and c.relname='bluetape_ratelimit_buckets'`).Scan(
		&owner, &relkind, &rls, &forceRLS, &primaryKey, &indexValid, &indexReady, &triggers)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "ratelimit_migration_owner" || relkind != "r" || rls || forceRLS ||
		primaryKey != "namespace,bucket_key" || !indexValid || !indexReady || triggers != 0 {
		t.Fatalf("catalog owner=%q kind=%q rls=%v force=%v pk=%q index=%v/%v triggers=%d",
			owner, relkind, rls, forceRLS, primaryKey, indexValid, indexReady, triggers)
	}
	var usage, create, dml, dangerous, member bool
	err = db.QueryRowContext(ctx, `select
has_schema_privilege('ratelimit_runtime','public','usage'),
has_schema_privilege('ratelimit_runtime','public','create'),
has_table_privilege('ratelimit_runtime','public.bluetape_ratelimit_buckets','select,insert,update,delete'),
has_table_privilege('ratelimit_runtime','public.bluetape_ratelimit_buckets','truncate,references,trigger'),
pg_has_role('ratelimit_runtime','ratelimit_migration_owner','member')`).Scan(&usage, &create, &dml, &dangerous, &member)
	if err != nil {
		t.Fatal(err)
	}
	if !usage || create || !dml || dangerous || member {
		t.Fatalf("privileges usage=%v create=%v dml=%v dangerous=%v member=%v", usage, create, dml, dangerous, member)
	}
}

func rateRoleDSN(t *testing.T, dsn, username, password string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword(username, password)
	return parsed.String()
}

func containsFold(value, fragment string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(fragment))
}
