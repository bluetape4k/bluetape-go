# Issue #592 Probabilistic Redis Shared Key Builder Test Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

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
