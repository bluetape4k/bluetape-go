package sqlratelimit

import (
	"context"
	"database/sql"
	"fmt"
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

func TestRateLimitCatalogValidatorAcceptsExactSchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	_, db := openRateLimitPostgres(ctx, t)
	var owner string
	if err := db.QueryRowContext(ctx, `select current_user`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err := validateRateLimitCatalog(ctx, db, owner); err != nil {
		t.Fatalf("exact catalog rejected: %v", err)
	}
}

func TestRateLimitCatalogValidatorRejectsHostileDriftBeforeProviderTraffic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	_, db := openRateLimitPostgres(ctx, t)
	var owner string
	if err := db.QueryRowContext(ctx, `select current_user`).Scan(&owner); err != nil {
		t.Fatal(err)
	}

	wrongColumnOrderSchema := strings.Replace(
		SchemaSQL,
		"    namespace bytea not null,\n    bucket_key bytea not null,",
		"    bucket_key bytea not null,\n    namespace bytea not null,",
		1,
	)
	tests := []struct {
		name       string
		statements []string
	}{
		{
			name: "column-order",
			statements: []string{
				`drop table public.bluetape_ratelimit_buckets cascade`,
				wrongColumnOrderSchema,
			},
		},
		{
			name: "column-type",
			statements: []string{
				`alter table public.bluetape_ratelimit_buckets alter column tokens_micros type numeric(30,5)`,
			},
		},
		{
			name: "column-nullability",
			statements: []string{
				`alter table public.bluetape_ratelimit_buckets alter column last_allowed drop not null`,
			},
		},
		{
			name: "primary-key-order",
			statements: []string{
				`alter table public.bluetape_ratelimit_buckets drop constraint bluetape_ratelimit_buckets_pkey`,
				`alter table public.bluetape_ratelimit_buckets add primary key (bucket_key,namespace)`,
			},
		},
		{
			name: "check-definition",
			statements: []string{
				`alter table public.bluetape_ratelimit_buckets drop constraint bluetape_ratelimit_buckets_rate_micros_per_second_check`,
				`alter table public.bluetape_ratelimit_buckets add constraint bluetape_ratelimit_buckets_rate_micros_per_second_check check (rate_micros_per_second >= 0)`,
			},
		},
		{
			name: "check-not-validated",
			statements: []string{
				`alter table public.bluetape_ratelimit_buckets drop constraint bluetape_ratelimit_buckets_rate_micros_per_second_check`,
				`alter table public.bluetape_ratelimit_buckets add constraint bluetape_ratelimit_buckets_rate_micros_per_second_check check (rate_micros_per_second > 0) not valid`,
			},
		},
		{
			name: "expiry-index-other-relation",
			statements: []string{
				`drop index public.bluetape_ratelimit_buckets_expires_at_idx`,
				`create table public.ratelimit_catalog_other(expires_at timestamptz not null)`,
				`create index bluetape_ratelimit_buckets_expires_at_idx on public.ratelimit_catalog_other(expires_at)`,
			},
		},
		{
			name: "expiry-index-definition",
			statements: []string{
				`drop index public.bluetape_ratelimit_buckets_expires_at_idx`,
				`create index bluetape_ratelimit_buckets_expires_at_idx on public.bluetape_ratelimit_buckets(updated_at)`,
			},
		},
		{
			name: "owner",
			statements: []string{
				`create role ratelimit_catalog_attacker nologin`,
				`alter table public.bluetape_ratelimit_buckets owner to ratelimit_catalog_attacker`,
			},
		},
		{
			name:       "row-level-security",
			statements: []string{`alter table public.bluetape_ratelimit_buckets enable row level security`},
		},
		{
			name: "forced-row-level-security",
			statements: []string{
				`alter table public.bluetape_ratelimit_buckets enable row level security`,
				`alter table public.bluetape_ratelimit_buckets force row level security`,
			},
		},
		{
			name: "policy",
			statements: []string{
				`create policy ratelimit_catalog_hostile on public.bluetape_ratelimit_buckets using (false)`,
			},
		},
		{
			name: "trigger",
			statements: []string{
				`create function public.ratelimit_catalog_hostile_trigger() returns trigger language plpgsql as 'begin return new; end'`,
				`create trigger ratelimit_catalog_hostile before insert on public.bluetape_ratelimit_buckets for each row execute function public.ratelimit_catalog_hostile_trigger()`,
			},
		},
		{
			name: "relation-kind",
			statements: []string{
				`drop table public.bluetape_ratelimit_buckets cascade`,
				`create view public.bluetape_ratelimit_buckets as select ''::bytea as namespace,''::bytea as bucket_key`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			for _, statement := range tt.statements {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					t.Fatalf("hostile mutation %q: %v", statement, err)
				}
			}
			providerCalls := 0
			if err := validateRateLimitCatalog(ctx, tx, owner); err == nil {
				providerCalls++
			}
			if providerCalls != 0 {
				t.Fatalf("hostile catalog passed preflight and enabled %d provider calls", providerCalls)
			}
		})
	}
}

type catalogQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var _ catalogQuerier = (*sql.DB)(nil)
var _ catalogQuerier = (*sql.Tx)(nil)

func catalogMismatch(format string, args ...any) error {
	return fmt.Errorf("rate limit catalog mismatch: "+format, args...)
}

type catalogColumn struct {
	name     string
	typeName string
	notNull  bool
}

func validateRateLimitCatalog(ctx context.Context, db catalogQuerier, expectedOwner string) error {
	var owner, relkind string
	var rls, forceRLS bool
	var policies, triggers int
	err := db.QueryRowContext(ctx, `select owner.rolname,c.relkind::text,c.relrowsecurity,c.relforcerowsecurity,
(select count(*) from pg_catalog.pg_policy p where p.polrelid=c.oid),
(select count(*) from pg_catalog.pg_trigger t where t.tgrelid=c.oid and not t.tgisinternal)
from pg_catalog.pg_class c
join pg_catalog.pg_namespace n on n.oid=c.relnamespace
join pg_catalog.pg_roles owner on owner.oid=c.relowner
where n.nspname='public' and c.relname='bluetape_ratelimit_buckets'`).Scan(
		&owner, &relkind, &rls, &forceRLS, &policies, &triggers)
	if err != nil {
		return catalogMismatch("relation: %v", err)
	}
	if owner != expectedOwner || relkind != "r" || rls || forceRLS || policies != 0 || triggers != 0 {
		return catalogMismatch(
			"relation owner=%q want=%q kind=%q rls=%v force=%v policies=%d triggers=%d",
			owner, expectedOwner, relkind, rls, forceRLS, policies, triggers,
		)
	}

	wantColumns := []catalogColumn{
		{name: "namespace", typeName: "bytea", notNull: true},
		{name: "bucket_key", typeName: "bytea", notNull: true},
		{name: "rate_micros_per_second", typeName: "bigint", notNull: true},
		{name: "burst_micros", typeName: "bigint", notNull: true},
		{name: "idle_ttl_micros", typeName: "bigint", notNull: true},
		{name: "tokens_micros", typeName: "numeric(30,6)", notNull: true},
		{name: "last_allowed", typeName: "boolean", notNull: true},
		{name: "updated_at", typeName: "timestamp with time zone", notNull: true},
		{name: "expires_at", typeName: "timestamp with time zone", notNull: true},
	}
	rows, err := db.QueryContext(ctx, `select a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),a.attnotnull
from pg_catalog.pg_attribute a
where a.attrelid='public.bluetape_ratelimit_buckets'::regclass
  and a.attnum>0 and not a.attisdropped
order by a.attnum`)
	if err != nil {
		return catalogMismatch("columns: %v", err)
	}
	var columns []catalogColumn
	for rows.Next() {
		var column catalogColumn
		if err := rows.Scan(&column.name, &column.typeName, &column.notNull); err != nil {
			_ = rows.Close()
			return catalogMismatch("column scan: %v", err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return catalogMismatch("column rows: %v", err)
	}
	if err := rows.Close(); err != nil {
		return catalogMismatch("column close: %v", err)
	}
	if len(columns) != len(wantColumns) {
		return catalogMismatch("columns=%v want=%v", columns, wantColumns)
	}
	for i := range wantColumns {
		if columns[i] != wantColumns[i] {
			return catalogMismatch("column[%d]=%v want=%v", i, columns[i], wantColumns[i])
		}
	}

	wantConstraints := map[string]struct{}{
		"primarykey(namespace,bucket_key)":  {},
		"check((rate_micros_per_second>0))": {},
		"check((burst_micros>0))":           {},
		"check((idle_ttl_micros>0))":        {},
		"check(((tokens_micros>=(0)::numeric)and(tokens_micros<=(burst_micros)::numeric)))": {},
		"check((expires_at>=updated_at))": {},
	}
	rows, err = db.QueryContext(ctx, `select con.convalidated,con.condeferrable,con.condeferred,
pg_catalog.pg_get_constraintdef(con.oid,false)
from pg_catalog.pg_constraint con
where con.conrelid='public.bluetape_ratelimit_buckets'::regclass
order by con.contype,con.conname`)
	if err != nil {
		return catalogMismatch("constraints: %v", err)
	}
	gotConstraints := make(map[string]struct{})
	for rows.Next() {
		var validated, deferrable, deferred bool
		var definition string
		if err := rows.Scan(&validated, &deferrable, &deferred, &definition); err != nil {
			_ = rows.Close()
			return catalogMismatch("constraint scan: %v", err)
		}
		if !validated || deferrable || deferred {
			_ = rows.Close()
			return catalogMismatch("constraint state definition=%q validated=%v deferrable=%v deferred=%v", definition, validated, deferrable, deferred)
		}
		gotConstraints[canonicalCatalogDefinition(definition)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return catalogMismatch("constraint rows: %v", err)
	}
	if err := rows.Close(); err != nil {
		return catalogMismatch("constraint close: %v", err)
	}
	if len(gotConstraints) != len(wantConstraints) {
		return catalogMismatch("constraints=%v want=%v", gotConstraints, wantConstraints)
	}
	for definition := range wantConstraints {
		if _, ok := gotConstraints[definition]; !ok {
			return catalogMismatch("missing constraint %q in %v", definition, gotConstraints)
		}
	}

	var tableSchema, tableName, accessMethod, indexColumns, predicate, expressions string
	var valid, ready, primary, unique bool
	var keyColumns, allColumns int
	err = db.QueryRowContext(ctx, `select table_ns.nspname,table_class.relname,am.amname,
i.indisvalid,i.indisready,i.indisprimary,i.indisunique,i.indnkeyatts,i.indnatts,
coalesce((select string_agg(a.attname,',' order by key.ordinality)
 from unnest(i.indkey) with ordinality as key(attnum,ordinality)
 left join pg_catalog.pg_attribute a on a.attrelid=i.indrelid and a.attnum=key.attnum),''),
coalesce(pg_catalog.pg_get_expr(i.indpred,i.indrelid),''),
coalesce(pg_catalog.pg_get_expr(i.indexprs,i.indrelid),'')
from pg_catalog.pg_class index_class
join pg_catalog.pg_namespace index_ns on index_ns.oid=index_class.relnamespace
join pg_catalog.pg_index i on i.indexrelid=index_class.oid
join pg_catalog.pg_class table_class on table_class.oid=i.indrelid
join pg_catalog.pg_namespace table_ns on table_ns.oid=table_class.relnamespace
join pg_catalog.pg_am am on am.oid=index_class.relam
where index_ns.nspname='public' and index_class.relname='bluetape_ratelimit_buckets_expires_at_idx'`).Scan(
		&tableSchema, &tableName, &accessMethod, &valid, &ready, &primary, &unique,
		&keyColumns, &allColumns, &indexColumns, &predicate, &expressions)
	if err != nil {
		return catalogMismatch("expiry index: %v", err)
	}
	if tableSchema != "public" || tableName != "bluetape_ratelimit_buckets" || accessMethod != "btree" ||
		!valid || !ready || primary || unique || keyColumns != 1 || allColumns != 1 ||
		indexColumns != "expires_at" || predicate != "" || expressions != "" {
		return catalogMismatch(
			"expiry index target=%s.%s method=%q valid=%v ready=%v primary=%v unique=%v key/all=%d/%d columns=%q predicate=%q expressions=%q",
			tableSchema, tableName, accessMethod, valid, ready, primary, unique,
			keyColumns, allColumns, indexColumns, predicate, expressions,
		)
	}
	return nil
}

func canonicalCatalogDefinition(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), "")
}

func assertRateLimitCatalog(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	if err := validateRateLimitCatalog(ctx, db, "ratelimit_migration_owner"); err != nil {
		t.Fatal(err)
	}
	var usage, create, dml, dangerous, member bool
	err := db.QueryRowContext(ctx, `select
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
