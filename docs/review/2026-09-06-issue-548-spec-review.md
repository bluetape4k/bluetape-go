# Issue #548 설계 검토

## 검토 범위와 근거

- 대상 issue: <https://github.com/bluetape4k/bluetape-go/issues/548>
- 최초 검토 spec commit: `95327ba3b60321c9c8435d1a08ce16a6790dd7d7`
- 대상 문서:
  `docs/superpowers/specs/2026-09-06-issue-548-geo-coordinate-geohash-design.md`
- 근거: live issue 본문, RFC 7946, WGS 84, Elasticsearch geohash reference,
  `core`, `measure`, `money`, `graph`의 value-object/error/test pattern
- 명령 범위: 읽기 전용 source inspection과 Markdown 검증. 구현/Go test는 아직 실행하지 않았다.

## 최초 7-Tier 결과와 조치

| 우선순위 | Lens | 근거 | 조치 | 상태 |
|---|---|---|---|---|
| P1 | developer/API | Invalid `Cell{}`의 error 없는 accessor 결과가 정의되지 않았다. | `Center`/`Bounds`의 no-panic zero-result와 `Decode` error 우선 확인을 명시했다. | 해결 |
| P1 | main integration | `-180`과 `180`을 같은 meridian으로 선언했지만 `Bounds.Contains` 식에는 동치가 반영되지 않았다. | 양쪽 표현을 동일하게 포함하는 경계 규칙과 테스트를 추가했다. | 해결 |
| P2 | performance | 순수 hot path의 allocation 관찰 기준이 없었다. | 다섯 benchmark와 `ReportAllocs` 요구를 추가하고 compiler-sensitive 시간 threshold는 두지 않았다. | 해결 |
| P2 | developer/API | 외부에서 invalid `Point`를 만들 수 없는데 방어 validation 목적이 설명되지 않았다. | Package-internal/future decoding invariant 보호임을 명시했다. | 해결 |
| P2 | developer/API | `Cell`이 hash/precision을 caller에게 노출하는지 불분명했다. | 원본 hash는 보존하지 않고 precision은 내부 validation 전용임을 명시했다. | 해결 |
| P2 | user/caller | Degree 단위, Point/Bounds/GeoJSON 순서와 오류 precedence가 caller에게 불명확했다. | Named-variable example 요구와 함수별 오류/zero-result 표를 추가했다. | 해결 |
| P2 | operator/Ops | Issue delivery와 milestone release gate가 한 DoD에 섞일 수 있었다. | WIP/smoke evidence를 추가하고 milestone closure/tag/publication은 별도 release gate로 분리했다. | 해결 |

## 영향 lane 재검토

| Lens | 최종 결과 | 증거 |
|---|---|---|
| performance | `P0=0 P1=0` | Rerun lane이 시간 한도를 넘겨 중단됐다. `lane timed out; main integration fallback performed`. 결과/거리/encode/decode는 유한한 pure computation이고 benchmark/alloc 관찰이 spec에 반영됐음을 main session이 확인했다. |
| stability | `P0=0 P1=0` | Mutable state, goroutine, I/O와 lifecycle이 없는 범위임을 재확인했다. |
| security | `P0=0 P1=0` | Finite/range/precision 제한과 coordinate/hash 원문 비노출을 재확인했다. |
| operator/Ops | `P0=0 P1=0` | Issue delivery와 0.22.0 release authority가 분리됐음을 main integration에서 확인했다. |
| developer/API | `P0=0 P1=0` | Cell zero value, defensive validation, internal precision 계약을 repaired spec에서 read-back했다. |
| user/caller | `P0=0 P1=0` | Degree/order/error matrix와 example 요구가 반영됐음을 read-back했다. |
| main integration | `P0=0 P1=0` | Dateline 동치, geohash midpoint, acceptance/DoD traceability를 확인했다. |

## P2/P3 처분

모든 #548 P2는 spec에 반영했다. P3는 없다. Nanosecond hard threshold는 compiler와 host
noise를 public compatibility 약속으로 만들므로 의도적으로 거절하고 allocation report와
기능 invariant를 검증한다.

## Writer DoD

- `SPW-01 PASS`: 한국어 Type A spec review, live issue와 exact spec commit을 evidence로 고정했다.
- `SPW-02 PASS`: scope, severity, 위치, 조치, rerun, gap과 verdict를 포함했다.
- `SPW-03 PASS`: 식별자·명령·수치는 보존하고 한국어 기술 문장과 용어를 검토했다.
- `SPW-04 PASS`: 각 finding을 repaired spec의 계약과 acceptance/DoD에 다시 연결했다.
- `SPW-05 PASS`: Markdown 표·heading·link·최종 상태를 read-back했다.

## 판정과 남은 gate

최신 통합 결과는 `P0=0 P1=0`이다. 다만 dateline과 invalid `Cell`의 caller-visible 의미를
보강했으므로 사용자 재승인 전 Step 2-R은 `PENDING`이다. 구현, 테스트와 CI 검증은 이후
단계이며 현재 검토의 PASS로 간주하지 않는다.
