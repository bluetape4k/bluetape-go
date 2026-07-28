package sqlleader

// SchemaSQL은 PostgreSQL lease table이 없을 때 생성하는 SQL을 반환한다.
//
// migration 실행은 호출자가 소유하며 기존 relation을 사용하기 전에 검증해야 한다.
// New는 SchemaSQL을 암묵적으로 실행하지 않는다.
const SchemaSQL = `create table if not exists public.bluetape_leader_leases (
    leader_key text primary key,
    group_name text not null,
    member_id text not null,
    owner_token text not null,
    lease_until timestamptz not null,
    created_at timestamptz not null,
    updated_at timestamptz not null
)`
