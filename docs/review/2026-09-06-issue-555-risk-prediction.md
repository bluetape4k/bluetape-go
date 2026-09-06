# Issue #555 Graph Backend Conformance 위험 예측

## 기준과 판정 규칙

- 기준 branch: `feat/issue-555-graph-conformance`
- 구현 전 HEAD: `10b887b97236b6d704429162aca1d5121aca7ffd`
- 승인 근거: #555 spec, Step 2-R review, 구현 plan, Step 3-R review
- 현재 source 상태: `graph/**/*.go` 변경 없음
- 판정: 구현 전 관찰 가능한 계약은 `PASS`, 실행 증거가 필요한 항목은 `PENDING`,
  안전한 복구 경로가 없으면 `BLOCKED`로 기록한다.

## 위험 예측표

| 위험 | 조기 신호와 관찰 명령 | 완화와 중단 기준 | 현재 판정 | 복구 또는 재실행 owner |
| --- | --- | --- | --- | --- |
| callback non-cooperation | `go test -count=1 ./graph/graphtest -run 'Test.*NonCooperative'`; timeout 뒤 callback goroutine이 남거나 cleanup이 먼저 시작된다. | operation cancel 뒤 bounded join을 요구한다. join grace를 넘기면 cleanup/close를 실행하지 않고 fail-stop한다. | PENDING | Task 4 lifecycle commit owner |
| signal-timeout join 경쟁 | `go test -race -count=10 ./graph/graphtest -run 'Test.*(Started|Timeout|Cancel)'`; 완료와 deadline 경합에서 성공이 선택되거나 `Started`가 누락·중복된다. | cancellation precedence와 `Started` exactly-once handshake를 고정한다. race 또는 순서 위반 시 Task 4로 rollback한다. | PENDING | Task 4 lifecycle commit owner |
| cleanup/close 경합 | `go test -race -count=10 ./graph/graphtest -run 'Test.*(Cleanup|Close|Order)'`; callback join 전 cleanup, close 선행, 중복 호출이 나타난다. | `callback join → fixture cleanup → adapter close` 순서와 exactly-once 호출을 검증한다. 위반 시 provider 연결을 중단한다. | PENDING | Task 4와 Task 5 owner |
| factory partial resource 유출 | `go test -count=1 ./graph/graphtest -run 'Test.*Factory'`와 provider 종료 로그; factory 오류·panic 뒤 client/container가 남는다. | factory가 반환 전 partial client를 닫고 container termination은 생성 직후 등록한다. 회수 증거가 없으면 provider suite를 중단한다. | PENDING | Task 4와 Task 7 provider factory owner |
| pre-materialization `limit+1` 누락 | `go test -count=1 ./graph/graphtest -run 'Test.*Oversized'`; sort/truncate 뒤 성공하거나 adapter가 상한 없는 query를 보낸다. | adapter query에 `limit+1`을 넣고 runner가 sort 전에 oversized result를 거부한다. 위반 시 adapter commit으로 rollback한다. | PENDING | Task 2와 Task 7 adapter owner |
| logical query submission 증가 | fake counter와 provider `t.Logf`를 읽는다. create/read/cleanup 한 동작이 둘 이상의 logical submission을 기록한다. | operation별 request builder와 counter를 고정하고 driver 내부 wire retry와 구분한다. logical count가 증가하면 provider migration을 중단한다. | PENDING | Task 5와 Task 7 provider adapter owner |
| metadata/query/credential 노출 | `go test -count=1 ./graph/graphtest -run 'Test.*(Redact|Metadata|ProviderError)'`; URI, query, password, namespace marker가 오류나 로그에 나타난다. | provider/phase/status/category/duration allowlist만 출력하고 raw cause는 외부 문자열로 노출하지 않는다. marker 발견 시 즉시 BLOCKED다. | PENDING | Task 1 validation과 Task 5 runner owner |
| digest/readiness drift | `rg -n 'sha256:|VerifyConnectivity' graph/provider_benchmark_test.go graph/neo4j`; mutable tag, digest 불일치, readiness 생략이 보인다. | benchmark에서 검증한 digest를 재사용하고 factory 반환 전 connectivity를 확인한다. drift가 있으면 source 확인 전 container를 시작하지 않는다. | PASS | Task 7 provider factory owner |
| old/new parity 손실 | 같은 tree에서 old provider test와 shared conformance를 실행한다. 기존 conversion/constructor assertion이 사라지거나 semantic case가 줄어든다. | parity 통과 뒤 겹치는 integration body만 별도 commit에서 삭제하고 pure unit test를 보존한다. 실패 시 Task 8 삭제 commit만 되돌린다. | PENDING | Task 8 migration owner |
| 10분 suite budget 초과 | `go test -count=1 -timeout=10m ./graph/neo4j -run 'Test.*Conformance'`를 세 process 순차 실행하고 phase duration을 기록한다. | broad retry 없이 phase별 원인을 분리한다. 단일 process가 10분을 넘기면 그 provider suite를 BLOCKED로 남긴다. | PENDING | Task 7과 Task 10 verification owner |
| local Testcontainers flake | 실행 직전 `colima status`, `docker context show`, `docker info`; bind/socket/startup 오류와 실제 test assertion을 구분한다. | healthy Colima는 재시작하지 않는다. runtime 불일치만 복구하고 전체 대상 명령을 처음부터 한 번 재실행한다. 반복 infrastructure 실패는 PENDING으로 공개한다. | PENDING | Task 7과 Task 10 verification owner |

## 구현 전 확인

- `git status --short`: clean
- `git log --oneline origin/develop..HEAD`: 승인된 spec/plan/review commit 세 개만 존재
- `git diff --check`: PASS
- `git diff --name-only origin/develop...HEAD`: 문서 네 개뿐이며 `graph/**/*.go` source 변경 없음

## 최종 관찰 갱신 규칙

Task 10에서 각 row의 실제 신호, 명령, 결과와 관련 commit을 덧붙인다. 예측 row는
삭제하거나 사후 설명으로 바꾸지 않는다. provider 실행이 환경 때문에 불가능하면 해당
row를 `PASS`로 간주하지 않고 정확한 오류와 다음 재실행 조건을 남긴다.
