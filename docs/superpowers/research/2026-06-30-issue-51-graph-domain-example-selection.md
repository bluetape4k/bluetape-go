# Issue #51 Graph Domain Example Selection

## Decision

Port the observability incident graph as the only `0.10.0` domain example.

## Evidence

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
