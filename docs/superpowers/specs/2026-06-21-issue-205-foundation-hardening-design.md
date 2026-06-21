# Issue #205 Foundation Contract Hardening Design

## Goal

Harden the existing `core`, `collections`, `codec`, and `serialization`
foundation contracts before `0.6.3` adds more core parity APIs. This work keeps
the public API narrow and Go-shaped while making edge behavior explicit,
tested, and documented.

## Source Evidence

- GitHub issue: #205 `audit: Harden existing core collections and codec contracts`
- Parent epic: #204 `[Epic] 0.6.3 Core foundation parity and hardening`
- Milestone: `0.6.3`
- Labels: `priority: p0`, `type: task`, `area: core`, `area: serialization`
- Assignee: `debop`
- Baseline command on this worktree:

```bash
make ci
```

Baseline result on 2026-06-21:

```text
PASS: make ci
```

Repository evidence used:

- `docs/research/2026-06-21-issue-202-source-parity-matrix.md` routes core,
  collections, codec, string, and validation hardening into #204 / #205 before
  new range/time/resource parity issues.
- `core/*_test.go`, `collections/*_test.go`, `codec/codec_test.go`, and
  `serialization/serialization_test.go` already cover many happy-path and
  malformed-input cases, but several nil, empty, boundary, and text/binary
  contract cases remain implicit.
- `collections/README.md` and `collections/README.ko.md` document nil key
  function rejection for some helpers, but do not mention nil mapper/predicate
  rejection for `MapErr`, `FilterErr`, and `FilterMap`.
- `codec` and `serialization` string APIs describe UTF-8 text conversion while
  current implementations convert decoded bytes into Go strings without UTF-8
  validation.

Retrieval note: context-mode search was unavailable in this session, so this
spec uses live GitHub issue metadata, GNO docs search with stale-result caveat,
codegraph status, and direct repository inspection.

## Brainstorming Summary

### Approach 1: Contract-First Hardening Without New Public APIs

Add focused tests for existing contracts, tighten the text/binary boundary where
documentation already promises UTF-8 text, and update README files to match the
actual helper behavior. Keep all new parity primitives out of this branch.

Chosen because #205 is a P0 audit/hardening task and #206-#208 own new range,
resource, wildcard, and time APIs.

### Approach 2: Broad Core Parity Expansion

Add missing Kotlin-inspired range, collection, wildcard, resource, and time
helpers immediately while touching the same packages.

Rejected because #204 explicitly sequences #205 before new parity APIs. Mixing
audit hardening with new APIs would make the review surface too broad and would
hide contract defects behind feature work.

### Approach 3: Documentation-Only Audit Closure

Record the gaps and file follow-up issues without changing tests or behavior.

Rejected because #205 acceptance criteria require tests for success, failure,
zero-value, nil, malformed codec input, leading-zero behavior, and README
accuracy. Documentation alone would not satisfy the milestone gate.

## Chosen Approach

Use Approach 1.

The implementation will focus on current package contracts:

1. `codec`: keep byte encoders byte-oriented, but make string decoders reject
   decoded bytes that are not valid UTF-8. Add an `errors.Is`-detectable
   invalid UTF-8 sentinel for decode string helper failures. Existing
   no-error `Encode*String` helpers cannot report invalid caller strings, so
   document them as legacy convenience string-to-byte conversions rather than
   validation points. Add malformed vector tests for Base64URL, hex invalid
   characters, and encoded invalid UTF-8 payloads.
2. `serialization`: keep `BytesSerializer` as the binary contract and make
   `StringSerializer` reject invalid UTF-8 on marshal and unmarshal with the
   same invalid UTF-8 sentinel contract used by `codec`. Add metadata tests for
   empty and overlong `VersionedSerializer` formats.
3. `collections`: cover nil/empty input and nil callback behavior across
   `ChunkBy`, `DistinctBy`, `MapErr`, `FilterErr`, `FilterMap`, `GroupBy`, and
   `CountBy`. Update English and Korean README behavior bullets to describe
   mapper/predicate callback rejection.
4. `core`: add boundary tests for validation ranges, `TruncateUTF8Bytes` zero,
   rune-boundary, and invalid UTF-8 rejection behavior, and numeric/hex edge
   cases already implied by the public helpers.

## Scope

In scope:

- `codec`
- `serialization`
- `collections`
- `core`
- Matching English/Korean README updates for touched package behavior
- Tracked review artifacts under `docs/superpowers/reviews`
- A lesson under `docs/lessons`

Out of scope:

- New range, time, wildcard, resource, or collection primitives.
- Kotlin/JVM extension-method parity.
- New dependencies.
- Broad package rewrites or naming churn.
- Testcontainers-backed packages, except final `make ci` may exercise the
  existing CI gate.

## Contract Decisions

- Text helper APIs with an error return are text contracts. If decoded bytes or
  caller-provided strings are not valid UTF-8, string decode, unmarshal, and
  truncation helpers should return an error rather than smuggling invalid text
  into callers.
- Existing no-error `Encode*String` helpers keep their compatibility behavior:
  they convert Go strings to bytes and encode those bytes. Documentation must
  state that these helpers cannot signal invalid UTF-8 and callers that need
  validation should validate before encoding or use byte helpers for binary
  payloads.
- Invalid UTF-8 text failures must use the exported `core.ErrInvalidUTF8`
  sentinel so callers can distinguish text-contract failures with
  `errors.Is`. This intentionally makes `codec` and `serialization` depend on
  the small `core` package for the shared error contract; the plan and final
  API review must prove there is no import cycle and caller usage is clear.
- Byte helper APIs remain binary contracts. `DecodeBase58`, `DecodeBase62`,
  `DecodeBase64`, `DecodeBase64URL`, `DecodeHex`, and `BytesSerializer` continue
  to accept arbitrary bytes when the encoding itself is valid.
- Empty input remains package-specific and documented:
  - byte codec decoders keep empty-string-to-empty-bytes round trips for Go byte
    API ergonomics;
  - string codec decoders keep empty-string-to-empty-string round trips when the
    encoded payload is empty and valid;
  - serializers reject nil unmarshal input but allow empty non-nil payloads
    where the contract already supports them.
- Collections preserve nil slice input as nil results where the helper already
  does so, and preserve empty non-nil input as empty non-nil results where the
  helper already does so.

## Test Strategy

Use TDD for behavior changes and regression tests for already-implemented edge
contracts:

1. Write failing tests for invalid UTF-8 in `codec` string decoders,
   `serialization.StringSerializer`, and `core.TruncateUTF8Bytes`.
2. Write regression tests for collection nil/empty/callback behavior.
3. Write regression tests for core range and truncation boundaries.
4. Implement the minimal production changes for UTF-8 validation and any
   discovered contract bug.
5. Update README files after behavior is proved by tests. English and Korean
   README behavior notes must explain that decode string helpers and
   `StringSerializer` are for UTF-8 text, no-error encode string helpers cannot
   report invalid UTF-8, and binary payloads should use byte codec helpers or
   `BytesSerializer`.
6. Run targeted package tests, package race gates where relevant, `make ci`,
   and `git diff --check`.

Required final commands:

```bash
go test -count=1 ./core ./collections ./codec ./serialization
go test -race -count=1 ./codec ./serialization
make ci
git diff --check
```

The final verification may add broader commands if implementation touches more
packages. Any environment blocker must be recorded with the exact command and
error.

## 7-Tier Review Contract

Steps 2-R, 3-R, 6-R, and 7-R use the same shape:

- Performance lane: no unnecessary allocation-heavy rewrites or broad global
  test loops.
- Stability lane: nil, empty, malformed, boundary, and invalid UTF-8 behavior
  is deterministic and tested.
- Security lane: malformed encoded input and text/binary boundaries do not
  accept ambiguous or invalid caller-controlled data as text.
- Operator/Ops lane: CI remains bounded; no new infrastructure or Docker scope
  is introduced by this branch.
- Developer/API lane: public API stays Go-native, documented, and backward
  compatible except for intentionally tightened invalid UTF-8 text rejection.
- User/Caller lane: README examples and behavior bullets match current APIs in
  English and Korean.
- Main integration review: synthesize all six lanes and record `P0=<n> P1=<n>`.

Native subagents are preferred when available. If subagent execution is not
available or stalls, the main session must perform independent role-switched
lanes, record the fallback, and continue without long blocking waits.

## Step DoD

| Step | Action | Expected DoD |
|---|---|---|
| Step 2 | Write this spec with baseline and evidence. | Spec exists; self-review finds no placeholders or contradictions; Step 2-R records `P0=0 P1=0`. |
| Step 3 | Write implementation plan under `docs/superpowers/plans`. | Plan lists exact files, RED/GREEN steps, commands, and review gates. |
| Step 4 | Implement via TDD. | RED failures observed for UTF-8 tightening before production changes; regression tests cover existing edge contracts. |
| Step 5 | Verify against spec and plan. | Targeted tests, race gates, `make ci`, and `git diff --check` are recorded. |
| Step 6 | Run 7-Tier code review. | Six lanes plus main integration verdict; `P0=0 P1=0`. |
| Step 7 | Lessons, commit, PR, and PR review. | Lessons commit exists before PR; PR body ends with `## DoD Status`; Step 7-R passes. |
