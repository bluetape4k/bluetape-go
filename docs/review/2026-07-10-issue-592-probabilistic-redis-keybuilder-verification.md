# Issue #592 Probabilistic Redis Key Builder Verification

Date: 2026-07-10 KST
Gate: Step 5 verifier
Baseline: `068df42615303090be1f57e03a494b596962e8e7`

## Requirement Evidence

| Spec invariant | Evidence | Verdict |
|---|---|---|
| Bloom key bytes remain exact | `TestBuildKeysUsesClusterHashTag` plus adapter `StructuralKey` output; focused targeted test passed. | PASS |
| HLL key bytes remain exact | `TestBuildHyperLogLogKeyKeepsSharedBuilderCompatibleLayout`; focused targeted test passed. | PASS |
| Colons remain valid in hash tag | `TestKeyBuilderForNamespaceKeepsClusterHashTag` uses `tenant-a:emails` and exact `:bits` key. | PASS |
| Local validation remains first | `keyBuilderForNamespace` calls `validateNamespace` before builder construction; invalid-input regression test passed. | PASS |
| Short local redacted ID remains | Existing `redactedRedisKeyID` is unchanged; new HLL marker test requires `redis-key:[0-9a-f]{12}` with no raw marker/key leak. | PASS |
| `RedisError` and metadata sentinels remain | `errors.go` is untouched; focused package tests include provider behavior and configuration metadata cases. | PASS |
| Scripts, commands, algorithms remain | Diff only changes `keys.go` and `options_test.go`; no Lua/filter/HLL command source changed. | PASS |
| No public API or README behavior changes | No exported identifiers or README files changed; `git diff --stat` confirms two implementation/test files only before this artifact. | PASS |

## Fresh Validation Evidence

| Command | Result |
|---|---|
| `go test -count=1 ./probabilistic/redis -run 'KeyBuilderForNamespace|KeepsSharedBuilderCompatibleLayout'` before adapter | Expected RED: `undefined: keyBuilderForNamespace`. |
| Same focused key/validation/error suite after adapter | PASS. |
| `make fmt-check` | PASS. |
| `make tidy-check` | PASS. |
| `go vet ./probabilistic/redis ./redis` | PASS. |
| `go test -p 1 -count=1 ./probabilistic/redis ./redis` | PASS. |
| `go test -p 1 -race -count=1 ./probabilistic/redis` | PASS. |
| `git diff --check` | PASS. |

## Boundary Verdict

The adapter maps any fixed-prefix, hash-tag, or structural-key builder failure
to a local opaque configuration error without wrapping shared `redis` errors.
For caller input, local `validateNamespace` remains the first and observable
validation contract. The existing package-specific redaction and `RedisError`
surface are unchanged.

## Remaining Required Gate

Run Step 6/6-R on the implementation diff, then full repository CI with
`TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false` before
publication. Benchmark evidence remains N/A; issue #560 owns any required
cross-provider table, chart, and analysis.
