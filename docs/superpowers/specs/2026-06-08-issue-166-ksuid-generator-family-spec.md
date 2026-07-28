# Issue #166 KSUID Generator Family Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 맥락

Issue #166 extends the `id` package after the issue #32 foundation was merged in
PR #169. The current `id` package exposes narrow Go-native generator contracts:

- `StringGenerator` with `NextString() (string, error)`.
- `Int64Generator` with `NextInt64() (int64, error)`.
- UUID v4/v7, random ULID, monotonic ULID, and Snowflake implementations with
  unexported concrete generator types.

The bluetape4k source parity target is `bluetape4k-idgenerators` KSUID support:

- `Ksuid.Seconds`: 20 bytes, 4-byte timestamp, 16-byte random payload, 27-char
  Base62 string.
- `Ksuid.Millis`: 20 bytes, 8-byte timestamp, 12-byte random payload, 27-char
  Base62 string.
- `KsuidGenerator`: adapter over the selected KSUID strategy.
- Edge tests around 20-byte decoded shape, payload length, timestamp extraction,
  invalid encodings, and multi-thread uniqueness.

The preferred Go dependency, `github.com/segmentio/ksuid` v1.0.4, directly
supports standard seconds-precision KSUIDs. Its local source shows:

- Epoch offset `1400000000`.
- Binary shape `4-byte timestamp + 16-byte payload`.
- Fixed string length `27`.
- `NewRandomWithTime`, `FromParts`, `Parse`, `Time`, `Timestamp`, `Payload`,
  `Compare`, `Sort`, `IsSorted`, `Next`, and `Prev`.
- Package-level generation uses a global mutex and global random reader.

The dependency does not provide the bluetape4k millisecond variant. That variant
is a bluetape4k-specific 20-byte layout, not the canonical Segment KSUID shape.

## 목표s

1. Add standard KSUID seconds support to `id` with the same narrow API style as
   UUID/ULID/Snowflake.
2. Make the seconds-vs-millis precision difference explicit in API names,
   docs, and tests.
3. Defer millisecond-precision KSUID to follow-up issue #171 because it is not the
   same wire/string format as standard Segment KSUID and cannot be safely
   distinguished from standard KSUID by a bare 27-character Base62 string.
4. Preserve concurrent uniqueness proof with goroutine stress and race tests.
5. Update README/CHANGELOG/WIP so KSUID is no longer shown as deferred if it is
   implemented in this issue.

## Non-goals

- Do not expose `github.com/segmentio/ksuid.KSUID` as stable bluetape-go API.
- Do not use `ksuid.SetRand`; global mutable entropy would leak test settings
  across callers and packages.
- Do not add UUID Base62, Flake, or Hashids in this issue.
- Do not claim strict clock monotonicity. KSUID order is wall-clock sortable,
  not a rollback-proof monotonic sequence.
- Do not add context-aware batch helpers unless implementation adds long-running
  or cancellable batch generation.

## Proposed API

Seconds precision:

```go
func NewKSUIDGenerator(options ...KSUIDOption) (StringGenerator, error)
func NewKSUID() (string, error)
func ParseKSUID(value string) (string, error)
func KSUIDTime(value string) (time.Time, error)
```

Options:

```go
type KSUIDOption func(*ksuidGenerator) error

func WithKSUIDEntropy(entropy io.Reader) KSUIDOption
func WithKSUIDTime(now func() time.Time) KSUIDOption
```

Implementation notes:

- Default entropy is `crypto/rand.Reader`.
- Default clock is `time.Now`.
- Use `segmentio/ksuid.FromParts(now, payload)` instead of
  `ksuid.NewRandom` or `ksuid.SetRand`, so entropy injection is generator-local.
- Validate `now.Unix() - 1400000000` is inside the standard KSUID
  `0..math.MaxUint32` seconds-offset range before calling Segment `FromParts`.
- Read exactly 16 payload bytes per seconds KSUID.
- Wrap entropy failures as `EntropyError{Kind: "ksuid", Err: err}`.
- Wrap parse failures as `ParseError{Kind: "ksuid", Value: value, Err: err}`.
- `ParseKSUID` returns the canonical 27-character Base62 string and rejects
  non-canonical encodings.
- `KSUIDTime` returns UTC time extracted from the canonical seconds KSUID.

Millisecond precision:

Deferred from 0.6.0.

Do not add these APIs in issue #166:

```go
func NewKSUIDMillisGenerator(...)
func NewKSUIDMillis()
func ParseKSUIDMillis(...)
func KSUIDMillisTime(...)
```

Rationale:

- Segment KSUID and bluetape4k millis KSUID are both 20-byte payloads rendered
  as 27-character Base62 strings.
- A bare millis string can be passed to a seconds parser and silently produce a
  wrong timestamp, and vice versa.
- The Kotlin source uses a custom `BytesBase62` alphabet/bit-stream encoder for
  KSUID millis, while Segment KSUID uses `0-9A-Z-a-z` Base62.
- A Go millis implementation therefore needs a separate design decision:
  exact Kotlin string compatibility, a distinguishable representation, or an
  explicit documented non-compatibility contract.

Follow-up issue #171 owns this design. `id/README*.md` must say standard KSUID
seconds is implemented and bluetape4k millis is deferred.

## Error Contract

- Nil option returns `OptionError{Option: "option", Err: ...}`.
- Nil entropy returns `OptionError{Option: "entropy", Err: ...}`.
- Nil clock returns `OptionError{Option: "now", Err: ...}`.
- Short/failing entropy readers return `EntropyError` wrapping the causal error.
- Out-of-range injected clocks return `OptionError{Option: "time", Err: ...}`
  compatible with `errors.Is(err, id.ErrInvalidOptions)`.
- Invalid KSUID strings return `ParseError` compatible with
  `errors.Is(err, id.ErrInvalidID)`.
- Empty, too-short, too-long, non-Base62, and out-of-range strings are invalid.

## Concurrency Contract

- A shared KSUID generator must be safe to call from many goroutines.
- KSUID generators must not use package-global mutable entropy.
- If the generator stores only immutable `io.Reader`/clock references and reads
  entropy/time per call, concurrency safety depends on the injected reader and
  clock function. The docs must state that custom entropy readers and custom
  clock functions must be concurrency-safe when the generator is shared.
- Default generators using `crypto/rand.Reader` are safe for concurrent use.

## Documentation Contract

Update:

- `id/README.md` and `id/README.ko.md`.
- Root `README.md` and `README.ko.md` package summary if KSUID is implemented.
- `CHANGELOG.md`.
- `WIP.md`.

Docs must explain:

- KSUID is a 27-character URL-safe Base62 string.
- Seconds KSUID is standard Segment-compatible KSUID.
- Millis KSUID remains deferred because seconds and millis cannot be safely
  disambiguated from a bare 27-character Base62 string.
- Choose KSUID when URL-safe, copy/paste-friendly, time-sortable string IDs are
  preferred and Snowflake machine coordination is not desired.
- Use UUID v7 or ULID when ecosystem interoperability is more important.

## Test Requirements

Targeted tests under `id`:

- generation success for seconds.
- canonical parse and time extraction.
- invalid length, invalid alphabet, and non-canonical strings.
- deterministic time with deterministic entropy, with exact decoded-byte
  assertions proving timestamp endian/order, payload placement, and fixed
  27-character canonical Base62 output.
- out-of-range injected clock rejection before ID generation.
- entropy failure wrapping.
- lexicographic sorting by timestamp for different timestamps.
- no strict same-timestamp monotonicity claim unless implementation explicitly
  provides it.
- shared-generator goroutine uniqueness stress for seconds using
  `testing/concurrency.NewGoroutineStressTester`.
- `go test -race -count=1 ./id`.
- benchmark coverage for seconds.

Validation commands:

```bash
git diff --check
go test -count=1 ./id
go test -race -count=1 ./id
go test -run '^$' -bench . -benchmem ./id
go test -count=1 ./...
make ci
```

## Acceptance Criteria

- PR metadata matches issue #166: assignee `debop`, milestone `0.6.0`, labels
  `type: task`, `priority: p1`, `area: utilities`.
- Standard seconds KSUID is implemented with generator-local entropy and clock.
- Millis KSUID is deferred to #171 with documented rationale.
- Public APIs are narrow and do not expose dependency concrete types.
- README selection guide no longer says KSUID is fully deferred if seconds
  support lands.
- Test requirements and validation commands in this spec pass before PR.
- P0/P1 findings are zero after Step 6-R and Step 7-R 7-Tier reviews.
