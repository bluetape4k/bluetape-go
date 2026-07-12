package sqlleader

// SchemaSQL creates the PostgreSQL lease table when it does not exist.
//
// Callers own migration execution and must verify an existing relation before
// using it. New never executes SchemaSQL implicitly.
const SchemaSQL = `create table if not exists public.bluetape_leader_leases (
    leader_key text primary key,
    group_name text not null,
    member_id text not null,
    owner_token text not null,
    lease_until timestamptz not null,
    created_at timestamptz not null,
    updated_at timestamptz not null
)`
