# Issue #51 Graph Domain Example Selection

> 한국어 연구 요약: 이 문서는 사용자 협업용 조사/결정 기록이다. 아래 표와 목록의 URL, package name, command, issue number, version, source path는 evidence이므로 그대로 보존한다. 의사결정, 선택/보류/거절 사유, 후속 이슈 경계는 한국어 독자가 바로 이해할 수 있도록 이 요약을 우선 적용한다.
> 추가 한국어 해석: 이 문서에서 영어로 남은 표의 값은 원문 근거이며, 실제 채택 여부는 한국어 결정 문장을 따른다. 후속 작업자는 보류와 거절 항목을 새 구현 범위로 착각하지 않아야 한다.\n

## 결정

Port the observability incident graph as the only `0.10.0` domain example.

## 근거

- Source parity target: `bluetape4k-graph/examples/observability-graph-examples`
  provides a checkout incident scenario with service dependencies, public APIs,
  alerts, incident root cause, ownership, CSV fixtures, and runnable tests.
- #38 guidance on #51 recommends one concrete example before broad adapter work,
  with observability or IAM/access preferred because both map to Go services.
- #50 keeps backend adapters as follow-up work, so the example must stay
  backend-neutral and prove caller value through the `graph` and `graph/graphio`
  packages already in `0.10.0`.

## Implemented Scope

- `examples/graph/observability` seeds 10 vertices and 10 edges matching the
  source fixture shape.
- Tests prove upstream impact, downstream dependency, affected API,
  alert-boundary, ownership, and NDJSON round-trip behavior.
- README and README.ko document seed data, runnable commands, production
  omissions, and deferred source examples.
- A README topology diagram is rendered as SVG and PNG.

## Deferred Examples

- Code dependency, fraud, knowledge, social, and recommendation examples need
  broader domain models or backend traversal contracts to be more than toy
  fixtures.
- Ktor graph integration is intentionally not copied into Go because it is a
  JVM/Ktor integration shape, not a Go service boundary.
- IAM/access graph remains valuable as the next security-focused Go example and
  is tracked by #368.

## Stress Boundary

The example introduces no goroutine or async job runner. `go test -race` covers
the package, while graph I/O concurrency stress remains in `graph/graphio`.
