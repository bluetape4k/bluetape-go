# Issue #592 Probabilistic Redis Shared Key Builder Test Spec

## Target

`probabilistic/redis` must delegate structural key construction to the shared
`redis.KeyBuilder` without changing any stored-key byte, namespace validation,
redacted key-ID, or public error behavior.

## Regression Cases

| Case | Setup | Required Assertions |
|---|---|---|
| Bloom key parity | Build keys for `tenant-a:emails` | Exact slot, bits, and config values retain prefix, braces, namespace bytes, and suffixes. |
| HyperLogLog key parity | Build key for `tenant-a:emails` | Exact HLL value retains the existing prefix and hash tag. |
| Namespace validation parity | Empty, whitespace, brace, and sensitive marker namespaces | Existing local validation rejects each input; no shared `redis` key-validation text/type escapes. |
| Redaction parity | Marker namespace and its built key | ID is `redis-key:` plus 12 lowercase hexadecimal characters; neither namespace nor full key appears. |
| Existing provider error contract | Existing invalid HLL payload / provider failures | `errors.As` still produces `RedisError`; key material remains redacted. |
| Existing metadata contract | Existing Bloom mismatch/corruption tests | `ErrConfigMismatch` and `ErrConfigCorrupt` mapping remains unchanged. |
| Behavior parity | Existing Bloom/HLL Testcontainers tests | Scripts, commands, persistence, and operations retain behavior. |

## Execution

```bash
go test -p 1 -count=1 ./probabilistic/redis ./redis
go test -p 1 -race -count=1 ./probabilistic/redis
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci
```

The provider package uses Testcontainers, so package and race commands run
serially. No benchmark command belongs to this contract: no algorithm, Redis
command, command count, or performance claim changes. Issue #560 remains the
owner of the cross-provider benchmark table, chart, and written analysis.
