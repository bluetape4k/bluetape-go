# Issue #59 Spec: Audit Example Service

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 범위

- Repository: `bluetape4k/bluetape-go`
- Issue: #59 `Add audit event example service`
- Milestone: `0.9.0`

## 요구사항

- Add a runnable Go example service with `go test`.
- Demonstrate aggregate changes, audit repository writes, audit history queries,
  and optional outbox replay.
- Keep the example Go-native and framework-free.
- Document that the example is not a full event-sourcing framework, JaVers-style
  object diff engine, or durable source-of-truth database.

## Source Evidence

Read source examples under `/Users/debop/work/bluetape4k/bluetape4k-javers`:

- `examples/javers-exposed-ddd/README.md`
- `examples/javers-ktor/README.md`
- `examples/javers-spring-boot4/README.md`

Shared source shape: command-side order state is the source of truth, audit
history is read through JaVers snapshots, and Kafka/Redis/framework wiring is
example-specific infrastructure. The Go example ports the boundary, not the JVM
framework integration.

## Acceptance

- `examples/audit` has an `ExampleOrderService` with stable output.
- Tests cover repository boundary failure, history queries, outbox replay,
  concurrent commands with `GoroutineStressTester`, and cancellation with
  `AsyncJobTester`.
- README and README.ko explain the non-goals and future `audit/sqloutbox` path.
