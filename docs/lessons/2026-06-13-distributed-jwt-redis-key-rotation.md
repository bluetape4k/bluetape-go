# Lessons Learned — Distributed JWT Redis Key Rotation (2026-06-13)

**Related issue**: #173
**Related PR**: pending
**Affected modules**: `jwt`, `jwt/redis`

## L1: Distributed JWT storage is a signing authority boundary

### Problem

Redis does not only cache JWT metadata in this feature. It stores active and
retained signing KeyChains, so an operator mistake can invalidate otherwise
valid tokens or expose signing material through weak infrastructure controls.

### Lesson

Document Redis as a trusted signing-key boundary whenever a distributed JWT
repository is added. README/runbook evidence must cover TLS, ACLs, namespace
isolation, persistence, eviction policy, diagnostics, reset limits, and explicit
token invalidation decisions.

### Evidence

- `jwt/README.md` and `jwt/README.ko.md` document Redis trust-boundary and
  runbook commands.
- `go test -race -p 1 -count=1 ./jwt ./jwt/redis` passed after the Redis
  provider and repository implementation.

## L2: Benchmark evidence needs chart and diagram context

### Problem

Raw benchmark numbers and Markdown tables make reviewers compare individual
values by hand. They also do not explain the Redis key-rotation path that
creates the measured operations.

### Lesson

When benchmark data is included in public docs or PR evidence, include a real
chart asset and a source-grounded diagram. README files should embed PNG assets,
with SVG and Graphviz evidence stored beside the generated images.

### Evidence

- `docs/images/readme-charts/distributed-jwt-redis-benchmark.png`
- `docs/images/readme-diagrams/redis-jwt-distributed-key-rotation.png`
- Diagram generator output reported `nodes=8 routes=9 segments=45` for the
  Redis JWT key-rotation diagram.

## L3: P2/P3 review findings should not expand the gate indefinitely

### Problem

Step 6-R can become slow when non-blocking findings trigger full re-review
cycles. That hides the real gate condition and delays PR evidence.

### Lesson

Keep `P0=0 P1=0` as the progression gate. Fix P2/P3 findings during the gate
only when they are local and risk-reducing; otherwise record them as follow-up
work with clear rationale.

### Evidence

- Step 6-R closed with `P0=0 P1=0 P2=0 P3=1`.
- Remaining P3 is parallel Redis/provider contention benchmark coverage, which
  is useful follow-up evidence but not a correctness blocker for #173.
