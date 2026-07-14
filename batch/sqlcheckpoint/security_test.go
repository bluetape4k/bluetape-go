package sqlcheckpoint

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

const (
	checkpointMigrationOwner = "sqlcheckpoint_migration_owner"
	checkpointRuntimeRole    = "sqlcheckpoint_runtime"
	checkpointRuntimePass    = "sqlcheckpoint-runtime-pass"
)

type checkpointCatalogQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type checkpointCatalogColumn struct {
	name       string
	typeName   string
	notNull    bool
	columnACLs int
}

type checkpointSecurityFixture struct {
	ctx   context.Context
	dsn   string
	admin *sql.DB
}

func newCheckpointSecurityFixture(t *testing.T) *checkpointSecurityFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), postgresTestTimeout)
	t.Cleanup(cancel)
	dsn, admin := startReadyPostgresPool(ctx, t, "security PostgreSQL")
	t.Cleanup(func() { _ = admin.Close() })
	conn, err := admin.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// Production order is part of the contract: close PUBLIC creation first,
	// create the non-login owner, migrate as that owner, preflight, prove the
	// runtime is denied, and only then grant its three DML privileges.
	setup := []string{
		`revoke create on schema public from public`,
		`create role sqlcheckpoint_migration_owner nologin`,
		`create role sqlcheckpoint_runtime login password 'sqlcheckpoint-runtime-pass' noinherit`,
		`create role sqlcheckpoint_forbidden_grantee nologin`,
		`alter schema public owner to sqlcheckpoint_migration_owner`,
		`set role sqlcheckpoint_migration_owner`,
		`set lock_timeout='5s'`,
		`set statement_timeout='10s'`,
		SchemaSQL,
		`reset role`,
	}
	for _, statement := range setup {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			t.Fatalf("security setup %q: %v", statement, err)
		}
	}
	return &checkpointSecurityFixture{ctx: ctx, dsn: dsn, admin: admin}
}

func TestPostgresSecurityFixtureUsesExactMigrationAndGrantOrder(t *testing.T) {
	fixture := newCheckpointSecurityFixture(t)
	ctx, dsn, admin := fixture.ctx, fixture.dsn, fixture.admin
	if err := validateCheckpointCatalog(ctx, admin, checkpointMigrationOwner, checkpointRuntimeRole, false); err != nil {
		t.Fatalf("catalog preflight before grants: %v", err)
	}

	runtimeDB := openRolePool(ctx, t, dsn, checkpointRuntimeRole, checkpointRuntimePass)
	deniedWriter := newPostgresWriter(t, runtimeDB, "least-privilege", nil)
	if _, _, err := deniedWriter.Load(ctx, "job"); err == nil {
		t.Fatal("runtime Load succeeded before grants")
	}
	if revision, err := deniedWriter.Commit(ctx, "job", 0, nil, "checkpoint"); err == nil || revision != 0 {
		t.Fatalf("runtime Commit before grants = %d, %v; want denied", revision, err)
	}

	grants := []string{
		`grant usage on schema public to sqlcheckpoint_runtime`,
		`grant select,insert,update on public.bluetape_batch_checkpoints to sqlcheckpoint_runtime`,
	}
	for _, statement := range grants {
		if _, err := admin.ExecContext(ctx, statement); err != nil {
			t.Fatalf("runtime grant %q: %v", statement, err)
		}
	}
	if err := validateCheckpointCatalog(ctx, admin, checkpointMigrationOwner, checkpointRuntimeRole, true); err != nil {
		t.Fatalf("catalog preflight after grants: %v", err)
	}

	writer := newPostgresWriter(t, runtimeDB, "least-privilege", nil)
	if revision, err := writer.Commit(ctx, "job", 0, nil, "one"); err != nil || revision != 1 {
		t.Fatalf("runtime INSERT = %d, %v", revision, err)
	}
	if revision, err := writer.Commit(ctx, "job", 1, nil, "two"); err != nil || revision != 2 {
		t.Fatalf("runtime UPDATE = %d, %v", revision, err)
	}
	if checkpoint, found, err := writer.Load(ctx, "job"); err != nil || !found || checkpoint.Version != 2 {
		t.Fatalf("runtime SELECT = %+v, %v, %v", checkpoint, found, err)
	}

	for _, statement := range []string{
		`create table public.sqlcheckpoint_forbidden(id integer)`,
		`delete from public.bluetape_batch_checkpoints`,
		`truncate table public.bluetape_batch_checkpoints`,
		`alter table public.bluetape_batch_checkpoints add column forbidden integer`,
		`drop table public.bluetape_batch_checkpoints`,
	} {
		if _, err := runtimeDB.ExecContext(ctx, statement); err == nil {
			t.Fatalf("runtime unexpectedly executed %q", statement)
		}
	}
	var forbiddenReferences, forbiddenTrigger bool
	if err := admin.QueryRowContext(ctx, `select
		has_table_privilege($1,'public.bluetape_batch_checkpoints','references'),
		has_table_privilege($1,'public.bluetape_batch_checkpoints','trigger')`, checkpointRuntimeRole).
		Scan(&forbiddenReferences, &forbiddenTrigger); err != nil {
		t.Fatal(err)
	}
	if forbiddenReferences || forbiddenTrigger {
		t.Fatalf("runtime dangerous privileges references=%v trigger=%v", forbiddenReferences, forbiddenTrigger)
	}
	// PostgreSQL can report GRANT success with a warning when no grant option is
	// held; the effective privilege check is the authoritative assertion.
	_, _ = runtimeDB.ExecContext(ctx,
		`grant select on public.bluetape_batch_checkpoints to sqlcheckpoint_forbidden_grantee`)
	var granteeSelect bool
	if err := admin.QueryRowContext(ctx,
		`select has_table_privilege('sqlcheckpoint_forbidden_grantee','public.bluetape_batch_checkpoints','select')`).
		Scan(&granteeSelect); err != nil {
		t.Fatal(err)
	}
	if granteeSelect {
		t.Fatal("runtime delegated checkpoint privileges")
	}
}

func TestPostgresCatalogValidatorRejectsOnePropertyHostileDrift(t *testing.T) {
	fixture := newCheckpointSecurityFixture(t)
	owner := checkpointMigrationOwner
	if err := validateCheckpointCatalog(fixture.ctx, fixture.admin, owner, checkpointRuntimeRole, false); err != nil {
		t.Fatalf("exact catalog rejected: %v", err)
	}

	reorderedSchema := `create table public.bluetape_batch_checkpoints (
		checkpoint_key bytea not null constraint bluetape_batch_checkpoints_key_size_check
			check (pg_catalog.octet_length(checkpoint_key) between 1 and 1024),
		namespace bytea not null constraint bluetape_batch_checkpoints_namespace_size_check
			check (pg_catalog.octet_length(namespace) between 1 and 128),
		revision bigint not null constraint bluetape_batch_checkpoints_revision_check check (revision > 0),
		payload bytea not null constraint bluetape_batch_checkpoints_payload_size_check
			check (pg_catalog.octet_length(payload) <= 16777216),
		updated_at timestamptz not null,
		constraint bluetape_batch_checkpoints_pkey primary key (namespace, checkpoint_key)
	)`
	tests := []struct {
		name             string
		statements       []string
		expectedMismatch string
	}{
		{name: "column-order", statements: []string{`drop table public.bluetape_batch_checkpoints`, reorderedSchema}},
		{name: "column-type", statements: []string{`alter table public.bluetape_batch_checkpoints alter column revision type numeric`}},
		{name: "column-nullability", statements: []string{`alter table public.bluetape_batch_checkpoints alter column updated_at drop not null`}},
		{name: "column-privilege", statements: []string{`grant select(namespace) on public.bluetape_batch_checkpoints to public`}},
		{name: "primary-key-order", statements: []string{
			`alter table public.bluetape_batch_checkpoints drop constraint bluetape_batch_checkpoints_pkey`,
			`alter table public.bluetape_batch_checkpoints add constraint bluetape_batch_checkpoints_pkey primary key(checkpoint_key,namespace)`,
		}},
		{name: "constraint-name", statements: []string{
			`alter table public.bluetape_batch_checkpoints drop constraint bluetape_batch_checkpoints_revision_check`,
			`alter table public.bluetape_batch_checkpoints add constraint hostile_revision_check check (revision > 0)`,
		}},
		{name: "constraint-definition", statements: []string{
			`alter table public.bluetape_batch_checkpoints drop constraint bluetape_batch_checkpoints_revision_check`,
			`alter table public.bluetape_batch_checkpoints add constraint bluetape_batch_checkpoints_revision_check check (revision >= 0)`,
		}},
		{name: "constraint-not-validated", statements: []string{
			`alter table public.bluetape_batch_checkpoints drop constraint bluetape_batch_checkpoints_revision_check`,
			`alter table public.bluetape_batch_checkpoints add constraint bluetape_batch_checkpoints_revision_check check (revision > 0) not valid`,
		}},
		{name: "unlogged", statements: []string{`alter table public.bluetape_batch_checkpoints set unlogged`}},
		{name: "temporary", statements: []string{
			`drop table public.bluetape_batch_checkpoints`,
			`create temporary table bluetape_batch_checkpoints(namespace bytea)`,
		}},
		{name: "owner", statements: []string{`create role sqlcheckpoint_hostile_owner nologin`, `alter table public.bluetape_batch_checkpoints owner to sqlcheckpoint_hostile_owner`}},
		{name: "owner-login", statements: []string{`alter role ` + owner + ` login`}},
		{name: "schema-owner", statements: []string{`create role sqlcheckpoint_hostile_schema_owner nologin`, `alter schema public owner to sqlcheckpoint_hostile_schema_owner`}},
		{name: "unrelated-schema-grantee", statements: []string{`create role sqlcheckpoint_unrelated_schema nologin`, `grant usage on schema public to sqlcheckpoint_unrelated_schema`}},
		{name: "unrelated-table-grantee", statements: []string{`create role sqlcheckpoint_unrelated_table nologin`, `grant select on public.bluetape_batch_checkpoints to sqlcheckpoint_unrelated_table`}},
		{name: "runtime-membership", statements: []string{`grant sqlcheckpoint_migration_owner to sqlcheckpoint_runtime`}},
		{name: "runtime-schema-create", statements: []string{`grant create on schema public to sqlcheckpoint_runtime`}},
		{name: "runtime-delete", statements: []string{`grant delete on public.bluetape_batch_checkpoints to sqlcheckpoint_runtime`}},
		{name: "runtime-grant-option", statements: []string{`grant select on public.bluetape_batch_checkpoints to sqlcheckpoint_runtime with grant option`}},
		{name: "row-level-security", statements: []string{`alter table public.bluetape_batch_checkpoints enable row level security`}},
		{name: "forced-row-level-security", statements: []string{
			`alter table public.bluetape_batch_checkpoints enable row level security`,
			`alter table public.bluetape_batch_checkpoints force row level security`,
		}},
		{name: "policy", statements: []string{`create policy sqlcheckpoint_hostile_policy on public.bluetape_batch_checkpoints using (false)`}},
		{name: "trigger", statements: []string{
			`create function public.sqlcheckpoint_hostile_trigger() returns trigger language plpgsql as 'begin return new; end'`,
			`create trigger sqlcheckpoint_hostile_trigger before insert on public.bluetape_batch_checkpoints for each row execute function public.sqlcheckpoint_hostile_trigger()`,
		}},
		{name: "rewrite-rule", statements: []string{`create rule sqlcheckpoint_hostile_rule as on insert to public.bluetape_batch_checkpoints do instead nothing`}},
		{name: "public-schema-create", statements: []string{`grant create on schema public to public`}},
		{name: "public-table-privilege", statements: []string{`grant select on public.bluetape_batch_checkpoints to public`}},
		{name: "oversized-namespace-row", expectedMismatch: "stored row bounds", statements: []string{
			`alter table public.bluetape_batch_checkpoints drop constraint bluetape_batch_checkpoints_namespace_size_check`,
			`insert into public.bluetape_batch_checkpoints(namespace,checkpoint_key,revision,payload,updated_at)
			 values (pg_catalog.convert_to(pg_catalog.repeat('n',129),'UTF8'),'key'::bytea,1,''::bytea,pg_catalog.now())`,
		}},
		{name: "oversized-key-row", expectedMismatch: "stored row bounds", statements: []string{
			`alter table public.bluetape_batch_checkpoints drop constraint bluetape_batch_checkpoints_key_size_check`,
			`insert into public.bluetape_batch_checkpoints(namespace,checkpoint_key,revision,payload,updated_at)
			 values ('namespace'::bytea,pg_catalog.convert_to(pg_catalog.repeat('k',1025),'UTF8'),1,''::bytea,pg_catalog.now())`,
		}},
		{name: "invalid-revision-row", expectedMismatch: "stored row bounds", statements: []string{
			`alter table public.bluetape_batch_checkpoints drop constraint bluetape_batch_checkpoints_revision_check`,
			`insert into public.bluetape_batch_checkpoints(namespace,checkpoint_key,revision,payload,updated_at)
			 values ('namespace'::bytea,'key'::bytea,0,''::bytea,pg_catalog.now())`,
		}},
		{name: "oversized-payload-row", expectedMismatch: "stored row bounds", statements: []string{
			`alter table public.bluetape_batch_checkpoints drop constraint bluetape_batch_checkpoints_payload_size_check`,
			`insert into public.bluetape_batch_checkpoints(namespace,checkpoint_key,revision,payload,updated_at)
			 values ('namespace'::bytea,'key'::bytea,1,pg_catalog.convert_to(pg_catalog.repeat('p',16777217),'UTF8'),pg_catalog.now())`,
		}},
		{name: "relation-kind", statements: []string{
			`drop table public.bluetape_batch_checkpoints`,
			`create view public.bluetape_batch_checkpoints as select ''::bytea namespace,''::bytea checkpoint_key,1::bigint revision,''::bytea payload,now() updated_at`,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := fixture.admin.BeginTx(fixture.ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			for _, statement := range tt.statements {
				if _, err := tx.ExecContext(fixture.ctx, statement); err != nil {
					t.Fatalf("hostile mutation %q: %v", statement, err)
				}
			}
			validationErr := validateCheckpointCatalog(fixture.ctx, tx, owner, checkpointRuntimeRole, false)
			if validationErr == nil {
				t.Fatal("hostile drift passed catalog preflight")
			}
			if tt.expectedMismatch != "" && !strings.Contains(validationErr.Error(), tt.expectedMismatch) {
				t.Fatalf("hostile drift mismatch = %v; want %q before other catalog checks", validationErr, tt.expectedMismatch)
			}
		})
	}
}

func validateCheckpointCatalog(
	ctx context.Context,
	db checkpointCatalogQuerier,
	expectedOwner string,
	runtimeRole string,
	expectRuntimeGrants bool,
) error {
	var owner, relkind, persistence string
	var ownerLogin, rls, forceRLS bool
	var policies, triggers, rules int
	err := db.QueryRowContext(ctx, `select owner.rolname,owner.rolcanlogin,c.relkind::text,c.relpersistence::text,
		c.relrowsecurity,c.relforcerowsecurity,
		(select count(*) from pg_catalog.pg_policy p where p.polrelid=c.oid),
		(select count(*) from pg_catalog.pg_trigger t where t.tgrelid=c.oid and not t.tgisinternal),
		(select count(*) from pg_catalog.pg_rewrite r where r.ev_class=c.oid and r.rulename <> '_RETURN')
	from pg_catalog.pg_class c
	join pg_catalog.pg_namespace n on n.oid=c.relnamespace
	join pg_catalog.pg_roles owner on owner.oid=c.relowner
	where n.nspname='public' and c.relname='bluetape_batch_checkpoints'`).Scan(
		&owner, &ownerLogin, &relkind, &persistence, &rls, &forceRLS, &policies, &triggers, &rules)
	if err != nil {
		return checkpointCatalogMismatch("relation: %v", err)
	}
	if owner != expectedOwner || ownerLogin || relkind != "r" || persistence != "p" ||
		rls || forceRLS || policies != 0 || triggers != 0 || rules != 0 {
		return checkpointCatalogMismatch(
			"relation owner=%q want=%q owner_login=%v kind=%q persistence=%q rls=%v force=%v policies=%d triggers=%d rules=%d",
			owner, expectedOwner, ownerLogin, relkind, persistence, rls, forceRLS, policies, triggers, rules)
	}
	var schemaOwner string
	if err := db.QueryRowContext(ctx, `select owner.rolname
		from pg_catalog.pg_namespace n
		join pg_catalog.pg_roles owner on owner.oid=n.nspowner
		where n.nspname='public'`).Scan(&schemaOwner); err != nil {
		return checkpointCatalogMismatch("namespace owner: %v", err)
	}
	if schemaOwner != expectedOwner {
		return checkpointCatalogMismatch("namespace owner=%q want=%q", schemaOwner, expectedOwner)
	}

	wantColumns := []checkpointCatalogColumn{
		{name: "namespace", typeName: "bytea", notNull: true},
		{name: "checkpoint_key", typeName: "bytea", notNull: true},
		{name: "revision", typeName: "bigint", notNull: true},
		{name: "payload", typeName: "bytea", notNull: true},
		{name: "updated_at", typeName: "timestamp with time zone", notNull: true},
	}
	rows, err := db.QueryContext(ctx, `select a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),a.attnotnull,
		coalesce(pg_catalog.array_length(a.attacl,1),0)
	from pg_catalog.pg_attribute a
	where a.attrelid='public.bluetape_batch_checkpoints'::regclass
	  and a.attnum>0 and not a.attisdropped
	order by a.attnum`)
	if err != nil {
		return checkpointCatalogMismatch("columns: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var columns []checkpointCatalogColumn
	for rows.Next() {
		var column checkpointCatalogColumn
		if err := rows.Scan(&column.name, &column.typeName, &column.notNull, &column.columnACLs); err != nil {
			return checkpointCatalogMismatch("column scan: %v", err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return checkpointCatalogMismatch("column rows: %v", err)
	}
	if len(columns) != len(wantColumns) {
		return checkpointCatalogMismatch("columns=%v want=%v", columns, wantColumns)
	}
	for index := range wantColumns {
		if columns[index] != wantColumns[index] {
			return checkpointCatalogMismatch("column[%d]=%v want=%v", index, columns[index], wantColumns[index])
		}
	}

	var invalidStoredRows int
	if err := db.QueryRowContext(ctx, `select count(*)
		from public.bluetape_batch_checkpoints
		where namespace is null or pg_catalog.octet_length(namespace) not between 1 and 128
		   or checkpoint_key is null or pg_catalog.octet_length(checkpoint_key) not between 1 and 1024
		   or revision is null or revision <= 0
		   or payload is null or pg_catalog.octet_length(payload) > 16777216
		   or updated_at is null`).Scan(&invalidStoredRows); err != nil {
		return checkpointCatalogMismatch("stored row bounds: %v", err)
	}
	if invalidStoredRows != 0 {
		return checkpointCatalogMismatch("stored row bounds: invalid_rows=%d", invalidStoredRows)
	}

	wantConstraints := map[string]string{
		"bluetape_batch_checkpoints_pkey":                 "primarykey(namespace,checkpoint_key)",
		"bluetape_batch_checkpoints_namespace_size_check": "check(((octet_length(namespace)>=1)and(octet_length(namespace)<=128)))",
		"bluetape_batch_checkpoints_key_size_check":       "check(((octet_length(checkpoint_key)>=1)and(octet_length(checkpoint_key)<=1024)))",
		"bluetape_batch_checkpoints_revision_check":       "check((revision>0))",
		"bluetape_batch_checkpoints_payload_size_check":   "check((octet_length(payload)<=16777216))",
	}
	rows, err = db.QueryContext(ctx, `select con.conname,con.convalidated,con.condeferrable,con.condeferred,
		pg_catalog.pg_get_constraintdef(con.oid,false)
	from pg_catalog.pg_constraint con
	where con.conrelid='public.bluetape_batch_checkpoints'::regclass
	order by con.conname`)
	if err != nil {
		return checkpointCatalogMismatch("constraints: %v", err)
	}
	defer func() { _ = rows.Close() }()
	gotConstraints := make(map[string]string)
	for rows.Next() {
		var name, definition string
		var validated, deferrable, deferred bool
		if err := rows.Scan(&name, &validated, &deferrable, &deferred, &definition); err != nil {
			return checkpointCatalogMismatch("constraint scan: %v", err)
		}
		if !validated || deferrable || deferred {
			return checkpointCatalogMismatch("constraint %q state validated=%v deferrable=%v deferred=%v", name, validated, deferrable, deferred)
		}
		gotConstraints[name] = canonicalCheckpointCatalogDefinition(definition)
	}
	if err := rows.Err(); err != nil {
		return checkpointCatalogMismatch("constraint rows: %v", err)
	}
	if len(gotConstraints) != len(wantConstraints) {
		return checkpointCatalogMismatch("constraints=%v want=%v", gotConstraints, wantConstraints)
	}
	for name, definition := range wantConstraints {
		if gotConstraints[name] != definition {
			return checkpointCatalogMismatch("constraint %q=%q want=%q", name, gotConstraints[name], definition)
		}
	}

	var valid, ready, primary, unique bool
	var keyColumns, allColumns int
	var indexColumns, predicate, expressions string
	err = db.QueryRowContext(ctx, `select i.indisvalid,i.indisready,i.indisprimary,i.indisunique,
		i.indnkeyatts,i.indnatts,
		coalesce((select string_agg(a.attname,',' order by key.ordinality)
		 from unnest(i.indkey) with ordinality as key(attnum,ordinality)
		 left join pg_catalog.pg_attribute a on a.attrelid=i.indrelid and a.attnum=key.attnum),''),
		coalesce(pg_catalog.pg_get_expr(i.indpred,i.indrelid),''),
		coalesce(pg_catalog.pg_get_expr(i.indexprs,i.indrelid),'')
	from pg_catalog.pg_index i
	join pg_catalog.pg_class c on c.oid=i.indrelid
	join pg_catalog.pg_namespace n on n.oid=c.relnamespace
	where n.nspname='public' and c.relname='bluetape_batch_checkpoints' and i.indisprimary`).Scan(
		&valid, &ready, &primary, &unique, &keyColumns, &allColumns, &indexColumns, &predicate, &expressions)
	if err != nil || !valid || !ready || !primary || !unique || keyColumns != 2 || allColumns != 2 ||
		indexColumns != "namespace,checkpoint_key" || predicate != "" || expressions != "" {
		return checkpointCatalogMismatch(
			"primary index err=%v valid=%v ready=%v primary=%v unique=%v key/all=%d/%d columns=%q predicate=%q expressions=%q",
			err, valid, ready, primary, unique, keyColumns, allColumns, indexColumns, predicate, expressions)
	}

	var publicSchemaCreate, publicTableUnsafe bool
	err = db.QueryRowContext(ctx, `select
		exists(select 1 from pg_catalog.pg_namespace n,
			lateral pg_catalog.aclexplode(coalesce(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) acl
			where n.nspname='public' and acl.grantee=0 and acl.privilege_type='CREATE'),
		exists(select 1 from pg_catalog.pg_class c
			join pg_catalog.pg_namespace n on n.oid=c.relnamespace,
			lateral pg_catalog.aclexplode(coalesce(c.relacl,pg_catalog.acldefault('r',c.relowner))) acl
			where n.nspname='public' and c.relname='bluetape_batch_checkpoints'
			and acl.grantee=0 and acl.privilege_type in ('SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER'))`).
		Scan(&publicSchemaCreate, &publicTableUnsafe)
	if err != nil || publicSchemaCreate || publicTableUnsafe {
		return checkpointCatalogMismatch("PUBLIC privileges schema_create=%v table_unsafe=%v err=%v", publicSchemaCreate, publicTableUnsafe, err)
	}

	var invalidSchemaACLs, invalidTableACLs, runtimeSchemaACLs, runtimeTableACLs int
	err = db.QueryRowContext(ctx, `select
		(select count(*) from pg_catalog.pg_namespace n,
		 lateral pg_catalog.aclexplode(coalesce(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) acl
		 where n.nspname='public' and not (
		   (acl.grantee=n.nspowner and acl.privilege_type in ('USAGE','CREATE'))
		   or (acl.grantee=0 and acl.privilege_type='USAGE' and not acl.is_grantable)
		   or (acl.grantee=(select oid from pg_catalog.pg_roles where rolname=$1)
		       and $2 and acl.privilege_type='USAGE' and not acl.is_grantable)
		 )),
		(select count(*) from pg_catalog.pg_class c
		 join pg_catalog.pg_namespace n on n.oid=c.relnamespace,
		 lateral pg_catalog.aclexplode(coalesce(c.relacl,pg_catalog.acldefault('r',c.relowner))) acl
		 where n.nspname='public' and c.relname='bluetape_batch_checkpoints' and not (
		   (acl.grantee=c.relowner and acl.privilege_type in ('SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER'))
		   or (acl.grantee=(select oid from pg_catalog.pg_roles where rolname=$1)
		       and $2 and acl.privilege_type in ('SELECT','INSERT','UPDATE') and not acl.is_grantable)
		 )),
		(select count(*) from pg_catalog.pg_namespace n,
		 lateral pg_catalog.aclexplode(coalesce(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) acl
		 where n.nspname='public'
		   and acl.grantee=(select oid from pg_catalog.pg_roles where rolname=$1)
		   and acl.privilege_type='USAGE' and not acl.is_grantable),
		(select count(*) from pg_catalog.pg_class c
		 join pg_catalog.pg_namespace n on n.oid=c.relnamespace,
		 lateral pg_catalog.aclexplode(coalesce(c.relacl,pg_catalog.acldefault('r',c.relowner))) acl
		 where n.nspname='public' and c.relname='bluetape_batch_checkpoints'
		   and acl.grantee=(select oid from pg_catalog.pg_roles where rolname=$1)
		   and acl.privilege_type in ('SELECT','INSERT','UPDATE') and not acl.is_grantable)`,
		runtimeRole, expectRuntimeGrants).Scan(
		&invalidSchemaACLs, &invalidTableACLs, &runtimeSchemaACLs, &runtimeTableACLs)
	if err != nil {
		return checkpointCatalogMismatch("ACL allowlist query: %v", err)
	}
	wantRuntimeSchemaACLs, wantRuntimeTableACLs := 0, 0
	if expectRuntimeGrants {
		wantRuntimeSchemaACLs, wantRuntimeTableACLs = 1, 3
	}
	if invalidSchemaACLs != 0 || invalidTableACLs != 0 ||
		runtimeSchemaACLs != wantRuntimeSchemaACLs || runtimeTableACLs != wantRuntimeTableACLs {
		return checkpointCatalogMismatch(
			"ACL allowlist invalid_schema=%d invalid_table=%d runtime_schema=%d/%d runtime_table=%d/%d",
			invalidSchemaACLs, invalidTableACLs, runtimeSchemaACLs, wantRuntimeSchemaACLs,
			runtimeTableACLs, wantRuntimeTableACLs)
	}

	if runtimeRole != "" {
		var usage, create, selectPrivilege, insertPrivilege, updatePrivilege bool
		var deletePrivilege, truncatePrivilege, referencesPrivilege, triggerPrivilege bool
		var member, anyMembership, ownerRole, roleInherits, grantOption bool
		err = db.QueryRowContext(ctx, `select
			has_schema_privilege($1,'public','usage'),has_schema_privilege($1,'public','create'),
			has_table_privilege($1,'public.bluetape_batch_checkpoints','select'),
			has_table_privilege($1,'public.bluetape_batch_checkpoints','insert'),
			has_table_privilege($1,'public.bluetape_batch_checkpoints','update'),
			has_table_privilege($1,'public.bluetape_batch_checkpoints','delete'),
			has_table_privilege($1,'public.bluetape_batch_checkpoints','truncate'),
			has_table_privilege($1,'public.bluetape_batch_checkpoints','references'),
			has_table_privilege($1,'public.bluetape_batch_checkpoints','trigger'),
			pg_has_role($1,$2,'member'),
			exists(select 1 from pg_catalog.pg_auth_members m
				where m.member=(select oid from pg_catalog.pg_roles where rolname=$1)),
			(select c.relowner=(select oid from pg_catalog.pg_roles where rolname=$1)
			 from pg_catalog.pg_class c where c.oid='public.bluetape_batch_checkpoints'::regclass),
			(select rolinherit from pg_catalog.pg_roles where rolname=$1),
			exists(select 1 from pg_catalog.pg_class c
				join pg_catalog.pg_namespace n on n.oid=c.relnamespace,
				lateral pg_catalog.aclexplode(coalesce(c.relacl,pg_catalog.acldefault('r',c.relowner))) acl
				where n.nspname='public' and c.relname='bluetape_batch_checkpoints'
				and acl.grantee=(select oid from pg_catalog.pg_roles where rolname=$1)
				and acl.is_grantable)`, runtimeRole, expectedOwner).Scan(
			&usage, &create, &selectPrivilege, &insertPrivilege, &updatePrivilege,
			&deletePrivilege, &truncatePrivilege, &referencesPrivilege, &triggerPrivilege,
			&member, &anyMembership, &ownerRole, &roleInherits, &grantOption)
		if err != nil {
			return checkpointCatalogMismatch("runtime privileges: %v", err)
		}
		// PostgreSQL's default public schema ACL gives PUBLIC USAGE; only CREATE is
		// unsafe and revoked before migration. The explicit runtime USAGE grant is
		// still applied with the table grants for an auditable role contract.
		if !usage || selectPrivilege != expectRuntimeGrants ||
			insertPrivilege != expectRuntimeGrants || updatePrivilege != expectRuntimeGrants ||
			create || deletePrivilege || truncatePrivilege || referencesPrivilege || triggerPrivilege ||
			member || anyMembership || ownerRole || roleInherits || grantOption {
			return checkpointCatalogMismatch(
				"runtime usage=%v create=%v select=%v insert=%v update=%v delete=%v truncate=%v references=%v trigger=%v member=%v any_membership=%v owner=%v inherit=%v grant_option=%v",
				usage, create, selectPrivilege, insertPrivilege, updatePrivilege, deletePrivilege,
				truncatePrivilege, referencesPrivilege, triggerPrivilege, member, anyMembership, ownerRole, roleInherits, grantOption)
		}
	}
	return nil
}

func checkpointCatalogMismatch(format string, args ...any) error {
	return fmt.Errorf("checkpoint catalog mismatch: "+format, args...)
}

func canonicalCheckpointCatalogDefinition(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(value, "pg_catalog.", ""))), "")
}

func openRolePool(ctx context.Context, t *testing.T, dsn, username, password string) *sql.DB {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword(username, password)
	db, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	waitPostgresReady(ctx, t, db, "runtime PostgreSQL pool")
	return db
}
