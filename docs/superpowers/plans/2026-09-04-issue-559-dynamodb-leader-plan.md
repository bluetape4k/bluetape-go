# DynamoDB leader provider 구현 계획

> 승인 범위: 사용자가 승인한 0.20.0 Type A 실행의 #559 slice. 구현은
> `feat/issue-559-dynamodb-leader` worktree에서 수행하며, 다른 lane의 변경을
> 되돌리지 않는다.

## 목표와 완료 조건

`leader.Elector` 계약을 보존하는 caller-owned DynamoDB provider를 추가한다.
조건부 Put/Update/Delete, strongly consistent read, injected clock, renewal과
commit-unknown cleanup을 fake-first로 증명하고 package/parent README의
English/Korean pair를 유지한다. 완료 전 P0=0, P1=0, targeted normal/race,
vet/lint/format/tidy 증거와 PR exact-head CI를 확보한다.

## 파일 책임

| 경로 | 책임 |
|---|---|
| `leader/dynamodb/doc.go` | package 목적과 caller-owned lifecycle Go doc |
| `leader/dynamodb/options.go` | narrow client, option/config, schema/clock/logger 검증 |
| `leader/dynamodb/errors.go` | validation sentinel와 redacted provider helper |
| `leader/dynamodb/elector.go` | Campaign/Resign/Leader/renewal/local lifecycle |
| `leader/dynamodb/elector_test.go` | mutex-safe fake, lifecycle/failure/conformance tests |
| `leader/dynamodb/example_test.go` | compile-checked fake client example |
| `leader/dynamodb/README.md` | API/schema/IAM/consistency/capacity/runbook |
| `leader/dynamodb/README.ko.md` | 동일 정보 Korean locale |
| `leader/README.md`, `leader/README.ko.md` | backend index와 DynamoDB 링크 |
| `docs/superpowers/specs/2026-09-04-issue-559-dynamodb-leader-design.md` | 승인 설계/source ledger |
| 이 문서 | 실행 task와 검증 증적 |

## Task 1 — spec/plan 및 의존성 gate

- [x] live #559 body, parent/related issue, local leader/SQL/Redis/leadertest와 AWS 공식 문서를 확인한다.
- [x] API/zero-value/error/non-goal/context/goroutine/cleanup 계약을 spec에 고정한다.
- [x] source parity를 `keep`(leader contract/key lifecycle), `adapt`(SQL bounded lifecycle→DynamoDB conditions), `replace`(SQL query→SDK expressions), `defer`(Global Tables/fencing), `non-goal`(ORM)으로 기록한다.
- [x] plan review에서 performance/stability/security/operator/API/user 관점 P0/P1을 각각 확인한다. 초기 review의 unused alias/unbounded probe와 후속 review의 late response를 수정하고 Step 6-R 증적에 기록한다.
- [x] `go test` RED test skeleton을 먼저 작성하고 missing symbols의 첫 실패를 기록한다.

## Task 2 — validation와 typed error (TDD)

파일: `options.go`, `errors.go`, `*_test.go`

- [x] nil 및 typed-nil client(reflect nil-capable kinds), blank table, duplicate/unsafe attribute, invalid leader options, invalid clock/retry를 검증한다.
- [x] `ErrInvalidClient`, `ErrInvalidOptions`, `ErrMalformedItem`와 `leader.OperationError` wrapping을 구현한다.
- [x] raw AWS message/table/group/token이 `Error()`와 `%+v`에 나오지 않고 `errors.Is`/`errors.As`가 원인과 `leader.ErrCommitUnknown`을 보존하는지 테스트한다.
- [x] constructor가 client를 닫지 않고 option/attribute map을 alias하지 않는지 확인한다.

## Task 3 — item/request builder와 fake

파일: `elector.go`, `elector_test.go`

- [x] epoch-ms deadline, TTL epoch-second ceil, key/owner/lease/TTL AttributeValue를 생성한다.
- [x] Put condition `attribute_not_exists(#key)`, takeover Update condition `attribute_not_exists(#lease) OR #lease <= :now`, renewal owner+lease condition, resign owner condition을 정확히 캡처한다.
- [x] fake는 request map/AttributeValue를 deep-copy하고 call sequence/count/context와 output-plus-error를 기록한다.
- [x] marshal/parse malformed item과 number overflow/empty owner를 provider error text 없이 검증한다.

## Task 4 — Campaign GREEN lifecycle

- [x] local state `owned/campaigning/cleanup/generation/resigning`과 renewal cancel/done join을 mutex로 구현한다.
- [x] 최초 Put 성공, conditional contention, expired takeover, bounded retry와 caller context 종료를 구현한다.
- [x] conditional 이외 오류 뒤 strongly consistent probe가 own token을 확인하면 성공으로 reconcile한다.
- [x] 내부 attempt timeout 뒤 늦은 Put/Update 응답도 bounded probe로 reconcile하고, takeover deadline을 실제 update 직전에 재계산한다.
- [x] probe가 empty/other면 typed operation error, probe 실패 또는 post-dispatch cancellation이면 `ErrCommitUnknown`과 cleanup pending을 반환한다.
- [x] 같은 elector의 retry만 허용하고 busy 외 오류를 자동 재시도하지 않는다.

## Task 5 — renewal/resign/Leader GREEN lifecycle

- [x] renewal ticker는 `RenewInterval` 하나만 소유하고 context cancel/close를 deterministic하게 수행한다.
- [x] renewal conditional loss는 `IsLeader=false`와 no-cleanup, transport error+own probe는 recovery, probe failure는 cleanup pending으로 매핑한다.
- [x] resign은 cancel→done join→conditional Delete 순서를 지키고 gone/replaced conditional failure은 idempotent success로 처리한다.
- [x] resign transport error와 bounded probe를 reconcile하고 unknown이면 retry 가능한 cleanup pending을 남긴다.
- [x] `Leader`는 `ConsistentRead=true`, missing/expired empty, malformed typed error, injected clock을 증명한다.

## Task 6 — 문서/예제 및 conformance

- [x] package README 두 locale에 schema/IAM/read consistency/TTL/capacity/error/live opt-in/cleanup runbook을 쓴다.
- [x] parent README 두 locale에 sibling link와 backend 차이를 추가한다.
- [x] compile-checked example은 fake client만 사용하고 client/config/credential lifecycle이 caller-owned임을 보여준다.
- [x] `leadertest.Harness` adapter는 backend clock/conditional semantics가 다른 항목을 직접 연결하지 않고, fake equivalent scenario와 제외 사유를 Step 6-R에 기록한다.

## Task 7 — 검증 명령

변경 package는 순차 실행한다.

```bash
gofmt -w leader/dynamodb/*.go
go test -count=1 ./leader/dynamodb
go test -race -count=1 ./leader/dynamodb
go test -run Example -count=1 ./leader/dynamodb
go vet ./leader/dynamodb
make fmt-check
make tidy-check
make vet
make lint
git diff --check
```

가능하면 `go test -count=1 ./leader/...`를 실행하되, baseline에서 확인한
`leader/sql` timeout 등 비변경 실패는 package 결과와 분리한다. Floci/live
테스트는 explicit environment/build tag 없이는 실행하지 않고, 실행 시
readiness·cleanup·credential 경계를 증적에 남긴다.

## Task 8 — review/PR gate

- [ ] Step 6-R 7-Tier review: performance, stability, security, operator/Ops,
  developer/API, user/caller six lanes와 main integration을 수행한다.
- [ ] P0/P1 finding이 있으면 수정 후 같은 exact head에서 재검증한다. skipped/
  unavailable lane은 PASS가 아니라 PENDING/recorded fallback이다.
- [ ] Korean PR body 끝에 `## DoD Status`와 tests/static/review/docs/gaps를
  기록하고 issue #559/milestone/assignee를 live read-back한다.
- [ ] PR 생성은 승인된 target repo/base/head 범위에서만 수행한다. merge는
  fresh exact-head CI/review 확인 뒤 별도 승인을 받는다.

## 롤백과 후속

PR revert가 rollback이며 기존 leader backend/Redis/SQL key와 client lifecycle은
변경하지 않는다. Global Tables/fencing, live benchmark(#560)와 production IAM
rollout은 후속 issue/운영 결정으로 남긴다.
