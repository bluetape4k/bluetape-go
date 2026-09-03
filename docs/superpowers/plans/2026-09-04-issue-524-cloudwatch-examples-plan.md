# Issue #524 CloudWatch 예시 구현 계획

## 목표

`examples/cloudwatch`에 Metrics와 Logs의 AWS SDK v2 호출 예시를 추가하고,
limits·cancellation·redaction·cardinality 주의를 fake-first 테스트로
검증한다. 기존 패키지의 public behavior는 변경하지 않는다.

## 작업 순서

- [ ] spec/risk artifact와 issue metadata를 고정한다.
- [ ] 실패하는 fake/request 테스트와 compile-checked examples를 작성한다.
- [ ] `cloudwatch`/`cloudwatchlogs` SDK dependency를 추가하고 request builders를 구현한다.
- [ ] bilingual README와 root package index를 동기화한다.
- [ ] `gofmt`, `go test -count=1 ./examples/cloudwatch`, race, `make tidy-check`,
      `make vet`, `make lint`, `git diff --check`를 실행한다.
- [ ] implementation review와 lesson artifact에 P0/P1 결과 및 N/A 범위를 기록한다.

## 파일 지도

| 파일 | 책임 |
|---|---|
| `examples/cloudwatch/doc.go` | package 설명과 caller-owned 경계 |
| `examples/cloudwatch/example_test.go` | request builders, fakes, examples, limits/redaction/cancellation tests |
| `examples/cloudwatch/README.md` | English usage and AWS contract notes |
| `examples/cloudwatch/README.ko.md` | 한국어 source-equivalent guide |
| `README.md`, `README.ko.md` | package discoverability |
| `go.mod`, `go.sum` | direct CloudWatch SDK modules |

## 검증/롤백

실제 AWS 호출은 기본 경로에 포함하지 않는다. 실패 시 해당 issue branch의
commit을 revert하는 것으로 rollback하며, 이미 전송된 metric/log를 삭제하거나
재처리하지 않는다.
