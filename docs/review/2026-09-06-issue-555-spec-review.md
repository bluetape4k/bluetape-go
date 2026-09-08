# Issue #555 설계 검토

## 검토 범위와 근거

- 대상 issue: <https://github.com/bluetape4k/bluetape-go/issues/555>
- 최초 검토 spec commit: `90f544a4a16a573e5e8878edf48ae9133d5eca6d`
- 대상 문서:
  `docs/superpowers/specs/2026-09-06-issue-555-graph-backend-conformance-design.md`
- 근거: live issue 본문, `graph`, `graph/neo4j`, `leader/leadertest`,
  `ratelimit/ratelimittest`, `internal/testcleanup`, provider benchmark lifecycle
- 명령 범위: 읽기 전용 source inspection과 Markdown 검증. Testcontainers와 구현 test는
  아직 실행하지 않았다.

## 최초 7-Tier 결과와 조치

중복 finding은 main session이 하나의 계약 결함으로 정규화했다.

| 우선순위 | Lens | 근거 | 조치 | 상태 |
|---|---|---|---|---|
| P1 | performance/security | Read/traversal 결과를 `Collect`하기 전 상한이 없었다. | `Config` result limit, provider `limit+1`, runner pre-sort fail-closed와 oversized fake test를 추가했다. | 해결 |
| P1 | performance/stability/developer | Case/cleanup/close timeout과 error/panic/join 정책이 정의되지 않았다. | Exact default/maximum, `RunWithConfig`, cancel/join-before-cleanup/close, `errors.Join`, panic recovery와 non-cooperative fail-stop을 고정했다. | 해결 |
| P1 | stability | Cancellation API에 blocking-boundary handshake가 없었다. | Runner-owned `Started` signal과 missing/duplicate/early-return test를 추가했다. | 해결 |
| P1 | stability/Ops | Readiness, digest pin, Ryuk-independent cleanup과 partial factory ownership이 없었다. | Digest-pinned image, 90초 retry, `testcleanup.Register`, `WithoutCancel` partial close와 lifecycle order를 고정했다. | 해결 |
| P1 | operator/Ops | Provider/image/version과 phase 진단 metadata가 API에 없었다. | Bounded sanitized `ProviderMetadata`와 provider/phase/duration log contract를 추가했다. | 해결 |
| P1 | security | Optional 미지원 reason이 raw log injection/secret 노출을 허용했다. | 64자 allowlist `ReasonCode`로 바꾸고 unsafe input을 factory 전에 거절한다. | 해결 |
| P1 | user/caller | Traversal의 backend-generated ID와 fixture logical key mapping이 불명확했다. | `Traverse`를 ordered vertex logical-key `[]string`으로 고정하고 edge/backend ID를 제외했다. | 해결 |
| P2 | security | Fixture 값을 query 문자열에 보간하고 caller column을 오류에 노출할 수 있었다. | Static/allowlisted query, parameter binding, fixed result column과 all-phase redaction을 명시했다. | 해결 |
| P2 | stability/developer/user | Capability key 누락, mutable map, config/metadata zero와 상한 의미가 불명확했다. | Exact known key, defensive copy, default/maximum과 factory-before-validation을 고정했다. | 해결 |
| P2 | operator/Ops | Shared runner 전환 rollback과 CI runtime evidence가 없었다. | Old/new parity 후 중복 제거, revert point, 10분 suite budget과 targeted rerun evidence를 추가했다. | 해결 |

## 영향 lane 재검토

| Lens | 최종 결과 | 증거 |
|---|---|---|
| performance | `P0=0 P1=0` | Rerun lane이 시간 한도를 넘겨 중단됐다. `lane timed out; main integration fallback performed`. Result/query 상한과 exact timeout을 main session이 read-back했다. Fake harness microbenchmark는 DB-dominated test-support API에 안정적인 의미가 없어 P2로 defer하고 기존 mapping benchmark를 보존한다. |
| stability | `P0=0 P1=0` | Join-before-cleanup/close, partial factory `WithoutCancel`, panic/error/lifecycle, handshake, readiness와 namespace 계약을 독립 재검토했다. |
| security | `P0=0 P1=0` | Result bound, reason allowlist, parameter binding, result-column allowlist, metadata와 all-phase redaction을 독립 재검토했다. |
| operator/Ops | `P0=0 P1=0` | Rerun lane이 시간 한도를 넘겨 중단됐다. `lane timed out; main integration fallback performed`. Digest/readiness, bounded container fallback, diagnostics, staged rollback, runtime/WIP/Nightly와 release authority 분리를 main session이 확인했다. |
| developer/API | `P0=0 P1=0` | Cell과 무관한 graph API의 config, handshake, logical-key traversal, capability와 lifecycle 일관성을 repaired spec에서 read-back했다. |
| user/caller | `P0=0 P1=0` | Capability, logical-key traversal, examples와 lifecycle ownership을 재검토했다. Config maximum P2는 exported maximum으로 즉시 보정했다. |
| main integration | `P0=0 P1=0` | 중복 제거, repository pattern, docs/release/evidence와 모든 P1 disposition을 확인했다. |

## P2/P3 처분

- Fake harness allocation benchmark는 provider callback/DB latency를 대표하지 않고 brittle한
  compiler-dependent gate가 되므로 defer한다. Result/query 상한과 기존 graph mapping
  benchmark가 실제 위험을 검증한다.
- Milestone open issue 0, release-preparation branch, tag와 publication은 #555 PR이 아니라
  0.22.0 release gate가 소유한다.
- 그 밖의 P2는 repaired spec에 반영했다. P3는 없다.

## Writer DoD

- `SPW-01 PASS`: 한국어 Type A spec review, live issue와 exact spec commit을 evidence로 고정했다.
- `SPW-02 PASS`: scope, severity, 위치, 조치, rerun, gap과 verdict를 포함했다.
- `SPW-03 PASS`: 식별자·명령·수치는 보존하고 한국어 기술 문장과 용어를 검토했다.
- `SPW-04 PASS`: 각 finding을 repaired spec의 API/test/acceptance/DoD에 다시 연결했다.
- `SPW-05 PASS`: Markdown 표·heading·link·최종 상태를 read-back했다.

## 판정과 남은 gate

최신 통합 결과는 `P0=0 P1=0`이다. 다만 `Config`, `ProviderMetadata`, `Started`, result limit,
`ReasonCode`와 logical-key traversal은 승인된 공개 test API를 실질적으로 보강하므로 사용자
재승인 전 Step 2-R은 `PENDING`이다. 구현, Testcontainers와 CI 증거는 이후 단계다.
