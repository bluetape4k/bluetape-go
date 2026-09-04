# Issue #545 Step 3-R 구현 계획 검토

- 이슈: [#545](https://github.com/bluetape4k/bluetape-go/issues/545)
- Epic: [#540](https://github.com/bluetape4k/bluetape-go/issues/540)
- 계획: `docs/superpowers/plans/2026-08-16-issue-545-jwks-provider-plan.md`
- 설계: `docs/superpowers/specs/2026-08-16-issue-545-jwks-provider-design.md`
- 범위: `jwt/jwks` optional JWKS provider, RSA/ECDSA/EdDSA public key only
- 최종 판정: **PASS — P0=0, P1=0**

## 독립 검토 결과

| 관점 | 초기 지적 | main 통합 후 | 반영 근거 |
| --- | --- | --- | --- |
| Performance | P0=0, P1=0, P2=0 | P0=0, P1=0 | lock 밖 I/O, immutable publication, cooldown/flight, benchmark raw evidence |
| Stability | P0=0, P1=2, P2=2 | P0=0, P1=0 | caller cancellation cooldown 제외, flight identity/generation, late leader 무시, 전원 취소·warm hit 회귀 |
| Security | P0=0, P1=2, P2=5; 후속 P1=1 | P0=0, P1=0 | HTTPS 기본, loopback HTTP 예외, private/link-local/metadata dial 차단, RSA exponent, kid/key_ops/Ed25519/body/key-count 경계 |
| Operator/Ops | P0=0, P1=0, P2=1 | P0=0, P1=0 | owner별 runbook, preflight/clear/rollback/overlap, low-cardinality event, release handoff |
| Developer/API | P0=0, P1=0, P2=1 | P0=0, P1=0 | nil KeyFunc construction RED, external `package jwks_test`, typed errors, Algorithm 변환 |
| User/Caller | P0=0, P1=0, P2=0, P3=1 | P0=0, P1=0 | claims boundary, six JOSE header rejection, direct URL/OIDC/JWE 경계, RS/PS 변환과 ES/EdDSA 상수 안내 |

모든 lane은 read-only였고 구현 파일을 변경하거나 heavy test를 실행하지 않았다.
계획 수정 후 각 지적을 main이 read-back하여 P0/P1을 0으로 만들었다. 별도
다이어그램은 새 사용자-facing 관계가 없어 N/A다.

## Main integration self-review

- **SPW-01 범위:** root `jwt` API를 변경하지 않고 `jwt/jwks`에만 의존성과
  provider를 둔다. JWE/OIDC discovery/global cache/background refresh는
  명시적으로 제외한다.
- **SPW-02 보안:** 기본 HTTPS, loopback 테스트 예외, 기본 dial의 private/
  link-local/metadata 차단, asymmetric allowlist, RSA exponent, raw Ed25519
  길이, `key_ops=verify`, bounded `kid`, redacted error를 계획에 고정했다.
- **SPW-03 안정성:** HTTP/read/decode는 lock 밖에서 수행하고 immutable
  publication, generation, flight identity, cancellation/takeover 및 stale
  fail-closed를 named test와 race gate로 검증한다.
- **SPW-04 운영:** startup readiness `Refresh`, warning/page 기준, owner별
  preflight/rollback/overlap runbook과 release promotion N/A handoff를 둔다.
- **SPW-05 검증:** TDD RED/GREEN/REFACTOR 순서, package/race/full CI,
  benchmark 환경·raw output·dependency/license evidence, final six-lane
  review와 lesson artifact를 계획했다.

## 계획 품질·검증

- exact file path, RED/GREEN/REFACTOR, rollback/re-execution과 예상 결과가
  계획에 있다.
- `git diff --check`: **PASS**
- mutation-check 대상: 이 review artifact의 단일 경로, **PASS**
- Step 3-R completion gate: **P0=0, P1=0**, 구현 진입 가능

## 남은 검증 공백

아직 구현 전이므로 실제 Go 테스트, race, benchmark, dependency resolution,
SSRF dial, rollback drill, CI 결과는 Step 6/7 구현·검증 단계에서 fresh하게
수집해야 한다. 이 공백은 계획 단계의 정상 상태이며 구현 완료를 주장하지 않는다.
