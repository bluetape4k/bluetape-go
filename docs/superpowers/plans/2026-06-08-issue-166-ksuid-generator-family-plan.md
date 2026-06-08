# Issue #166 KSUID Generator Family Plan

## Scope

Implement standard seconds-precision KSUID support in the existing `id` package.
Millisecond-precision KSUID is deferred to #171.

## Inputs

- Issue #166: `Port KSUID generator family`.
- Spec:
  `docs/superpowers/specs/2026-06-08-issue-166-ksuid-generator-family-spec.md`.
- Spec review:
  `docs/superpowers/reviews/2026-06-08-issue-166-ksuid-generator-family-spec-review.md`.
- Go dependency source: `github.com/segmentio/ksuid@v1.0.4`.
- Kotlin source parity:
  `bluetape4k-projects/utils/idgenerators/src/main/kotlin/io/bluetape4k/idgenerators/ksuid`.

## Implementation Steps

### 1. Dependency

- Add `github.com/segmentio/ksuid v1.0.4` to `go.mod` with `go get`.
- Keep it as a direct dependency because public `id` implementation depends on
  it.
- Do not expose `ksuid.KSUID` in public bluetape-go API.

### 2. `id/ksuid.go`

Add unexported implementation:

```go
type ksuidGenerator struct {
    entropy io.Reader
    now     func() time.Time
}
```

Public API:

```go
type KSUIDOption func(*ksuidGenerator) error

func WithKSUIDEntropy(entropy io.Reader) KSUIDOption
func WithKSUIDTime(now func() time.Time) KSUIDOption
func NewKSUIDGenerator(options ...KSUIDOption) (StringGenerator, error)
func NewKSUID() (string, error)
func ParseKSUID(value string) (string, error)
func KSUIDTime(value string) (time.Time, error)
```

Generation:

- Default entropy: `crypto/rand.Reader`.
- Default clock: `time.Now`.
- Validate nil generator, nil option, nil entropy, and nil clock consistently
  with UUID/ULID patterns.
- Capture `now := g.now()` and validate `now.Unix() - 1400000000` is within the
  standard KSUID `0..math.MaxUint32` seconds-offset range before entropy read or
  Segment `FromParts`.
- Read exactly 16 bytes via `io.ReadFull`.
- Use `segmentio/ksuid.FromParts(now, payload)` to avoid global
  `ksuid.SetRand` and package-level random state.
- Return `value.String()`.

Parsing/time:

- Use `segmentio/ksuid.Parse`.
- Return `parsed.String()` from `ParseKSUID`.
- Reject any input where `value != parsed.String()`.
- Return `parsed.Time().UTC()` from `KSUIDTime`.
- Wrap parse failures with `ParseError{Kind: "ksuid", ...}`.

### 3. Tests

Add `id/ksuid_test.go`.

Required cases:

- `TestKSUIDGeneratesAndParses`:
  - `NewKSUID()`.
  - Length is 27.
  - `ParseKSUID` returns canonical string.
  - `KSUIDTime` is non-zero UTC time.
- `TestKSUIDGeneratorUsesInjectedTimeAndEntropy`:
  - Use fixed time and deterministic 16-byte entropy.
  - Generate one KSUID.
  - Parse with `segmentio/ksuid.Parse` inside the test.
  - Assert timestamp equals fixed time at second precision.
  - Assert payload equals the fixed 16 bytes.
  - Assert returned string has fixed 27-character canonical Base62 form.
- `TestKSUIDSortsByTimestamp`:
  - Use deterministic entropy readers and two different timestamps.
  - Assert lexicographic order follows timestamp order.
  - Do not assert same-second monotonicity.
- `TestKSUIDRejectsInvalidInput`:
  - Empty, too short, too long, invalid alphabet, and non-canonical casing/value.
  - Out-of-range valid-Base62 input, ideally a 27-character value just above the
    Segment maximum encoded KSUID.
  - Check `errors.Is(err, ErrInvalidID)`.
- `TestKSUIDOptionsRejectNil`:
  - Nil option, nil entropy, nil clock.
- `TestKSUIDGeneratorRejectsOutOfRangeTime`:
  - Before KSUID epoch returns no ID and `ErrInvalidOptions`.
  - Beyond maximum 32-bit seconds offset returns no ID and `ErrInvalidOptions`.
- `TestKSUIDWrapsEntropyFailure`:
  - Short/failing reader returns `EntropyError` and wraps the causal error.

Update `id/id_concurrency_test.go`:

- Add `ksuid` to `TestGUIDGeneratorsStayUniqueAcrossGoroutines`.
- Use shared `NewKSUIDGenerator()` instance.
- Keep normal and race validation.

Update `id/id_example_test.go`:

- Add a small `ExampleNewKSUID` or KSUID usage line if it does not make the
  examples noisy.

Update `id/id_benchmark_test.go`:

- Add `BenchmarkKSUIDNextString`.
- Create one `NewKSUIDGenerator()` outside the benchmark loop and measure
  repeated `NextString()` calls, failing the benchmark on returned errors.
- Optionally add `BenchmarkKSUIDNextStringParallel` only if the implementation
  or docs make a parallel-throughput claim.

### 4. Documentation

Update:

- `id/README.md`.
- `id/README.ko.md`.
- root `README.md`.
- root `README.ko.md`.
- `CHANGELOG.md`.
- `WIP.md`.

Required wording:

- KSUID is standard seconds-precision Segment-compatible KSUID.
- It is a 27-character URL-safe Base62 string.
- It is useful for copy/paste-friendly time-sortable string IDs without
  Snowflake machine coordination.
- Wall-clock rollback can weaken ordering; do not treat it as strict monotonic
  sequence.
- KSUID millis is deferred to #171.
- Custom entropy readers and custom clock funcs must be concurrency-safe when a
  generator is shared.

### 5. Verification

Run:

```bash
gofmt -w id/ksuid.go id/ksuid_test.go id/id_concurrency_test.go id/id_example_test.go id/id_benchmark_test.go
git diff --check
go test -count=1 ./id
go test -race -count=1 ./id
go test -count=1 ./id -run 'TestKSUID|TestGUIDGeneratorsStayUniqueAcrossGoroutines|TestGeneratorsAreConcurrentSafe' -v
go test -race -count=1 ./id -run 'TestKSUID|TestGUIDGeneratorsStayUniqueAcrossGoroutines|TestGeneratorsAreConcurrentSafe' -v
go test -run '^$' -bench . -benchmem ./id
go test -count=1 ./...
make ci
```

### 6. Review

- Run Step 6-R 7-Tier review after implementation.
- Use subagent lanes for at least:
  - code/API correctness.
  - dependency/source parity.
  - test/concurrency/race proof.
  - docs/release/evidence integrity.
- P0/P1 must be zero before PR creation.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Accidentally using global `ksuid.SetRand` leaks test entropy globally. | Use `ksuid.FromParts` with generator-local payload only. |
| Caller mistakes KSUID for strict monotonic sequence. | README and tests state timestamp sorting only, not same-second monotonicity. |
| Custom entropy or clock is not concurrency-safe. | Document caller responsibility and prove default generator with stress/race tests. |
| Millis source parity is overclaimed. | Keep millis out of #166 and link #171. |
| Dependency API leaks into stable surface. | Public APIs return only `string`, `time.Time`, and repo-owned errors/options. |

## DoD

- `id` package exposes standard KSUID seconds generation, parse, and time
  extraction.
- KSUID docs replace the old full-defer wording and link #171 for millis.
- Tests, race tests, benchmarks, full repo tests, and `make ci` pass.
- Step 6-R integrated review has `P0=0 P1=0`.
- PR metadata matches issue #166.
