# Issue #548 구현 계획 검토

## 검토 범위와 근거

- 대상 issue: <https://github.com/bluetape4k/bluetape-go/issues/548>
- 승인 spec commit: `95327ba3b60321c9c8435d1a08ce16a6790dd7d7`
- 계획 작성 기준 commit: `d6c1b977dbfa699626f10d9a1eb6d435d8d894c6`
- 검토 대상:
  `docs/superpowers/plans/2026-09-06-issue-548-geo-coordinate-geohash-plan.md`
- 검토 대상 SHA-256:
  `3664a3efc7194005b0bb95aa5295a6357129c55d142728daab331cc30c0e4eab`
- 근거: 승인 spec, live issue metadata, 현재 `core`·`measure`·`money` 값 타입과
  오류·테스트 관례, Go 표준 fuzz/benchmark 계약
- 명령 범위: 계획과 저장소를 읽고 Markdown·용어·scope를 검증했다. 구현과 Go test는
  아직 실행하지 않았다.

## 최초 Step 3-R 결과와 조치

여섯 독립 관점의 중복 finding은 main integration에서 하나의 계획 결함으로 정규화했다.

| 우선순위 | Lens | 근거 | 조치 | 상태 |
| --- | --- | --- | --- | --- |
| P1 | user/caller | degree 단위, `Point`·`Bounds`·GeoJSON 인수 순서와 오류 precedence를 호출자가 오해할 수 있었다. | 실행 가능한 example, 함수별 zero result·`errors.Is` 표, English·Korean locale 문서의 완전한 snippet을 추가했다. | 해결 |
| P1 | user/caller | antimeridian과 `-180`/`180` 동치가 예제와 테스트에서 충분히 연결되지 않았다. | 일반·crossing·full-world·edge 행렬과 양쪽 meridian 표현 검증을 추가했다. | 해결 |
| P2 | performance | Hot path allocation, precision 양끝과 실행 환경의 재현 근거가 부족했다. | 다섯 benchmark, precision 1/12, ordinary/antimeridian sub-benchmark, constructor 사전 검증과 tracked WIP ledger를 고정했다. | 해결 |
| P2 | stability | Bounds finite out-of-range, 극점 거리와 `Cell.Validate` precedence 행렬이 불완전했다. | 네 필드 범위·다중 오류, pole/near-pole finite 결과와 `precision → center → bounds` 테스트를 추가했다. | 해결 |
| P2 | security | NaN/Inf, 긴 hash와 원문 오류 노출의 fail-closed 경계가 더 명시적이어야 했다. | 모든 필드 finite/range, hash 길이 1..12 우선 검사와 값 비노출 sentinel 검증을 추가했다. | 해결 |
| P2 | operator/Ops | Issue delivery, exact-head CI와 milestone release 증거가 섞일 수 있었다. | 구현·PR·Nightly·tag/publication authority와 stop condition을 분리했다. | 해결 |
| P2 | developer/API | Invalid zero value와 accessor, encode/decode의 다중 오류 순서가 불명확했다. | 공개 함수별 validation 순서, zero result, `Cell` 방어 validation과 known vector를 고정했다. | 해결 |

## 최종 영향 lane 검토

| Lens | 최종 결과 | 증거 |
| --- | --- | --- |
| performance | `P0=0 P1=0 P2=0` | 두 차례 독립 재검토에서 bounded precision/length, constructor 사전 검증, 세 반복 allocation 판정과 WIP raw ledger를 확인했다. |
| stability | `P0=0 P1=0 P2=0` | 독립 재검토에서 모든 finite out-of-range, 극점·near-pole, `Cell` precedence와 fuzz corpus 재현 계약을 확인했다. |
| security | `P0=0 P1=0` | 길이·precision·finite/range 상한, panic 없는 실패와 coordinate/hash 원문 비노출을 read-back했다. |
| operator/Ops | `P0=0 P1=0` | 구현·PR·exact-head CI·Nightly·release gate와 실패 시 `PENDING` 전환을 read-back했다. |
| developer/API | `P0=0 P1=0` | 공개 값 타입, accessor, sentinel/zero result, validation precedence와 no-dependency 경계를 read-back했다. |
| user/caller | `P0=0 P1=0` | Named-variable example, degree/GeoJSON 순서, antimeridian, error handling과 locale parity를 read-back했다. |
| main integration | `P0=0 P1=0` | Spec coverage 표의 모든 계약이 선행 task, RED/GREEN 명령과 evidence에 연결됨을 확인했다. |

## P2/P3 처분

모든 P2는 계획에 반영했다. 더 깊은 Geohash longitude/latitude subdivision midpoint
fixture는 P3 후속 보강으로 남긴다. 현재 계획은 공식 prefix 1..12, 전역 midpoint,
`Nextafter` 인접값, WGS 84 모서리와 모든 precision round-trip을 이미 검증하므로 구현을
막지 않는다. `ns/op` 고정 threshold는 compiler와 host noise를 기능 회귀로 오판하므로
거절하고 allocation과 기능 invariant만 gate로 사용한다.

## Writer DoD

- `SPW-01 PASS`: 독자는 구현자와 reviewer이며, 승인 spec·issue·repository pattern을
  변경 불가 근거로 고정했다.
- `SPW-02 PASS`: Writing Plans header, checkbox task, exact path·명령·RED/GREEN·commit과
  handoff 계약을 확인했다.
- `SPW-03 PASS`: 한국어 기술 문장에 KO-01부터 KO-07까지 적용하고 식별자·명령·수치를
  보존했다.
- `SPW-04 PASS`: 모든 finding을 수정된 step과 spec coverage evidence에 다시 연결했다.
- `SPW-05 PASS`: heading, 표, 목록, code fence, link와 stop condition을 최종 read-back했다.

### Korean naturalness

- `KO-01 PASS`: SHA, 수치, 명령, API 이름과 판정 의미를 고정했다.
- `KO-02 PASS`: 추상적 품질 표현을 구체적인 test·command·threshold로 바꿨다.
- `KO-03 PASS`: 번역투 접속어와 반복 결론을 제거했다.
- `KO-04 PASS`: 좌표·경계·정밀도·할당 용어를 문맥별로 일관되게 사용했다.
- `KO-05 PASS`: 검토 문서에 불필요한 홍보 문구나 유머를 넣지 않았다.
- `KO-06 PASS`: plan, review, locale handoff와 공개 문서 요구를 함께 확인했다.
- `KO-07 PASS`: 문맥 용어 audit에서 차원·주체가 뒤바뀐 표현을 찾지 못했다.

## 판정과 남은 gate

Step 3-R 최종 판정은 `PASS`, `P0=0 P1=0`이다. 구현은 아직 시작하지 않았고, plan
승인과 실행 방식 선택 전에는 coordinator의 `plan`·`plan-review` gate도 `PENDING`으로
유지한다. PR, CI, merge, tag와 publication은 각각 별도 gate다.
