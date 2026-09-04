# Issue #524 CloudWatch 예시 구현 검토

## 2026-09-04 보강 검토

API/security 리뷰의 partial-rejection, service-range, typed-nil과 timestamp
overflow 지적을 반영했다. `PutLogEvents`가 `RejectedLogEventsInfo`와
`RejectedEntityInfo`를 typed error로 전달하고, metric 값(`±2^360`)과 Logs
timestamp(14일 이전/2시간 이후)를 preflight한다. client는 typed-nil reflect
검사를 통과해야 하며 log span 비교는 int64 overflow 없이 수행한다. rejection
index defensive copy, range/overflow/typed-nil 회귀 테스트와 targeted race/vet가
PASS다. 보강 판정은 `P0=0, P1=0`이며 새 commit의 exact-head CI를 별도 확인한다.

운영 관점의 P2로 partial rejection에서 이미 수락된 event를 전체 batch 재전송하지
않고 거부 index만 재시도해야 하는 caller 책임을 확인했다. EN/KO README와 lesson에
entity rejection의 부분 수락 가능성과 reconciliation 경계를 명시했으며 P0/P1
차단은 없다.

## 검토 범위

기준 base `906a68fdb41551ccaa6ce1394a2370e654ade10e`, branch
`feat/issue-524-cloudwatch-examples`의 `examples/cloudwatch`, root README
index, `go.mod`/`go.sum`, 설계·계획 artifact를 검토했다.

## 증적

| 명령 | 결과 |
|---|---|
| `go test -count=1 ./examples/cloudwatch` | PASS |
| `go test -race -count=1 ./examples/cloudwatch` | PASS |
| `go test -run '^Example' -count=1 ./examples/cloudwatch` | PASS (compile-only examples) |
| `go vet ./examples/cloudwatch` | PASS |
| `make tidy-check` | PASS (commit 후 go.mod/go.sum drift 없음) |
| `gofmt -l` 대상 확인 | PASS |
| `git diff --check` | PASS |
| baseline `go test -count=1 ./...` (origin/develop) | PASS |

## 판정

- P0: 0 — payload/credential leak, unsafe global state, silent write 없음.
- P1: 0 — fake request deep copy, service limit preflight, cancellation
  before/after response, provider error redaction, sequence-token omission을
  테스트로 확인했다.
- P2: 1 — partial acceptance/retry 운영 지침을 문서화했으며 API 확장은 범위 밖이다.
- P3: 0 — 현재 범위에서 추가 finding 없음.

실제 AWS/Floci endpoint, IAM/provisioning, retry worker와 global observability
registry는 명시적 N/A다.
