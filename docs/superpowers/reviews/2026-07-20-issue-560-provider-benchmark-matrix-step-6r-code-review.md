# 이슈 #560 Provider Benchmark Matrix Step 6-R 코드 리뷰

이슈: #560 `perf: Add ecosystem-driven provider benchmark analysis matrix`

날짜: 2026-07-20

기준 및 merge base: `cbbc68f05811625894217e8ea006fa03e0bbc009`

검토한 구현 SHA: `967edab49fada36a067c9d04e3c3a7bc6ce8d145`

측정 authority SHA: `1f1069f4b119957d96158c969f819d6374f902c8`

브랜치: `perf/issue-560-provider-benchmark-matrix`

게이트: 독립적인 여섯 관점과 메인 세션 통합 검토.

## 라이브 메타데이터

| 항목 | 라이브 결과 |
|---|---|
| 이슈 상태 | OPEN |
| 마일스톤 | `0.19.0` |
| 담당자 | `debop` |
| 레이블 | `type: task`, `area: leader`, `area: testing`, `area: utilities`, `area: graph`, `area: database`, `priority: p2` |
| Pull request | 이 게이트에서는 아직 생성하지 않음 |
| 원격 CI/리뷰/스레드 | 승인된 PR을 생성하기 전까지 N/A |

## 수렴 이력

첫 검토는 구현 및 측정 산출물을 함께 확인했고, 발견 사항을 회귀 테스트와 작은
보정 커밋으로 닫았다.

| 커밋 | 결정 |
|---|---|
| `a928e0a` | context를 무시하는 leader worker가 있어도 round가 제한 시간 안에 반환하도록 join을 제한하고 공유 오류 상태를 동기화했다. |
| `7613375` | Docker 직렬 실행, 환경 authority, provider/version 대조, 전체 재수집 절차를 운영 문서에 고정하고 실패 artifact 이름 충돌을 제거했다. |
| `1f1069f` | JSON/PEM redaction과 16 MiB 출력 상한을 fail-closed capture 계약으로 만들었다. |
| `8cfaa64` | 위 측정 authority에서 아홉 family를 다시 수집하고 표와 차트를 최종 측정값에 맞췄다. |
| `8dc09ff` | Cache와 Graph I/O의 비입증 범위를 실제 우선순위 결정 문장으로 바꿨다. |
| `8543fdb` | hyphen/camelCase credential alias 우회와 16 MiB 초과 override를 차단했다. |
| `967edab` | 정확히 상한인 출력을 성공으로 구분하고, lesson 증거를 실제 테스트와 맞추며, PNG 생성기를 단일 `rsvg-convert` 경로로 정리했다. |

최초 안정성 검토의 무제한 join, 보안 검토의 JSON alias 및 출력 상한, 운영 검토의
재수집 절차 및 정확한 상한 판정, 개발자 검토의 중복 rasterizer, 사용자 검토의
Cache/Graph I/O 우선순위 문구는 모두 보정됐다. 영향을 받는 관점은 변경 후 다시
검토했고, 최종 여섯 관점은 모두 같은 SHA를 확인했다.

## 최종 정확한 HEAD 결과

| Tier | 관점 | 판정 | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | 성능 | PASS | 0 | 0 | 0 | 1 |
| 2 | 안정성 | PASS | 0 | 0 | 0 | 0 |
| 3 | 보안 | PASS | 0 | 0 | 0 | 0 |
| 4 | 운영/Ops | PASS | 0 | 0 | 0 | 0 |
| 5 | 개발자/API | PASS | 0 | 0 | 0 | 0 |
| 6 | 사용자/호출자 | PASS | 0 | 0 | 0 | 0 |
| 메인 | 통합 | PASS | 0 | 0 | 0 | 1 |

모든 최종 lane은 구현 SHA
`967edab49fada36a067c9d04e3c3a7bc6ce8d145`를
`cbbc68f05811625894217e8ea006fa03e0bbc009` 기준으로 검토했다.

## 수용한 비차단 finding

성능 P3: leader benchmark는 시나리오마다 fresh fixture를 시작하므로 suite 전체
실행 시간은 늘어난다. 그러나 fixture startup은 timer가 멈춘 상태에서 일어나고
실제 provider API round만 측정한다. 시나리오 사이의 warm-state 오염을 피하는
대가이며 `ns/op`에는 포함되지 않으므로 변경하지 않는다.

## 측정 및 증거 경계

- 아홉 canonical family와 열 개 command block은 모두 측정 authority
  `1f1069f4b119957d96158c969f819d6374f902c8`, clean pre-run 상태,
  `exit_status: 0`, 기본 16 MiB 상한을 기록한다.
- 측정 authority 이후 Go benchmark source는 변경되지 않았다. 후속 변경은 산출물
  publication, 보고서 결정 문구, sanitizer 및 출력 경계, 단일 PNG renderer에 한정된다.
- 확장 credential/JSON/PEM/path/endpoint scan은 성공 및 개발 실패 artifact 전체에서
  검출 0건이었다. 현재 capture 계약은 private `limit+1` byte로 실제 overflow만
  판별하고 canonical 후보에는 최대 `limit` byte만 전달한다.
- 표, Vega-Lite JSON, SVG는 raw min/median/max와 일치한다. PNG 다섯 개는 각 SVG의
  정확한 2배 크기이고 현재 `rsvg-convert` 재생성 결과와 byte-for-byte 일치한다.

## 검증 증거

- `make ci` — 구현 및 최종 측정 산출물 head `8cfaa64`에서 PASS. 일반 테스트와
  race 테스트, Testcontainers package를 포함한다.
- 최종 HEAD의 `make fmt-check`, `make tidy-check`, `make vet`, `make lint` — PASS,
  lint `0 issues`.
- leader round 정상 join, winner cancellation, deadline, 비협조 worker, first-error
  peer cancellation 일반 및 race 회귀 — PASS.
- rate-limit, cache near-resource, graph lifecycle cleanup 집중 race 회귀 — PASS.
- `scripts/capture-provider-benchmark_test.sh` — atomic publication, failure retention,
  secret/PEM/JSON alias redaction, exact-limit 성공, overflow fail-close, 상한 override
  거부를 포함한 10개 계약 PASS.
- chart parser self-test, 두 번의 deterministic generation, SVG XML, Vega-Lite JSON,
  다섯 PNG 원본 시각 검토 — PASS.
- `git diff --check origin/develop...HEAD` 및 clean worktree — PASS.
- 영문/한국어 README의 아홉 family, Docker 필요성, 직렬 실행, snapshot 비순위화
  경고가 동등함을 확인했다.

## 메인 통합 판정

로컬 PR 준비 상태 PASS.

- P0 = 0
- P1 = 0
- P2 = 0
- P3 = 1, timer 밖 fresh fixture의 전체 실행시간 trade-off로 수용
- Leader, rate-limit, cache, Graph I/O, GraphDB 결과는 의미가 같은 행 안에서만
  비교하며 local/API lower bound, lease expiry, L1 reference object, L2 serialization,
  parser/materialization, path-shaped traversal 경계를 분리한다.
- 보고서는 universal winner를 선언하지 않고 사용 사례별 선택 및 비입증 범위와
  후속 우선순위를 명시한다.
- Type A lesson과 재현 가능한 raw/environment/chart evidence가 커밋돼 있다.
- 구현은 승인된 push 및 PR 생성 준비가 됐다. PR의 정확한 remote head가 CI와
  Step 7-R을 통과하기 전에는 merge-ready가 아니다.

## DoD

| 항목 | 상태 |
|---|---|
| 라이브 이슈 및 마일스톤 상태 갱신 | 완료 |
| 독립적인 여섯 관점 검토 | 완료 |
| 동일한 정확한 구현 SHA 검토 | 완료: `967edab49fada36a067c9d04e3c3a7bc6ce8d145` |
| 메인 통합 검토 | 완료 |
| P0/P1/P2 정규화 | 완료: `P0=0 P1=0 P2=0` |
| 수용한 P3 기록 | 완료: fresh fixture 실행시간 trade-off 1건 |
| 집중, race, container, 정적, 전체 CI 증거 | 완료 |
| 아홉 family raw output 및 다섯 표/차트 | 완료 |
| Type A 재사용 lesson | 완료: `docs/lessons/2026-07-20-issue-560-provider-benchmark-matrix.md` |
| 원격 CI/리뷰/스레드 | N/A: PR을 아직 생성하지 않음 |
| Merge 부수 효과 | 미승인: 새로운 명시적 merge 승인이 필요함 |
