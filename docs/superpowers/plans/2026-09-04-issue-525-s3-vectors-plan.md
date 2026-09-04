# #525 S3 Vectors 구현 계획

## SPW-01 — 범위와 기준

이 계획은 승인된 Type A epic #517의 child issue #525만 다룬다. 대상 branch는
`feat/issue-525-s3-vectors`, base는 `develop`이다. 설계 기준은
`docs/superpowers/specs/2026-09-04-issue-525-s3-vectors-design.md`와
GitHub issue #525다.

## 실행 순서

1. `s3vectors` package, `doc.go`, errors와 caller-owned `Client`/`Provider`를
   추가한다. SDK의 eight operation을 thin forwarding method로 제공한다.
2. 검증을 먼저 수행한다. nil/typed-nil client, zero provider, 식별자,
   vectors/keys, finite float32, TopK와 parallel segment 조합을 검사하고
   malformed output과 cancellation을 경계에서 처리한다.
3. fake client를 deep-copy 방식으로 작성하고 table-driven RED/GREEN 테스트로
   request construction, metadata/filter 보존, input 불변성, SDK 오류/응답,
   context cancellation과 redaction을 고정한다.
4. `ExampleNew`와 package README 두 locale를 작성한다. README에는 AWS client,
   credentials, embedding/schema ownership, paginator 사용, live opt-in 및
   검증되지 않은 local emulator 비지원 범위를 포함한다.
5. `gofmt`, `go test`, `go test -race`, `go vet`, `go mod tidy`와 diff 검사를
   실행한다. 이 package에는 shared mutable state/worker가 없으므로 별도
   stress harness는 N/A로 기록하되 race test는 수행한다.
6. 최종 diff/공개 docs/lesson을 읽고 P0/P1 finding을 0으로 수렴한 뒤 부모
   세션에 변경 파일, API, 명령 결과와 남은 live-test gap을 보고한다.

## 파일별 DoD

| 경로 | 결과 |
|---|---|
| `s3vectors/*.go` | SDK eight-operation thin adapter, validation, safe errors |
| `s3vectors/*_test.go` | fake-first success/failure/cancellation/redaction proof |
| `s3vectors/README.md` | English usage and operational boundaries |
| `s3vectors/README.ko.md` | 한국어 source-equivalent usage and limitations |
| `s3vectors/doc.go` | exported package contract and ownership |
| `go.mod`, `go.sum` | `service/s3vectors` direct dependency only |
| `docs/lessons/2026-09-04-issue-525-s3-vectors.md` | reusable lesson and future guard |

## 검증·rollback

- RED 단계에서 fake 호출 횟수와 request snapshot을 먼저 고정하고 구현 후
  같은 명령을 GREEN으로 재실행한다.
- 실패 시 변경 package만 되돌리지 않고 실패 원인에 맞춰 validation,
  error wrapping 또는 fake snapshot을 수정한 뒤 영향받은 테스트를 다시
  실행한다.
- live AWS와 emulator는 기본 검증에서 실행하지 않는다. 그러므로 이번
  branch의 성공 기준은 fake/compile evidence이며 live compatibility는
  별도 opt-in 후속 작업이다.

## SPW-02 — 계획 계약

의존 순서, 정확한 파일, acceptance 매핑, test command, rollback/rerun 지점을
기록했다. PR/merge/cleanup은 부모 workflow가 별도 gate로 소유한다.

## SPW-03 — 한국어 기술 문체

파일명, API, 명령, branch/ref는 그대로 두고 계획 설명은 한국어 기술 문체로
작성했다. `fake-first`, `live opt-in`, `thin adapter`는 code/workflow token으로
보존했다.

## SPW-04 — 추적성

설계의 eight operation과 ownership을 구현 파일 및 테스트 항목에 매핑했다.
기본 CI가 live AWS/emulator를 요구하지 않는다는 issue acceptance를 테스트
범위와 README DoD에 연결했다.

## SPW-05 — read-back

계획을 저장 후 다시 읽어 단계 순서, 파일별 결과, rollback과 N/A 사유가
모순 없이 이어지는지 확인했다. 구현·package 검증·lesson 기록은 계획과
일치하며, PR/merge/cleanup은 부모 workflow의 별도 gate로 남겼다.
