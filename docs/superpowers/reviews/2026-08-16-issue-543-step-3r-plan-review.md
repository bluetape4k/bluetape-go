# Issue #543 Step 3-R 구현계획 리뷰

## 문서 상태

- 대상: Issue #543 Gin adapter
- 리뷰 단계: Step 3-R, 구현계획
- 리뷰일: 2026-08-16
- 리뷰 범위: 승인된 설계, 구현계획, 현재 Git worktree 경계
- 구현 코드: 아직 없음

## SPW-01~05 점검

- SPW-01 범위: 계획은 resilience core marker, Gin bridge 네 종류, conformance, 문서, benchmark, PR handoff를 포함한다. JWKS network/provider는 #545로 제외했다.
- SPW-02 실행 가능성: 각 task에 정확한 파일, 실패 테스트, 명령, expected outcome, Lore commit intent/trailer를 적었다.
- SPW-03 검증 가능성: serial/race/conformance/benchmark/script-contract/Step 6-R 검증을 단계별로 배치했다.
- SPW-04 추적성: Issue #543, parent Epic #540, 설계 문서, review, benchmark output, lesson, PR body link를 Task 10에서 재확인한다.
- SPW-05 언어·문서: 외부 문서와 README는 한국어 우선이며, Go identifier·command·URL·machine token은 원문을 유지한다.

## 독립 lane 결과와 remediation

| Lane | 초기 판정 | 계획에 반영한 remediation | 최종 판정 |
| --- | --- | --- | --- |
| Performance | P1 3, P2 3 | timer 경계와 overhead 산식, cold/warm protocol, serial/parallel join invariant, CPU matrix, environment provenance, atomic capture, parser/chart generator와 failure artifact를 고정했다. | P0 0 / P1 0 |
| Stability | P1 3, P2 2 | NonRetryable이 custom RetryIf와 circuit cancellation 예외보다 먼저 처리되도록 하고, body GetBody/replay 불가, response header/attempt state 복원, outer Recovery 순서와 concurrent tests를 고정했다. | P0 0 / P1 0 |
| Security | P1 2, P2 3 | ParseOptions/Policies 방어적 복사, ContextParser strict cancellation과 legacy best-effort 분리, callback request의 Authorization 제거, typed-nil 거부, web.Problem 기반 redaction, rate-limit cancellation mapping을 고정했다. | P0 0 / P1 0 |
| Operator/Ops | P1 1, P2 4 | preflight→canary→observe→rollback→recovery runbook, readiness의 no-startup-validation 경계, observer fields/threshold, benchmark timeout/output limit/dirty-tree gate, script-contract CI, PR checks/watch 명령을 추가했다. | P0 0 / P1 0 |
| Developer/API | P1 5, P2 2 | web.RequestContextOptions 중복 정의를 제거하고 Gin-native RateLimitKeyFunc, ContextParser variadic contract, AuthenticationError Error/ProblemDetails, typed-nil validation, global Gin import boundary, concrete examples를 고정했다. | P0 0 / P1 0 |
| User/Caller | P1 1, P2 4 | 실제 fixture를 갖춘 ExampleBootstrap/ExampleMigration, Gin-specific conformance assertions, observer wiring table, README parity/readiness/runbook을 계획에 추가했다. worktree 경로도 실제 하이픈 경로로 교정했다. | P0 0 / P1 0 |

## Main integration review

### 유지한 경계

1. Gin import는 web/gin에만 둔다. web, webtest, root/core는 framework-neutral 상태를 유지한다.
2. RequestContext는 web.RequestContextOptions를 재사용하고, JWT는 Parser 또는 ContextParser 중 하나만 선택한다.
3. RateLimitKeyFunc는 Gin-native API이고 내부에서 ratelimit.KeyFunc로 변환한다.
4. 이미 response가 commit된 뒤의 handler 오류는 성공으로 바꾸지 않고 resilience.NonRetryable로 policy failure를 보존한다.
5. callback은 호출자 소유지만 raw token/parser error를 읽거나 기록하지 못하도록 sanitized request 경계를 둔다.
6. benchmark는 parser-only JWT fixture를 사용하며 JWKS network/provider는 #545로 남긴다.

### P1 blocker 확인

현재 계획 문서 기준 known P1은 없다. 다음 구현 단계에서 이 조건을 깨는 diff가 발생하면 Task 10 Step 6-R에서 P1로 되돌리고 구현을 중지한다.

### 남은 P2와 수용 조건

- benchmark 숫자는 local snapshot evidence이며 clean-tree와 동일 fixture provenance가 없으면 N/A로 기록한다.
- Gin-specific 상태 관찰은 framework-neutral conformance와 별도 assertion으로 유지한다.
- RequestContext startup validation API는 추가하지 않고, readiness 문서의 첫-request 400 경계와 ExampleBootstrap smoke test로 명시한다.

## 계획 self-review evidence

실행한 명령:

~~~bash
git rev-parse --show-toplevel
git worktree list --porcelain
rg -n 'TODO|TBD|FIXME|later|appropriate|handle edge cases|placeholder' docs/superpowers/plans/2026-08-16-issue-543-gin-adapter.md
git diff --check -- docs/superpowers/plans/2026-08-16-issue-543-gin-adapter.md docs/superpowers/specs/2026-08-16-issue-543-gin-adapter-design.md
~~~

기대 결과:

- root는 /Users/debop/work/bluetape4k/bluetape-go/.worktrees/feat-gin-adapter-543다.
- worktree branch는 feat/gin-adapter-543다.
- 금지된 placeholder/TODO 표현과 whitespace 오류가 없다.
- 계획은 10 task, 68개 이상의 추적 checkbox, 단계별 명령과 expected outcome을 가진다.

## Gate verdict

- P0: 0
- P1: 0
- P2: 계획에서 수용 조건으로 추적
- verdict: APPROVED FOR TDD IMPLEMENTATION
- merge: 별도 최신 사용자 승인 전까지 보류
