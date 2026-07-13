package sqlratelimit

// MaxCleanupBatch is the largest number of expired rows one Cleanup call may delete.
const MaxCleanupBatch = 1000

// SchemaSQL creates the PostgreSQL rate-limit table and expiry index when absent.
// Callers own migration execution and must verify existing objects before use.
// New never executes SchemaSQL implicitly.
const SchemaSQL = `create table if not exists public.bluetape_ratelimit_buckets (
    namespace bytea not null,
    bucket_key bytea not null,
    rate_micros_per_second bigint not null check (rate_micros_per_second > 0),
    burst_micros bigint not null check (burst_micros > 0),
    idle_ttl_micros bigint not null check (idle_ttl_micros > 0),
    tokens_micros numeric(30, 6) not null
        check (tokens_micros >= 0 and tokens_micros <= burst_micros),
    last_allowed boolean not null,
    updated_at timestamptz not null,
    expires_at timestamptz not null check (expires_at >= updated_at),
    primary key (namespace, bucket_key)
);
create index if not exists bluetape_ratelimit_buckets_expires_at_idx
on public.bluetape_ratelimit_buckets (expires_at)`
