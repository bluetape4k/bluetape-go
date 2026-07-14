package sqlcheckpoint

const insertCheckpointSQL = `insert into public.bluetape_batch_checkpoints
(namespace, checkpoint_key, revision, payload, updated_at)
values ($1::bytea, $2::bytea, 1, $3::bytea, pg_catalog.clock_timestamp())
on conflict (namespace, checkpoint_key) do nothing
returning revision`

const updateCheckpointSQL = `update public.bluetape_batch_checkpoints set
revision = revision + 1, payload = $3::bytea, updated_at = pg_catalog.clock_timestamp()
where namespace = $1::bytea and checkpoint_key = $2::bytea and revision = $4::bigint
returning revision`

const (
	savepointSQL           = `savepoint bluetape_sqlcheckpoint_guard`
	releaseSavepointSQL    = `release savepoint bluetape_sqlcheckpoint_guard`
	rollbackToSavepointSQL = `rollback to savepoint bluetape_sqlcheckpoint_guard`
)
