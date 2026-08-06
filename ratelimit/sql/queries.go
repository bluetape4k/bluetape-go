package sqlratelimit

const allowQuery = `insert into public.bluetape_ratelimit_buckets as bucket (
    namespace, bucket_key, rate_micros_per_second, burst_micros, idle_ttl_micros,
    tokens_micros, last_allowed, updated_at, expires_at
)
select $1::bytea, $2::bytea, $5::bigint, $4::bigint, $6::bigint,
    ($4::numeric - $3::numeric), true, observed_at,
    observed_at + $6::bigint * interval '1 microsecond'
from (select pg_catalog.clock_timestamp() as observed_at) as clock
on conflict (namespace, bucket_key) do update set
    tokens_micros = case when
        least(bucket.burst_micros::numeric,
            bucket.tokens_micros + greatest(0::numeric,
                extract(epoch from (greatest(bucket.updated_at, excluded.updated_at) - bucket.updated_at))) *
                bucket.rate_micros_per_second::numeric) >= $3::numeric
        then least(bucket.burst_micros::numeric,
            bucket.tokens_micros + greatest(0::numeric,
                extract(epoch from (greatest(bucket.updated_at, excluded.updated_at) - bucket.updated_at))) *
                bucket.rate_micros_per_second::numeric) - $3::numeric
        else least(bucket.burst_micros::numeric,
            bucket.tokens_micros + greatest(0::numeric,
                extract(epoch from (greatest(bucket.updated_at, excluded.updated_at) - bucket.updated_at))) *
                bucket.rate_micros_per_second::numeric)
    end,
    last_allowed = least(bucket.burst_micros::numeric,
        bucket.tokens_micros + greatest(0::numeric,
            extract(epoch from (greatest(bucket.updated_at, excluded.updated_at) - bucket.updated_at))) *
            bucket.rate_micros_per_second::numeric) >= $3::numeric,
    updated_at = greatest(bucket.updated_at, excluded.updated_at),
    expires_at = greatest(bucket.updated_at, excluded.updated_at) +
        bucket.idle_ttl_micros * interval '1 microsecond'
where bucket.rate_micros_per_second = excluded.rate_micros_per_second
  and bucket.burst_micros = excluded.burst_micros
  and bucket.idle_ttl_micros = excluded.idle_ttl_micros
returning last_allowed,
    pg_catalog.floor(tokens_micros / 1000000)::bigint,
    case when last_allowed then 0::bigint else
        least(pg_catalog.ceil(greatest(0::numeric, $3::numeric - tokens_micros) * 1000000 /
            rate_micros_per_second), 9223372036854775807::numeric)::bigint
    end,
    least(pg_catalog.ceil((burst_micros::numeric - tokens_micros) * 1000000 /
        rate_micros_per_second), 9223372036854775807::numeric)::bigint`

const cleanupQuery = `with observed as materialized (
    select pg_catalog.clock_timestamp() as observed_at
), candidates as (
    select bucket.namespace, bucket.bucket_key
    from public.bluetape_ratelimit_buckets as bucket cross join observed
    where bucket.expires_at <= observed.observed_at
    order by bucket.expires_at
    limit $1
    for update of bucket skip locked
), deleted as (
    delete from public.bluetape_ratelimit_buckets as bucket
    using candidates
    where bucket.namespace = candidates.namespace
      and bucket.bucket_key = candidates.bucket_key
    returning 1
)
select count(*)::bigint from deleted`
