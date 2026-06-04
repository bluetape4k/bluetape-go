# Issue #116 Redis NearCache Failure-Injection 7-Tier Review

## Scope

- Issue: #116 `test: Add Redis NearCache failure-injection coverage`
- Diff base: `origin/develop`
- Reviewed files:
  - `cache/redisnear/failure_injection_test.go`
  - `README.md`
  - `README.ko.md`
  - `docs/lessons/2026-06-04-redis-near-cache-failure-injection.md`

## Integrated Verdict

| Severity | Count | Verdict |
|---|---:|---|
| P0 / CRITICAL | 0 | PASS |
| P1 / HIGH | 0 | PASS |
| P2 / MEDIUM | 0 | PASS |
| P3 / LOW | 0 | PASS |

Gate verdict: PASS. P0 = 0 and P1 = 0.

## Tier Findings

| Tier | Focus | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| 1 Security | Secrets, ACL/TLS, message trust boundary | 0 | 0 | 0 | 0 | README keeps Pub/Sub as invalidation command, not auth boundary; no secrets or new inputs added. |
| 2 Ops/SRE reliability | Redis outage, terminal failure, operator signal | 0 | 0 | 0 | 0 | Outage test forces Redis termination, waits for `OnError`, and verifies local cache miss; docs state recreate after terminal failure/restart. |
| 3 Structural impact | Public API, package boundaries, dependency impact | 0 | 0 | 0 | 0 | No production API or dependencies changed; helper stays package-private in tests. |
| 4 Go/code quality | Idioms, cleanup, shared helper reuse | 0 | 0 | 0 | 0 | Tests reuse existing `waitForRedis` and `assertEventuallyMiss`; no dead helper remains. |
| 5 Tests/types/silent failure | Assertion strength, flake risk, failure path proof | 0 | 0 | 0 | 0 | Tests assert both outage local clear and recreate peer invalidation; `OnError` counter prevents silent receive-loop failure. |
| 6 Performance/stability | Runtime overhead, race, Testcontainers lifecycle | 0 | 0 | 0 | 0 | Tests are integration-only; container termination guarded by `sync.Once`; no production hot path changed. |
| 7 Docs/release/evidence | README locale parity, lessons, release risk | 0 | 0 | 0 | 0 | English/Korean README guidance updated in sync; lessons file captures durable operational rule. |

## Validation Evidence

- `go test -count=1 ./cache/redisnear -run "RedisOutage|RecreateAfterRedisOutage"`:
  PASS (`ok github.com/bluetape4k/bluetape-go/cache/redisnear 1.562s`)
- `go test -count=1 ./cache/redisnear`: PASS
  (`ok github.com/bluetape4k/bluetape-go/cache/redisnear 3.427s`)
- `go test -race -count=1 ./cache/redisnear`: PASS
  (`ok github.com/bluetape4k/bluetape-go/cache/redisnear 4.115s`)
- `go test -count=1 ./cache ./cache/redisnear`: PASS
  (`cache 0.380s`, `cache/redisnear 3.356s`)
- `git diff --check`: PASS
- `make ci`: PASS

## Review Notes

- The tests intentionally do not claim automatic Redis resubscribe/reconnect as
  a production guarantee. They prove a safe recreate path after terminal Redis
  outage, which matches the README operational contract.
- RESP3 client tracking remains tracked by #110 and is out of scope for this
  Pub/Sub failure-injection PR.
