# Issue #523 SQS S3 envelope 구현 계획

## 목표와 완료 조건

`messaging/sqsextended`에 caller-owned SQS/S3 adapter를 추가한다. 완료 조건은
versioned envelope round-trip, bounded payload/checksum, S3→SQS send order,
SQS-first delete order, failure/cancellation/redaction 계약, fake-first tests,
EN/KO README와 fresh Go verification이다.

## 순서

1. `go.mod`의 기존 AWS S3/SQS module surface와 issue/research 근거를 확인한다.
2. 승인 설계를 `docs/superpowers/specs/`와 risk ledger에 고정한다.
3. package docs와 fake/test를 먼저 작성해 RED contract를 만든다.
4. envelope codec와 safe errors를 구현한다.
5. `Send`, `Receive`, `Delete`를 구현하고 request/response/cancellation 경계를
   fake로 GREEN 검증한다.
6. package README/README.ko.md와 root package index를 갱신한다.
7. `gofmt`, targeted test/race/vet, `make fmt-check`, `make tidy-check`,
   `make vet`, `git diff --check`를 실행하고 implementation review/lesson을
   남긴다.

## 파일 책임

| 경로 | 책임 |
|---|---|
| `messaging/sqsextended/doc.go` | package 목적과 ownership/cancellation Go doc |
| `messaging/sqsextended/envelope.go` | versioned canonical envelope codec와 validation |
| `messaging/sqsextended/errors.go` | redacted sentinel/typed error |
| `messaging/sqsextended/provider.go` | narrow SDK clients, send/receive/delete 순서 |
| `messaging/sqsextended/*_test.go` | fake-first table tests, failure/cleanup/race proof |
| `messaging/sqsextended/README*.md` | API와 operational boundary locale pair |
| `README*.md` | root package index/AWS section |
| `docs/review/*523*` | risk/implementation 7-tier evidence |
| `docs/lessons/*523*` | 재사용 가능한 Go/AWS envelope 교훈 |

## Risk-driven test matrix

| 위험 | 완화/증거 |
|---|---|
| S3 성공 후 SQS 실패로 orphan 발생 | `Send` call order, `OrphanedObject`, no auto-delete test |
| SQS visibility timeout보다 payload read가 오래 걸림 | `VisibilityTimeout` pass-through과 README caveat, no implicit extension |
| partial read/large response allocation | `ContentSize` upper bound + `io.LimitReader`/extra-byte detection |
| payload corruption/missing object | `GetObject` close, exact size, SHA-256 mismatch와 no delete test |
| SQS ack 후 S3 delete 실패 | SQS-first order, `QueueDeleted`/typed error test |
| AWS error/payload leak | typed error `Error`/`%+v` redaction assertions |
| fake aliases caller request/response | deep-copy input, fresh output/body and concurrent isolation |
| typed-nil interface panic | reflect nil-capability constructor table |

## Go-patterns hardening mapping

- GO-HARD-02/07: SDK method subset과 no-live dependency decision을 spec에 고정한다.
- GO-HARD-03: envelope canonical order, exact caller bucket/key, bounded bytes를 검증한다.
- GO-HARD-04: context checkpoints, resource close, SQS/S3 cleanup order를 검증한다.
- GO-HARD-06: package README와 compile-checked example을 검증한다.
- GO-05: provider 자체는 mutable shared state가 없으므로 race는 fake concurrent
  isolation과 parallel Send/Receive call safety에 한정한다.

## 검증 명령

```bash
gofmt -w messaging/sqsextended/*.go
go test -count=1 ./messaging/sqsextended
go test -race -count=1 ./messaging/sqsextended
go vet ./messaging/sqsextended
make fmt-check
make tidy-check
make vet
git diff --check
```

live AWS/LocalStack/Floci는 검증하지 않으며, 이는 실패가 아니라 명시적 N/A다.
