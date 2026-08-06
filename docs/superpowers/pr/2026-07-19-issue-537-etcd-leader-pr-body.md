Fixes #537.

## Summary

- Add an etcd v3 leader-election provider backed by the official
  `concurrency.Session` and `concurrency.Election` primitives.
- Fail closed on lease, exact-key watch, Proclaim, cancellation, response-loss,
  cleanup, and caller-owned client shutdown boundaries.
- Add digest-pinned real-etcd conformance, authorization, contention, resource,
  hard-stop, and race coverage.
- Publish English/Korean usage, capacity, TLS/RBAC, migration, rollback, and
  release-runbook guidance.
- Raise the active dependency and rollback security floor to
  `x/crypto v0.52.0` and `x/net v0.55.0`.

## Contract

- Callers retain ownership of `*clientv3.Client`.
- Every Campaign creates a provider-owned Session; no raw Session options,
  session adoption, restart-resume, or fencing token API is exposed.
- `Campaign` is synchronous. Callers receive an executable `IsLeader` guard
  for every protected work unit and must stop and join work before reacquiring.
- Cleanup inventory clears only after a separate healthy client proves exact
  candidate-range absence and zero etcd contenders.
- Mutually untrusted tenants require separate etcd clusters.

## Review

- Step 2-R design review, Step 3-P risk prediction, Step 3-R plan review, and
  Step 6-R 7-tier review artifacts are included under `docs/superpowers/`.
- Step 6-R exact implementation head:
  `f5d24a83b08777cced3ede65c755af061417556b`.
- Step 6-R verdict: `P0=0 P1=0 P2=0 P3=1`.
- The accepted P3 is the unused, module-only `x/crypto/openpgp` advisory
  `GO-2026-5932`; there is no import/call path and no fixed module version.

## Validation

- PASS `make ci` on the reviewed implementation head — 578 seconds.
- PASS full `leader/leadertest` and `leader/etcd` normal and race suites.
- PASS real etcd `v3.6.13` conformance, authorization, 32-contender resource,
  cleanup, and hard-stop tests.
- PASS supervisor rollback ordering tests, including proof success, proof
  failure, and zero-contender failure.
- PASS `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...` with zero
  reachable and zero imported-package vulnerabilities.
- PASS `make fmt-check`, `make tidy-check`, `make vet`, and `make lint`.
- PASS English/Korean documentation and runbook contract checks.
- PENDING GitHub CI on the published PR head.

## DoD Status

- [x] etcd leader provider and constructor-only public contract implemented.
- [x] Exact ownership-loss and proof-gated cleanup semantics implemented.
- [x] Caller-owned client shutdown and rollback ordering made bounded and
  fail-closed.
- [x] Real-server, contention, authorization, failure, normal, and race tests
  added.
- [x] Capacity, TLS/RBAC, observability, migration, rollback, and bilingual docs
  added.
- [x] Type A reusable lesson committed.
- [x] Step 6-R six-lane review completed with P0=0 P1=0 P2=0.
- [x] Final local gates completed.
- [ ] GitHub CI and Step 7-R exact-PR-head review pending.
- [ ] Merge requires fresh explicit approval after all remote gates pass.
