package sqlleader

import (
	"context"
	"database/sql"
	"net/url"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/bluetape4k/bluetape-go/leader/leadertest"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
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
	if err := admin.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	adminConn, err := admin.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer adminConn.Close()
	setup := []string{
		`create role leader_migration_owner nologin`,
		`create role leader_runtime login password 'runtime-pass'`,
		`grant usage, create on schema public to leader_migration_owner`,
		`set role leader_migration_owner`,
		SchemaSQL,
		`create function public.leader_noop_trigger() returns trigger language plpgsql as 'begin return new; end'`,
		`reset role`,
		`revoke create on schema public from public`,
		`revoke create on schema public from leader_migration_owner`,
		`grant usage on schema public to leader_runtime`,
		`grant select, insert, update, delete on public.bluetape_leader_leases to leader_runtime`,
	}
	for _, statement := range setup {
		if _, err := adminConn.ExecContext(ctx, statement); err != nil {
			t.Fatalf("security setup failed: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`drop owned by leader_runtime`)
		_, _ = admin.Exec(`drop owned by leader_migration_owner cascade`)
		_, _ = admin.Exec(`drop role if exists leader_runtime`)
		_, _ = admin.Exec(`drop role if exists leader_migration_owner`)
	})

	runtimeDSN := roleDSN(t, dsn, "leader_runtime", "runtime-pass")
	runtimeDB, err := sql.Open("pgx", runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtimeDB.Close() })
	if err := runtimeDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	control := newPostgresConformanceControl(runtimeDB)
	leadertest.Run(t, leadertest.Harness{
		New: func(_ testing.TB, opts leader.Options) (leader.Elector, error) {
			elector, err := New(runtimeDB, opts)
			if err == nil {
				elector.testHook = control.hook(opts)
			}
			return elector, err
		},
		Control: control,
	})

	denied := []string{
		`create table public.runtime_forbidden(id integer)`,
		`alter table public.bluetape_leader_leases add column forbidden integer`,
		`truncate table public.bluetape_leader_leases`,
		`create trigger runtime_forbidden before insert on public.bluetape_leader_leases
for each row execute function public.leader_noop_trigger()`,
	}
	for _, statement := range denied {
		if _, err := runtimeDB.ExecContext(ctx, statement); err == nil {
			t.Fatalf("runtime role unexpectedly executed %q", statement)
		}
	}

	assertRuntimeRoleCatalog(t, ctx, admin)
}

func roleDSN(t *testing.T, dsn, username, password string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword(username, password)
	return parsed.String()
}

func assertRuntimeRoleCatalog(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var usage, create, dml, dangerous, member, rls, forceRLS bool
	var owner, primaryKey string
	var triggers int
	err := db.QueryRowContext(ctx, `select
has_schema_privilege('leader_runtime','public','usage'),
has_schema_privilege('leader_runtime','public','create'),
has_table_privilege('leader_runtime','public.bluetape_leader_leases','select,insert,update,delete'),
has_table_privilege('leader_runtime','public.bluetape_leader_leases','truncate,references,trigger'),
pg_has_role('leader_runtime','leader_migration_owner','member'),
r.rolname,
c.relrowsecurity,c.relforcerowsecurity,
coalesce((select string_agg(a.attname,',' order by k.ordinality)
  from pg_catalog.pg_constraint con
  cross join lateral unnest(con.conkey) with ordinality as k(attnum,ordinality)
  join pg_catalog.pg_attribute a on a.attrelid=con.conrelid and a.attnum=k.attnum
  where con.conrelid=c.oid and con.contype='p'),''),
(select count(*) from pg_catalog.pg_trigger t where t.tgrelid=c.oid and not t.tgisinternal)
from pg_catalog.pg_class c
join pg_catalog.pg_namespace n on n.oid=c.relnamespace
join pg_catalog.pg_roles r on r.oid=c.relowner
where n.nspname='public' and c.relname='bluetape_leader_leases'`).Scan(
		&usage, &create, &dml, &dangerous, &member, &owner, &rls, &forceRLS, &primaryKey, &triggers)
	if err != nil {
		t.Fatal(err)
	}
	if !usage || create || !dml || dangerous || member {
		t.Fatalf("runtime privileges usage=%v create=%v dml=%v dangerous=%v member=%v", usage, create, dml, dangerous, member)
	}
	if owner != "leader_migration_owner" || rls || forceRLS || primaryKey != "leader_key" || triggers != 0 {
		t.Fatalf("catalog owner=%q rls=%v forceRLS=%v pk=%q triggers=%d", owner, rls, forceRLS, primaryKey, triggers)
	}
}
