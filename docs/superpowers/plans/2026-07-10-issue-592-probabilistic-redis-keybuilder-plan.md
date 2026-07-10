# Probabilistic Redis Shared Key Builder Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reuse the shared Redis structural key builder in `probabilistic/redis`
without changing stored-key bytes, local namespace validation, or the provider's
public redacted error contract.

**Architecture:** Keep the probabilistic package as the owner of namespace
validation and its short redacted key ID. Add private helpers that create a
shared `redis.KeyBuilder` only after local validation and convert any impossible
fixed-configuration builder failure into an opaque local error. Use the helper
for Bloom slot/bits/config and HyperLogLog keys; retain every caller-visible
error mapping and Redis operation unchanged.

**Tech Stack:** Go, `github.com/bluetape4k/bluetape-go/redis`,
`github.com/redis/go-redis/v9`, Testcontainers Redis, standard library.

---

## File Map

| File | Responsibility |
|---|---|
| `probabilistic/redis/keys.go` | Use shared structural key construction behind local validation and local opaque configuration errors. |
| `probabilistic/redis/options_test.go` | Lock Bloom/HLL key bytes, invalid-input boundary, and short local redaction ID. |
| `docs/lessons/2026-07-10-issue-592-probabilistic-redis-keybuilder.md` | Record why a shared construction helper does not imply a shared public error or redaction contract. |
| `docs/review/2026-07-10-issue-592-probabilistic-redis-keybuilder-review.md` | Step 6-R integrated review evidence. |

## Task 0: Commit Approved Design Artifacts

**Complexity:** low

**Files:** Commit the two specs, Step 2-R review, this plan, and Step 3-R
review before source or test implementation begins.

- [ ] **Step 1:** Run `git diff --check` and verify only the five planning
  artifacts are staged.
- [ ] **Step 2:** Commit them with Lore trailers. The intent is to preserve the
  compatible shared-construction boundary; record that shared error/redaction
  adoption was rejected, confidence is high, scope risk is narrow, and source
  validation is not yet run.
- [ ] **Step 3:** Run `git status --short` and require a clean worktree before
  the RED test step.

## Task 1: RED Exact-Key And Error-Boundary Tests

**Complexity:** low

**Files:** Modify `probabilistic/redis/options_test.go`; read
`probabilistic/redis/keys.go`, `probabilistic/redis/options.go`, and
`redis/key.go`.

**Patterns:** Apply `$bluetape-go-patterns`: table-driven exact contract tests,
no new dependencies, no parallel Testcontainers work, and preserve
`errors.Is`/`errors.As` behavior. This change owns no concurrency contract, so
existing serial provider tests and the repository race gate are sufficient;
no new stress helper is applicable.

- [ ] **Step 1:** Add `TestKeyBuilderForNamespaceKeepsClusterHashTag`. It must
  call the new private adapter directly, so the current source fails to compile
  until the shared-builder migration exists:

  ```go
  builder, err := keyBuilderForNamespace(keyPrefix, "tenant-a:emails")
  if err != nil {
      t.Fatalf("keyBuilderForNamespace failed: %v", err)
  }
  key, err := builder.StructuralKey("bits")
  if err != nil {
      t.Fatalf("StructuralKey failed: %v", err)
  }
  if got, want := key.Value, "bluetape:probabilistic:bloom:v1:{tenant-a:emails}:bits"; got != want {
      t.Fatalf("key = %q, want %q", got, want)
  }
  ```

- [ ] **Step 2:** Add `TestBuildKeysKeepsSharedBuilderCompatibleLayout` and
  `TestBuildHyperLogLogKeyKeepsSharedBuilderCompatibleLayout` with exact
  Bloom slot/bits/config values and HLL expected value
  `bluetape:probabilistic:hll:v1:{tenant-a:emails}` and a marker namespace
  check that `redactedID` has the existing `redis-key:[0-9a-f]{12}` shape and
  contains neither marker text nor the full key.
- [ ] **Step 3:** Add `TestKeyBuilderForNamespaceRetainsLocalValidation` with
  `tenant:{bad}`, whitespace-only, and `tenant-secret`; require an error and
  reject error strings containing `redis: invalid key` or `invalid hash tag`.
- [ ] **Step 4:** Run
  `go test -count=1 ./probabilistic/redis -run 'KeyBuilderForNamespace|KeepsSharedBuilderCompatibleLayout'`.
  Expected: RED with `undefined: keyBuilderForNamespace` after the tests are
  added and before Task 2 implements the adapter.

## Task 2: Minimal Shared-Builder Adapter

**Complexity:** medium

**Files:** Modify `probabilistic/redis/keys.go`; test
`probabilistic/redis/options_test.go`.

**Patterns:** Apply `$bluetape-go-patterns`: retain local public error behavior,
use idiomatic private helpers, do not wrap shared validation errors through a
caller-visible path, and avoid changes to scripts, commands, or exported APIs.

- [ ] **Step 1:** Import `github.com/bluetape4k/bluetape-go/redis` as
  `btredis`. Add `keyBuilderForNamespace(prefix, namespace string)`; it calls
  `validateNamespace(namespace)`, creates `btredis.NewKeyBuilder(prefix)`, applies
  `builder.WithHashTag(namespace)`, and maps every non-nil builder error to a
  local opaque error such as `fmt.Errorf("probabilistic redis key configuration")`.
  Do not wrap the shared error.
- [ ] **Step 2:** Add a private helper that calls
  `builder.StructuralKey(parts...)`, returns `key.Value`, and maps a non-nil
  error to the same local opaque error. The caller must not observe a shared
  `ErrInvalidKey` or `ErrInvalidHashTag` cause.
- [ ] **Step 3:** Rewrite `buildKeys` to derive `slot`, `bits`, and `config`
  through the adapter and `StructuralKey`, then retain
  `redactedRedisKeyID(slot)`. Rewrite `buildHyperLogLogKey` through the same
  adapter and retain its local `redactedRedisKeyID(key)`.
- [ ] **Step 4:** Do not modify `errors.go`, Bloom Lua scripts, filter/HLL
  command calls, configuration metadata, or README files.
- [ ] **Step 5:** Run
  `gofmt -w probabilistic/redis/keys.go probabilistic/redis/options_test.go`
  followed by
  `go test -p 1 -count=1 ./probabilistic/redis -run 'KeepsSharedBuilderCompatibleLayout|BuildKeysUsesClusterHashTag|UnsafeNamespaces|RedisError'`.
  Expected: PASS.

## Task 3: Focused Contract Verification

**Complexity:** medium

**Files:** Verify `probabilistic/redis/{keys.go,options_test.go,filter_test.go,hyperloglog_test.go,config_test.go}` and `redis/key.go`.

- [ ] **Step 1:** Run `make fmt-check`, `make tidy-check`, and
  `go vet ./probabilistic/redis ./redis`.
- [ ] **Step 2:** Run serial provider tests:

  ```bash
  go test -p 1 -count=1 ./probabilistic/redis ./redis
  go test -p 1 -race -count=1 ./probabilistic/redis
  ```

  Expected: PASS. The package uses Testcontainers, so no package test command
  runs in parallel with another Docker-backed command.
- [ ] **Step 3:** Run `git diff --check` and inspect `git diff --stat`.
  Confirm no documentation or benchmark artifact changed because public behavior
  and performance claims did not change.

## Task 4: Verifier, Review, Lesson, And Full CI

**Complexity:** medium

**Files:** Create `docs/review/2026-07-10-issue-592-probabilistic-redis-keybuilder-verification.md`,
`docs/review/2026-07-10-issue-592-probabilistic-redis-keybuilder-review.md`,
and `docs/lessons/2026-07-10-issue-592-probabilistic-redis-keybuilder.md`.

- [ ] **Step 1:** Before code review, run the Step 5 verifier. It must inspect
  the implementation diff and focused test outputs, confirm every spec
  invariant has evidence, and explicitly verify local errors/redaction remain
  the caller-visible contract.
- [ ] **Step 2:** Run Step 6 local six-perspective review and record
  performance, stability, security, operator/Ops, developer/API, and
  user/caller evidence. Require `P0=0 P1=0`; explicitly reject changing
  `RedisError`, `redactedRedisKeyID`, validation, scripts, and benchmark scope.
- [ ] **Step 3:** Write the lesson: shared Redis key construction can be
  compatible while shared validation/error/redaction contracts are not. Record
  the exact-key rule and #560 benchmark ownership.
- [ ] **Step 4:** Run the full gate serially:

  ```bash
  TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci
  git diff --check
  git status --short
  ```

  Expected: PASS. Retain these explicit Testcontainers overrides only for the
  command; do not mutate machine configuration.

## Rollback

Revert the implementation commit. Stored Redis keys, scripts, state, public
API, and configuration remain unchanged, so no data migration or cleanup is
needed.
