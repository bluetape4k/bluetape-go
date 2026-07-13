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
    least(pg_catalog.ceil(greatest(0::numeric, $3::numeric - tokens_micros) * 1000000 /
        rate_micros_per_second), 9223372036854775807::numeric)::bigint,
    least(pg_catalog.ceil((burst_micros::numeric - tokens_micros) * 1000000 /
        rate_micros_per_second), 9223372036854775807::numeric)::bigint`
