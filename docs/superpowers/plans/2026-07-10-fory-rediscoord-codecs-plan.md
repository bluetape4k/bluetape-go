# Go-native Apache Fory Rediscoord Codecs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add opt-in Go-native Apache Fory fast and compatible codecs for the inner `cache/rediscoord` result payload while preserving the existing coordination protocol.

**Architecture:** Add an isolated `cache/rediscoord/fory` package that owns Fory v1.3.0 configuration, a `BTFY` profile envelope, one mutex-guarded registered runtime, bounded decode/encode, and sanitized typed errors. Add an additive `MaxResultBytes` guard to `rediscoord` so the existing JSON/base64 outer envelope can be bounded before JSON allocation; keep JSON as the default and do not add a direct Redis value cache in this issue.

**Tech Stack:** Go 1.26, Apache Fory Go `v1.3.0`, `encoding/binary`, `sync.Mutex`, `go-redis/v9`, Go test/race, existing Testcontainers Redis fixtures.

---

### Task 1: Pin Apache Fory and establish package boundaries

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `cache/rediscoord/fory/doc.go`

- [ ] **Step 1: Add the dependency and package documentation.**

Add `github.com/apache/fory/go/fory v1.3.0` as a direct dependency. Document that
the package is Go-native, `TRUSTED_INTERNAL`, not encryption, and separate from
xlang interoperability and direct Redis value storage.

- [ ] **Step 2: Run the package compile gate.**

Run:

```bash
go test ./cache/rediscoord/fory
```

Expected: the package compiles; it may report no test files until Task 2 adds
the first failing test.

- [ ] **Step 3: Commit the dependency boundary.**

```bash
git add go.mod go.sum cache/rediscoord/fory/doc.go
git commit -m "chore: pin Apache Fory for Redis codecs"
```

### Task 2: Define typed errors and constructor validation through failing tests

**Files:**
- Create: `cache/rediscoord/fory/codec_test.go`
- Create: `cache/rediscoord/fory/errors.go`
- Create: `cache/rediscoord/fory/codec.go`

- [ ] **Step 1: Write failing constructor and zero-value tests.**

Cover:

```go
func TestNewNativeFastRejectsInvalidOptions(t *testing.T)
func TestNewNativeFastRejectsUnsupportedRootShapes(t *testing.T)
func TestZeroCodecReturnsUninitializedError(t *testing.T)
func TestCodecErrorRedactsPayloadCauseAndRegistrationText(t *testing.T)
```

Use a registered struct fixture, pointer/interface/function roots, zero and
negative limits, missing registration, and a registration callback returning an
error containing a secret marker. Assert `errors.As` exposes `CodecError`, the
reason is stable, and `Error()` excludes payload, registration, Fory-cause, key,
and owner-token markers.

- [ ] **Step 2: Run the tests and verify the expected RED state.**

Run:

```bash
go test ./cache/rediscoord/fory -run 'Test(NewNativeFastRejects|ZeroCodec|CodecError)'
```

Expected: FAIL because the package API and error implementation do not exist.

- [ ] **Step 3: Implement the typed error and immutable constructor skeleton.**

Define exported `CodecError` with `Operation()`, `Profile()`, `Reason()`,
`Error()`, and safe `Unwrap()`. Define `Options`, profile constants, validation
for all six positive limits, root shape validation, and constructor-only codec
state. A zero-value codec must return an `uninitialized` error.

- [ ] **Step 4: Run the focused tests and verify GREEN.**

```bash
go test ./cache/rediscoord/fory -run 'Test(NewNativeFastRejects|ZeroCodec|CodecError)'
```

Expected: PASS with no raw cause or marker leakage.

### Task 3: Implement native Fory runtime and `BTFY` envelope

**Files:**
- Modify: `cache/rediscoord/fory/codec.go`
- Modify: `cache/rediscoord/fory/codec_test.go`
- Create: `cache/rediscoord/fory/envelope.go`
- Create: `cache/rediscoord/fory/envelope_test.go`

- [ ] **Step 1: Write failing round-trip and envelope tests.**

Cover native-fast and native-compatible profiles with a registered struct,
primitive scalar, string, and `[]byte`. Assert the `BTFY` magic, version,
profile, length, exact round trip, and compatible added-field behavior. Add
wrong-profile/version/magic, raw Fory, JSON, truncation, trailing bytes,
length mismatch, and oversize tests.

- [ ] **Step 2: Run the tests and verify RED.**

```bash
go test ./cache/rediscoord/fory -run 'Test(Native|Envelope|RoundTrip|Rejects)'
```

Expected: FAIL on missing constructors, envelope, and codec methods.

- [ ] **Step 3: Implement the mutex-guarded Fory runtime.**

Construct `fory.New` with explicit `WithXlang(false)`, profile-specific
`WithCompatible`, `WithTrackRef(false)`, `WithMaxDepth`, and all explicit type
metadata limits. Apply registration once before returning. `Marshal` locks,
serializes, rejects payloads above `MaxPayloadBytes`, copies Fory bytes, and
unlocks before building one `BTFY` wrapper allocation. `Unmarshal` validates
the wrapper and max size without copying its payload, locks only around Fory
decode, and returns a sanitized `CodecError` on every failure.

- [ ] **Step 4: Run focused tests and verify GREEN.**

```bash
go test ./cache/rediscoord/fory -run 'Test(Native|Envelope|RoundTrip|Rejects)'
```

Expected: PASS for both profiles, value shapes, limits, and fail-closed
envelope behavior.

- [ ] **Step 5: Commit the codec implementation.**

```bash
git add cache/rediscoord/fory
git commit -m "feat: add native Fory rediscoord codecs"
```

### Task 4: Add concurrency and compile-checked examples

**Files:**
- Modify: `cache/rediscoord/fory/codec_test.go`
- Create: `cache/rediscoord/fory/example_test.go`

- [ ] **Step 1: Write failing concurrency and examples.**

Add a high-contention test that concurrently marshals and unmarshals distinct
registered values through one codec and asserts every result. Add
`ExampleNewNativeFast` and `ExampleNewNativeCompatible` with deterministic
registration and output-free compile-checked usage.

- [ ] **Step 2: Run the race gate.**

```bash
go test -race ./cache/rediscoord/fory
```

Expected: initially RED before the runtime is complete; after implementation,
PASS with no data races or buffer reuse corruption.

- [ ] **Step 3: Verify normal package tests.**

```bash
go test ./cache/rediscoord/fory
```

Expected: PASS, including examples.

### Task 5: Bound the outer `rediscoord` JSON envelope

**Files:**
- Modify: `cache/rediscoord/options.go`
- Modify: `cache/rediscoord/stampede_cache.go`
- Modify: `cache/rediscoord/stampede_cache_test.go`
- Modify: `cache/rediscoord/operation_error_test.go`

- [ ] **Step 1: Write failing outer-limit tests.**

Add tests proving `Options.MaxResultBytes` accepts zero as unlimited, rejects
negative values, rejects oversized result bytes before Redis publication, and
rejects oversized Redis result bytes before `json.Unmarshal`. Assert existing
operation/context error behavior remains intact.

- [ ] **Step 2: Run the focused tests and verify RED.**

```bash
go test ./cache/rediscoord -run 'Test.*ResultBytes|Test.*OperationError'
```

Expected: FAIL because `MaxResultBytes` does not exist.

- [ ] **Step 3: Implement the additive guard.**

Normalize `MaxResultBytes` as zero-or-positive. Check encoded result length in
`storeResult` before `client.Set`, and check raw Redis bytes in
`readOwnerResult` before `decodeResult`. Return a sanitized typed operation
error using existing `btredis.OpError`; preserve zero-default behavior.

- [ ] **Step 4: Run focused and full package tests.**

```bash
go test ./cache/rediscoord -run 'Test.*ResultBytes|Test.*OperationError'
go test -p 1 ./cache/rediscoord
```

Expected: PASS with existing lock, TTL, context, redaction, and coordination
tests unchanged in behavior.

- [ ] **Step 5: Commit the outer guard.**

```bash
git add cache/rediscoord/options.go cache/rediscoord/stampede_cache.go cache/rediscoord/stampede_cache_test.go cache/rediscoord/operation_error_test.go
git commit -m "feat: bound rediscoord result envelopes"
```

### Task 6: Documentation and rollout runbook

**Files:**
- Modify: `cache/rediscoord/README.md`
- Modify: `cache/rediscoord/README.ko.md`
- Modify: `cache/rediscoord/doc.go`

- [ ] **Step 1: Add English and Korean usage documentation.**

Document both constructors, explicit native mode, supported root shapes,
registration-before-concurrency, the six resource limits and starting values,
`CodecError` reason labels, `MaxResultBytes`, `TRUSTED_INTERNAL`, and the fact
that Redis can observe bytes because Fory is not encryption.

- [ ] **Step 2: Add the rollout/rollback runbook.**

Show a namespace format containing profile and schema generation. State that
all processes sharing a namespace must use one codec/profile/registration set;
mixed JSON/Fory and fast/compatible deployments are unsupported. Document
drain time `LockTTL + ResultTTL + safety margin`, rollback to the prior
codec/namespace pair, bounded `SCAN MATCH` cleanup, and no `KEYS`.

- [ ] **Step 3: Verify documentation examples against examples.**

```bash
go test ./cache/rediscoord/fory -run '^Example'
git diff --check
```

Expected: PASS; README code matches compile-checked examples and contains no
unsupported fallback or interoperability claim.

### Task 7: Final verification and review preparation

**Files:**
- No new files; review all changed files and issue-linked docs.

- [ ] **Step 1: Run focused unit and race checks.**

```bash
go test -p 1 ./cache/rediscoord/fory
go test -p 1 -race ./cache/rediscoord/fory
go test -p 1 ./cache/rediscoord
go test -p 1 -race ./cache/rediscoord
git diff --check
```

- [ ] **Step 2: Run repository gates.**

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
```

Run Testcontainers-backed package checks serially and use the repository's
standard `make ci` gate before PR creation. Do not claim benchmark improvement;
#599 owns benchmark table, Chart, and analysis.

- [ ] **Step 3: Review the final diff against #597 and the spec.**

Confirm no xlang/default codec/persistence migration slipped into the diff,
all public APIs have Go doc comments, both README files are synchronized, and
all P0/P1 review findings are zero before opening a PR.
