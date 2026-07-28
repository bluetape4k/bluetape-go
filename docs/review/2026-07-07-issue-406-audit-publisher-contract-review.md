# Issue #406 Review: Audit Publisher Contract

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- `audit/sqloutbox.Publisher` contract documentation.
- `Relay.RunOnce` caller cancellation handling.
- PostgreSQL-backed cancellation, duplicate retry, and concurrent relay stress
  tests.
- English/Korean README updates and issue traceability docs.

## 서브에이전트 메모

Native subagent spawning was not used because the available subagent tool
surface is gated to explicit user subagent requests. The main session performed
the six independent review lanes and integration verdict as fallback.

## 관점별 발견 사항

### 성능

P0: 0
P1: 0

- `isCallerCancellation` is a constant-time check on an already-returned
  publisher error.
- The new stress test adds package test cost but remains scoped to
  `./audit/sqloutbox`.

### 안정성

P0: 0
P1: 0

- Caller cancellation now returns without converting shutdown into retry or
  dead-letter state.
- Non-cancellation publisher errors still use the existing `MarkFailed` path.
- Stress coverage exercises concurrent `RunOnce` claim/publish/mark behavior.

### 보안

P0: 0
P1: 0

- No new secrets, network clients, or broker credentials are introduced.
- README now states returned publisher errors should be bounded and redacted
  because failure text is persisted.

### 운영/Ops

P0: 0
P1: 0

- Shutdown semantics are explicit: caller context cancellation leaves claimed
  rows for lease-based recovery instead of writing false failure state.
- Adapter-owned topology, TLS, retention, replay, logging, metrics, and
  redaction responsibilities are documented.

### 개발자/API

P0: 0
P1: 0

- Public API shape stays stable: `Publisher.Publish(context.Context, Record)
  error`.
- The doc comment now describes at-least-once duplicates, dedupe keys, and
  cancellation behavior.
- No generic message-bus abstraction was introduced.

### 사용자/호출자

P0: 0
P1: 0

- README examples remain valid.
- English and Korean README files are synchronized for the new contract.
- Existing class and sequence diagrams remain visually sound and still cover
  the public participants and retry branch; no new diagram is needed.

## 통합 판정

P0: 0
P1: 0

The change is narrow and contract-preserving. It fixes an observable shutdown
semantics gap, adds regression coverage for cancellation, duplicate retry
envelope stability, and concurrent relay execution, and records diagram
judgment without redrawing unchanged visuals.

## 증거

- `go test -count=1 ./audit/sqloutbox`
- `go test -race -count=1 ./audit/sqloutbox`
- `go test -count=1 ./audit/sqloutbox -run 'RelayRunOnce(PublisherContextCancellationDoesNotRetry|RetriesDuplicatePublishWithStableEnvelope|ConcurrentStressPublishesEachRecordOnce)'`
- `golangci-lint cache clean && golangci-lint run ./...`
- `make ci`
- `git diff --check`
- Visual inspection:
  - `docs/images/readme-diagrams/audit-sqloutbox-class-contract-map.png`
  - `docs/images/readme-diagrams/audit-sqloutbox-relay-sequence.png`
