# Issue #592 Probabilistic Redis Shared Key Builder Migration Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 맥락

Issue #570 adopts the shared `redis` substrate only where its contract is
compatible with the package that already owns the behavior. The probabilistic
Redis provider builds Bloom and HyperLogLog keys locally even though their fixed
prefixes and validated namespaces conform to `redis.KeyBuilder`.

This is a construction-only migration. It must not change externally stored
keys, validation, scripts, Redis commands, metadata sentinel mapping, or the
public probabilistic Redis error type.

## Current Contract Evidence

- Bloom namespace `tenant-a:emails` derives slot
  `bluetape:probabilistic:bloom:v1:{tenant-a:emails}`, bits key with `:bits`,
  and config key with `:config`.
- HyperLogLog derives
  `bluetape:probabilistic:hll:v1:{tenant-a:emails}`.
- `validateNamespace` owns the public namespace policy, including rejecting
  empty, whitespace, braces, unsafe identifiers, and sensitive markers. A
  colon remains valid inside a namespace.
- `redactedRedisKeyID` returns the probabilistic provider's established
  `redis-key:` plus 12-hex-character identifier. `RedisError` and its
  `errors.Is`/`errors.As` behavior are public package contracts.
- Bloom Lua scripts and their `config_mismatch`/`config_corrupt` sentinel
  mapping depend on the exact existing key roles.

## Decision

Retain `validateNamespace` as the first validation boundary. After it passes,
construct the Bloom slot/bits/config keys and HyperLogLog key with a shared
`redis.KeyBuilder` initialized from the existing fixed prefixes and applied
with the namespace as its hash tag.

Keep the local `redactedRedisKeyID` and the existing `RedisError` mapping.
`redis.KeyBuilder` is used only for its byte-preserving structural-key
construction; neither shared key-error types nor the shared 24-hex redaction
identifier may escape the probabilistic package.

## Alternatives Considered

| Approach | Decision | Reason |
|---|---|---|
| Replace `RedisError` with `redis.OpError` | Reject | It changes a public error type and changes the established key-ID length. |
| Let `KeyBuilder` validate namespaces directly | Reject | Package validation owns the user-facing error behavior and sensitive-marker policy. |
| Migrate Lua scripts or metadata handling | Reject | The issue is a construction migration; script and sentinel behavior must remain fixed. |
| Use `KeyBuilder` after local validation | Accept | It removes duplicated structural key formatting while preserving stored-key and public-error contracts. |

## Invariants And Acceptance Criteria

1. Bloom keys are byte-for-byte unchanged for every valid namespace:
   `bluetape:probabilistic:bloom:v1:{<namespace>}`, followed by `:bits` and
   `:config` for the structural keys.
2. HyperLogLog key bytes remain
   `bluetape:probabilistic:hll:v1:{<namespace>}`.
3. Valid namespaces containing `:` remain accepted and retain their exact text
   inside the Redis Cluster hash tag.
4. Invalid namespace inputs fail through the existing local validation path;
   shared `redis` key-validation errors do not become caller-visible.
5. Probabilistic `redactedRedisKeyID` remains `redis-key:` plus 12 lowercase
   hexadecimal characters and does not reveal the namespace or full Redis key.
6. `RedisError`, its `errors.Is`/`errors.As` behavior, and metadata sentinel
   mapping remain unchanged.
7. Bloom scripts, HyperLogLog Redis commands, command counts, hashers,
   configuration fingerprinting, expiry behavior, and algorithms are unchanged.
8. No exported API or README behavior changes.

## 위험 And Failure Modes

| Risk | Mitigation |
|---|---|
| Shared builder changes a literal key delimiter or hash-tag position | Assert the complete expected key values for a namespace containing `:`. |
| Shared validation leaks a different error contract | Validate locally before builder construction and assert invalid inputs remain invalid through the existing package path. |
| Shared redaction ID replaces the short provider ID | Keep `redactedRedisKeyID` local and assert the exact length/format and no marker leakage. |
| Script metadata loses its expected key relationship | Retain existing Testcontainers Bloom configuration/corruption coverage and HLL operation coverage. |

## 테스트 계획

- Extend unit key tests to assert exact Bloom and HyperLogLog key bytes,
  including the `tenant-a:emails` hash-tag case.
- Assert invalid braces, whitespace, and sensitive namespace inputs still fail
  before a shared builder error can become observable.
- Assert redacted IDs retain their legacy format and do not disclose marker
  namespaces or full keys.
- Retain existing provider error, metadata sentinel, Bloom behavior, and HLL
  Testcontainers coverage.
- Run package tests serially, race coverage serially, and repository CI with
  Testcontainers reuse disabled and Ryuk enabled.

## Benchmark Decision

No benchmark is run. This migration preserves key bytes, algorithms, Redis
commands, and command counts; it makes no performance claim. Issue #560 owns
the cross-provider benchmark matrix and its mandatory result table, chart, and
written analysis. Any future performance measurement must update all three
artifacts together.

## Non-Goals

- No change to namespace policy, error model, redaction format, public API,
  scripts, metadata sentinels, hasher, TTL, Redis command, algorithm, or key
  migration/retirement behavior.
- No new dependency, benchmark conclusion, README change, or Cluster routing
  feature.
