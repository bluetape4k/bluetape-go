# Issue #539 RDS IAM auth 위험 예측

## 범위와 근거

- 대상: 새 `rds/auth` package와 `feature/rds/auth` SDK module
- 근거: live issue [#539](https://github.com/bluetape4k/bluetape-go/issues/539), parent [#517](https://github.com/bluetape4k/bluetape-go/issues/517), AWS SDK generated API
- 제외: live AWS/RDS, DB driver/pool, credential chain, IAM provisioning, token refresh

| 위험 | 영향 | 완화 및 증거 |
|---|---|---|
| endpoint를 scheme/path 포함 URL로 전달 | 높음 | `SplitHostPort` 기반 strict validation table |
| IPv6/port 경계 오판 | 중간 | bracketed IPv6, port 1/65535 및 invalid port tests |
| raw token/credential/provider error가 노출 | 높음 | unexported token bytes, formatter/error redaction tests |
| SDK empty token 또는 output anomaly를 성공 처리 | 높음 | response non-empty validation과 typed sentinel |
| cancellation이 늦은 token 성공을 가림 | 높음 | credential fake blocking/response-after-cancel test |
| helper가 token refresh/DB lifecycle을 암묵적으로 소유 | 높음 | no cache/goroutine/driver API와 EN/KO ownership docs |
| SDK feature module drift | 중간 | `feature/rds/auth v1.7.1` direct pin, compile test, tidy/vet |

## Stop conditions

token/credential exposure, malformed endpoint acceptance, cancellation masking,
or implicit DB/refresh ownership is P0/P1 and blocks PR progression. Live AWS
검증이 없으면 fake-first/compile-check evidence만 사용한다.

## SPW status

- SPW-01: PASS — source, audience, identifiers, unresolved live behavior recorded.
- SPW-02: PASS — risks map to plan tasks and stop conditions.
- SPW-03: PASS — Korean technical register and exact API-token preservation reviewed.
- SPW-04: PASS — each risk has a concrete test or documentation evidence path.
- SPW-05: PASS — final implementation read-back과 fresh test/static evidence를
  `2026-09-04-issue-539-rds-iam-auth-implementation-review.md`에 기록했다.
