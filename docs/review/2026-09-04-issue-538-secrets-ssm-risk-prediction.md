# Issue #538 Secrets Manager 및 SSM 위험 예측

## 범위와 근거

- 대상: 새 `secretsmanager`, `ssm` package와 AWS SDK v2 service modules
- 근거: live issue [#538](https://github.com/bluetape4k/bluetape-go/issues/538), parent [#517](https://github.com/bluetape4k/bluetape-go/issues/517), AWS SDK generated API, existing `cache.LoadingCache`/`cache.Memory`
- 제외: live AWS, credential chain, provisioning, generic config framework, KMS envelope

| 위험 | 영향 | 완화 및 증거 |
|---|---|---|
| raw secret이 formatter/error/cache 경계에서 노출 | 높음 | unexported `Value` bytes, `String`/`GoString` redaction, error `%+v` test |
| SecretString/SecretBinary 또는 SSM nil output을 성공 처리 | 높음 | exact-one/missing-value table tests, zero provider call on invalid input |
| response 직후 cancellation이 성공을 가림 | 높음 | SDK response 후 `ctx.Err()` 우선 검사와 blocking fake |
| error/cancellation/stale 값이 TTL cache에 저장 | 높음 | `GetOrLoad` success-only contract, expiry/error/no-stale tests |
| shared cache race 또는 caller buffer alias | 높음 | value copy, mutex-safe existing cache, normal/race concurrent tests |
| decryption mode cache collision | 중간 | mode-prefixed SSM cache key와 captured request tests |
| caller 설정 precedence/credential lifecycle 오해 | 중간 | EN/KO README에 소유권과 비목표 명시 |
| SDK module drift | 중간 | direct version pin, concrete compile assertion, tidy/vet |

## Stop conditions

P0/P1 secret exposure, cancellation masking, cache stale/error hit, data race,
or unbounded input must block PR progression. live emulator가 지원되지 않으면
fake-first evidence를 유지하고 이를 live smoke PASS로 표현하지 않는다.

## SPW status

- SPW-01: PASS — source, audience, identifiers, unresolved live behavior recorded.
- SPW-02: PASS — risk/mitigation and stop conditions map to plan tasks.
- SPW-03: PASS — Korean technical register and API-token preservation reviewed.
- SPW-04: PASS — each predicted risk has a test or documentation evidence path.
- SPW-05: PASS — final implementation read-back과 fresh test/static evidence를
  `2026-09-04-issue-538-secrets-ssm-implementation-review.md`에 기록했다.
