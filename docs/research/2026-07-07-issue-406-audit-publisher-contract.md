# Issue #406 Audit Publisher Contract

Issue #406은 #407의 첫 helper adapter package 전에 `audit/sqloutbox.Publisher` boundary를 안정화한다.

## 결정

publisher boundary는 `audit/sqloutbox.Record`와 `context.Context`에 둔다. 이 slice에서는 generic broker abstraction을
도입하지 않는다.

contract는 다음과 같다.

- delivery는 at-least-once이므로 publisher와 consumer는 duplicate publish attempt를 처리해야 한다.
- `Record.EventID`와 `Record.IdempotencyKey`는 retry 및 expired claim lease 전반의 stable deduplication key다.
- caller-owned context cancellation 또는 deadline error는 `MarkFailed`를 호출하지 않고 `RunOnce`를 멈춘다.
- non-cancellation publish error는 bounded failure text로 retry/dead-letter state에 저장된다.
- broker topology, authentication, TLS, retention, replay, logging, metrics, redaction은 adapter/application responsibility로
  남는다.

## Implementation Scope

- public `Publisher` doc comment에 at-least-once, idempotency, caller cancellation rule을 확장했다.
- `Relay.RunOnce`는 wrapped caller context cancellation error를 retry/dead-letter failure로 저장하지 않고 그대로 반환하도록 바꿨다.
- PostgreSQL-backed store 위에서 cancellation, duplicate retry envelope, concurrent `RunOnce` stress test를 추가했다.
- `audit/sqloutbox` README pair와 parent `audit` README pair를 English/Korean으로 갱신했다.

## Diagram Decision

#406에는 새 README diagram이 필요하지 않다.

근거:

- 기존 class contract map은 source caller, `Store`, `Relay`, `Publisher`, `Record`라는 public participant를 이미 보여 준다.
- 기존 sequence diagram은 claim, publish error, retry/dead-letter, publish success branch를 이미 보여 준다.
- #406은 cancellation exception과 duplicate/idempotency wording을 바꾸지만, 기존 retry/error branch 및 README prose로 설명할 수
  없는 새 public type, transport participant, sequence branch를 추가하지 않는다.
- 기존 두 diagram의 full-size PNG를 검사했고 overlap, truncation, readability defect가 없었다.

## Validation Plan

- `go test -count=1 ./audit/sqloutbox`
- `go test -race -count=1 ./audit/sqloutbox`
- `git diff --check`
- PR 전 P0/P1 finding을 포함한 7-Tier review artifact.
