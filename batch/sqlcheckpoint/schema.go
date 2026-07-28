package sqlcheckpoint

// SchemaSQL 상수 공개 값이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 호출자는 이 식별자를 오류, 상태, 이벤트, 옵션, 또는 기본값 계약을 비교할 때 사용한다.
const SchemaSQL = `create table if not exists public.bluetape_batch_checkpoints (
    namespace bytea not null constraint bluetape_batch_checkpoints_namespace_size_check
        check (pg_catalog.octet_length(namespace) between 1 and 128),
    checkpoint_key bytea not null constraint bluetape_batch_checkpoints_key_size_check
        check (pg_catalog.octet_length(checkpoint_key) between 1 and 1024),
    revision bigint not null constraint bluetape_batch_checkpoints_revision_check
        check (revision > 0),
    payload bytea not null constraint bluetape_batch_checkpoints_payload_size_check
        check (pg_catalog.octet_length(payload) <= 16777216),
    updated_at timestamptz not null,
    constraint bluetape_batch_checkpoints_pkey primary key (namespace, checkpoint_key)
)`
