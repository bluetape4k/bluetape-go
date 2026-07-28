# Issue #407 Review: Audit Publisher Adapter

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- New `audit/sqloutbox/sqloutboxtest` helper package.
- Deterministic publisher helpers for discard, function adaptation, recording,
  duplicate-attempt assertions, and failure injection.
- English/Korean README updates, research note, and lesson note.
- Diagram decision for a package that implements an existing
  `sqloutbox.Publisher` participant.

## 서브에이전트 메모

Native subagent spawning was not used because the available subagent tool
surface is gated to explicit user subagent requests. The main session performed
the six independent review lanes and integration verdict as fallback.

## 관점별 발견 사항

### 성능

P0: 0
P1: 0

- `DiscardPublisher` and `PublisherFunc` add no allocations beyond caller work.
- `RecordingPublisher` uses one mutex and append-only attempt storage, which is
  appropriate for deterministic tests and examples.
- Failure injection is map lookup by event ID and stays out of production relay
  hot paths unless callers opt into this helper.

### 안정성

P0: 0
P1: 0

- Helpers honor context cancellation before recording or invoking caller code.
- `RecordingPublisher` is zero-value ready and concurrent-safe.
- Relay integration coverage proves retry and dead-letter handoff with
  Testcontainers-backed PostgreSQL.

### 보안

P0: 0
P1: 0

- No broker credentials, network clients, TLS settings, or durable topology are
  introduced.
- README boundaries state the package is not a durable transport adapter.
- Returned failure errors remain caller/test supplied and only affect existing
  sqloutbox persisted failure text paths.

### 운영/Ops

P0: 0
P1: 0

- The helper package does not imply Kafka/NATS/Redis Streams/RabbitMQ/Redpanda/
  Pulsar operational support.
- Documentation keeps broker topology, retention, replay, redaction, and
  consumer idempotency in later adapter or application-owned scopes.

### 개발자/API

P0: 0
P1: 0

- Public API is narrow and idiomatic: `DiscardPublisher`, `PublisherFunc`,
  `RecordingPublisher`, `WithFailures`, and sentinel errors.
- Examples compile as Go example tests.
- `Records` and `EventIDs` return defensive snapshots.

### 사용자/호출자

P0: 0
P1: 0

- Root, audit, and sqloutbox READMEs link the package from both English and
  Korean docs.
- Delivery semantics are documented: cancellation, non-cancellation failure,
  duplicate attempts, stable event ID / idempotency key handoff, and reset.
- No package-local diagram is needed because the existing sqloutbox class and
  relay sequence diagrams already include the `Publisher` participant.

## 통합 판정

P0: 0
P1: 0

The change is narrow and contract-preserving. It implements the first audit
publisher adapter as a deterministic test/example helper, avoids premature
broker topology, and adds stress/race plus relay-backed retry/dead-letter
coverage.

## 다이어그램 증거

| Gate | Evidence | Result |
|---|---|---|
| Touched diagram files | `git diff --name-only \| rg '(^|/)docs/images/readme-diagrams/.*\.(svg\|png)$' || true` returned no paths | PASS, no changed SVG/PNG |
| Need for new diagram | `docs/research/2026-07-07-issue-407-audit-publisher-adapter.md` records that `sqloutboxtest` adds no new runtime topology or sequence beyond `sqloutbox.Publisher` | PASS, documented exception |
| Existing class diagram eye check | Full-size visual inspection of `docs/images/readme-diagrams/audit-sqloutbox-class-contract-map.png` | PASS, readable and covers `Publisher` participant |
| Existing sequence diagram eye check | Full-size visual inspection of `docs/images/readme-diagrams/audit-sqloutbox-relay-sequence.png` | PASS, readable and covers relay publish/error branches |

## 증거

- `go test -count=1 ./audit/sqloutbox/sqloutboxtest`
- `go test -race -count=1 ./audit/sqloutbox/sqloutboxtest`
- `go test -count=1 ./audit/sqloutbox ./audit/sqloutbox/sqloutboxtest`
- `go test -race -count=1 ./audit/sqloutbox ./audit/sqloutbox/sqloutboxtest`
- `go test -count=1 ./audit/...`
- `git diff --check`
- `golangci-lint cache clean && make ci` after clearing stale lint cache that
  referenced the removed #406 worktree
