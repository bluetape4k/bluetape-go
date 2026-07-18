# Issue #536 RESP3 Client Tracking Spike Step 6-R Code Review

Issue: #536

Date: 2026-07-19

Base: `origin/develop` at `f4acaab1676ca4a989051a28f60f37ab147d87f9`

Reviewed implementation SHA: `c320b487721b275b16ba555a335ba436323a64bf`

Gate: six independent perspectives plus main-session integration.

## Convergence History

The first six-perspective pass reviewed
`16735c013cb67e23734cb1572dd5a915bbdf9a76`. Five perspectives reported no
findings. Stability reported one P2 and one P3:

1. The callback gate test admitted only one callback and therefore did not
   directly prove that shutdown waits for every callback in a shared active
   generation.
2. The repair-context assertion required at least half of the configured
   timeout to remain, which made the proof sensitive to scheduler delay.

Commit `c320b487721b275b16ba555a335ba436323a64bf` closed both findings. The test
now admits two callbacks, remains blocked after the first finishes, and
completes only after the second. The repair assertion now proves a live,
deadline-bearing context with `0 < remaining <= repairTimeout` without an
elapsed-budget heuristic.

Every perspective then independently refreshed its review on the same repaired
implementation SHA. No lane timed out, and main-session fallback was not
required.

## Terminal Exact-Head Results

| Tier | Perspective | Verdict | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | Performance | PASS | 0 | 0 | 0 | 0 |
| 2 | Stability | PASS | 0 | 0 | 0 | 0 |
| 3 | Security | PASS | 0 | 0 | 0 | 0 |
| 4 | Operator/Ops | PASS | 0 | 0 | 0 | 0 |
| 5 | Developer/API | PASS | 0 | 0 | 0 | 0 |
| 6 | User/Caller | PASS | 0 | 0 | 0 | 0 |
| Main | Integration | PASS | 0 | 0 | 0 | 0 |

Every terminal lane reviewed
`c320b487721b275b16ba555a335ba436323a64bf` against
`f4acaab1676ca4a989051a28f60f37ab147d87f9`.

## Accepted Scope Boundaries

- The spike proves command-coupled RESP3 invalidation delivery on a dedicated
  physical connection. It does not prove an autonomous coherent pooled
  near-cache.
- A context-free Redis close can be watched and failed by the test, but cannot
  be cancelled by the watchdog. Separately bounded fixture cleanup remains the
  recovery path.
- AUTH, TLS, Sentinel, Cluster, proxies, managed providers, RESP2, and
  `REDIRECT` remain unproven. The evidence note explicitly rejects production
  RESP3 adoption rather than implying support.
- The environment ledger records the source SHA captured before the evidence
  note commit. It labels that provenance explicitly and does not claim that the
  later review commits changed the observed Redis behavior.
- Issue #560 owns latency, throughput, allocation, cadence, and provider
  comparison measurements.

These are declared limits of the evidence-only spike, not unresolved defects.

## Verification Evidence

- Complete `^TestRESP3TrackingSpike` suite — PASS.
- Six Docker-backed integration cases with `-p 1 -count=3` — PASS.
- `go test -race -count=1 ./cache/redisnear` — PASS.
- `go test -count=1 ./cache/redisvalue ./cache/redisnear` — PASS.
- Stability repair subset under `-race -count=20` — PASS.
- Two-callback gate test with `-count=200` — PASS.
- `make fmt-check` — PASS.
- `make tidy-check` — PASS.
- `make vet` — PASS.
- `make lint` — PASS, `0 issues`.
- `make test` — PASS repository-wide.
- `git diff --check origin/develop...c320b487721b275b16ba555a335ba436323a64bf`
  — PASS.
- No production Go file, public API, dependency, or package README changed.
- The working tree was clean at the reviewed implementation SHA.

## Main Integration Verdict

PASS.

- P0 = 0
- P1 = 0
- P2 = 0
- P3 = 0
- The evidence supports explicit command drain only, records physical-connection
  affinity and disconnect loss, and rejects an autonomous production RESP3
  near-cache on the tested go-redis surface.
- The L1 reference-object and L2 serialization boundary remains accurate and
  unchanged.
- Production remains on `redisnear.NewPubSub`; any autonomous pump or coherent
  RESP3 near-cache API requires a separate Type A issue.
- Lesson gate: N/A. The reusable result is already preserved in the indexed
  research evidence ledger; a second lesson artifact would duplicate it.
- Push and PR creation remain outside the current authority.

## DoD

| Item | Status |
|---|---|
| Six independent perspectives covered | Done. |
| Same exact repaired implementation SHA reviewed | Done: `c320b487721b275b16ba555a335ba436323a64bf`. |
| Initial P2/P3 repaired and independently refreshed | Done. |
| Main integration review completed | Done. |
| P0/P1 normalized | Done: `P0=0 P1=0`. |
| Targeted, race, integration, static, and repository evidence | Done. |
| Production/API/dependency changes | None. |
| Push or PR side effect | Not performed; authority gate preserved. |
