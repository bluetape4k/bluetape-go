# Task 9 Review: Redis Distributed JWT Documentation and Examples

Issue: #173
Plan: `docs/superpowers/plans/2026-06-12-issue-173-distributed-jwt-keychain-repositories-plan.md`
Scope: T9 examples, README guidance, operator runbook, and benchmark chart references.

## Result

Gate: PASS

- P0: 0
- P1: 0
- P2: 0

## Reviewed Artifacts

- `jwt/redis/example_test.go`
- `jwt/README.md`
- `jwt/README.ko.md`
- `docs/images/readme-charts/distributed-jwt-redis-benchmark.svg`
- `docs/images/readme-charts/distributed-jwt-redis-benchmark.vl.json`
- `docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt`

## Verification Evidence

```bash
gofmt -w jwt/redis/example_test.go
go test -count=1 ./jwt/redis -run Example
```

Result:

```text
ok  	github.com/bluetape4k/bluetape-go/jwt/redis	0.393s [no tests to run]
```

The examples compile as package-level examples without `// Output` assertions, so
they document caller-owned Redis clients, explicit contexts, distributed HMAC/RSA
constructors, and context timeout usage without requiring a live Redis instance
during ordinary documentation tests.

```bash
rg -n "DistributedProvider|ComposeContext|ParseContext|MongoDB|#198|Redis|signing authority|DeleteKeyChainsContext|Kotlin|wire-compatible|token invalidation|redis-cli --tls|ACL|PTTL|JWKS|JWE|OIDC|auth middleware|background rotation|noeviction|ErrKeyNotFound|ErrInvalidKey" jwt/README.md
rg -n "DistributedProvider|ComposeContext|ParseContext|MongoDB|#198|Redis|signing authority|DeleteKeyChainsContext|Kotlin|wire-compatible|token invalidation|redis-cli --tls|ACL|PTTL|JWKS|JWE|OIDC|auth middleware|background rotation|noeviction|ErrKeyNotFound|ErrInvalidKey" jwt/README.ko.md
git diff --check
```

Evidence confirmed both README variants cover:

- Redis-backed distributed providers with `ComposeContext` and `ParseContext`.
- MongoDB deferral to #198.
- Go-owned Redis key format and no Kotlin/JVM wire compatibility.
- Fixed/local token continuity limitation and explicit invalidation decision.
- `DeleteKeyChainsContext` as test/operator reset only.
- Unsupported capabilities: JWKS, JWE, OIDC, auth middleware, sessions, roles,
  and background rotation timers.
- Operator diagnostics with the exact `redis-cli --tls` commands for `meta`,
  `current`, `keys`, `order`, and `PTTL`.
- TLS, ACL, persistence/backups, `noeviction`, retained-key eviction risk,
  least-privilege namespace ACLs, no shared untrusted Redis, no cross-tenant
  namespace reuse, outage/deadline ownership, namespace diagnostics,
  rollback/reset guidance, and secret-safe logging.

`git diff --check` produced no whitespace errors.

## Notes

The benchmark section links to the SVG chart and raw output instead of restating
all numbers inline. This keeps README guidance readable while preserving the
chart-backed evidence requested for benchmark results.
