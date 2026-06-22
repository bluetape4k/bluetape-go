# Issue #207 Wildcard and Hash Utility Design

Issue: [#207](https://github.com/bluetape4k/bluetape-go/issues/207)  
Parent Epic: [#204](https://github.com/bluetape4k/bluetape-go/issues/204)  
Milestone: `0.6.3`  
Date: 2026-06-22

## Goal

Add the Go-native subset of bluetape4k wildcard and hashing utilities that is
useful as shared foundation API:

- wildcard string matching with `*`, `?`, and escaped wildcard literals;
- lexical path wildcard matching with Ant-style `**` path segments;
- deterministic XXH64 helpers for bytes and strings.

The goal is selective utility parity, not a port of JVM classpath resources,
system properties, shutdown hooks, or generic object hashing.

## Current Evidence

- #207 asks for practical `Wildcard`, `XXHasher`, resource, system, and
  string/byte utility parity where it remains Go-native.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md` identifies
  `Wildcard.kt` and `XXHasher.kt` as the useful parity candidates, while
  excluding a broad hashing framework and JVM resource/system helper APIs.
- `docs/research/2026-06-01-issue-8-core-support-inventory.md` already maps
  simple UTF-8 byte/string helpers to the Go standard library and recommends
  package-specific hashers instead of a generic object-hash helper.
- Current `core` has string, number, validation, pointer, zero, and range
  helpers, but no wildcard or shared deterministic hash utility.
- Current `probabilistic` hashing is package-local Bloom-filter index
  derivation using SHA-256. It should remain separate because it is tied to
  Bloom filter bit-offset generation, not public utility hashing.
- Kotlin `Wildcard.kt` supports `?`, `*`, escaped `\*` and `\?`, and `**`
  when matching tokenized paths.
- Kotlin `XXHasher.kt` uses JVM object `hashCode()` values and streams those
  integer bytes through XXHash32. That behavior is not source-compatible for Go
  arbitrary values because Go has no stable universal object hash code.
- Kotlin `Resourcex.kt`, `Systemx.kt`, and `ShutdownQueue.kt` are JVM-centered:
  class loaders, Java system properties, Java version checks, and shutdown
  hooks. Go callers should use `os`, `io/fs`, `runtime`, and explicit cleanup
  ownership instead.

Retrieval note: GNO returned #207, #204, and parity documents. A local
`node-llama-cpp` Metal warning appeared during GNO search, but results were
returned. `context-mode` also warned that its installed CLI was one patch behind.

## Brainstorming Options

### Option 1: Wildcard Only

Add wildcard matching and defer hashing/resource/system helpers.

Pros: smallest public surface and no dependency change.  
Cons: does not satisfy the hashing part of #207 and leaves future cache/key
callers without a shared deterministic non-crypto hash helper.

### Option 2: Wildcard Plus Byte/String XXH64

Add wildcard matching plus raw bytes/string XXH64 helpers. Exclude generic
object hashing and JVM resource/system APIs.

Pros: satisfies the useful #207 parity, keeps input contracts explicit, avoids
surprising reflection or formatting-based hashing, and uses a small dependency
already present indirectly in `go.mod`.  
Cons: promotes `github.com/cespare/xxhash/v2` to a direct dependency and needs
clear documentation that the helper is non-cryptographic.

### Option 3: Broad Kotlin Utility Port

Port object-hash varargs, classpath resources, system-property wrappers,
shutdown queues, and string/byte helper aliases.

Pros: closest API parity on paper.  
Cons: Kotlin/JVM-shaped, hides OS and lifecycle errors, conflicts with Go's
standard library, and creates unstable generic hashing semantics.

## Chosen Approach

Use Option 2.

Implement only:

1. `core.MatchWildcard(pattern, value string) (bool, error)`
2. `core.FirstWildcardMatch(value string, patterns ...string) (int, error)`
3. `core.MatchWildcardPath(pattern, path string) (bool, error)`
4. `core.FirstWildcardPathMatch(path string, patterns ...string) (int, error)`
5. `core.XXH64Bytes(value []byte) uint64`
6. `core.XXH64String(value string) uint64`

Use `github.com/cespare/xxhash/v2` for XXH64. It is already an indirect module
at `v2.3.0`, has a small raw bytes/string API, is not archived as of the GitHub
metadata checked on 2026-06-22, and the module cache contains an MIT license.

Do not add:

- generic `Hash(values ...any)` or reflection-based object hashing;
- JVM-compatible `hashCode()` parity;
- `hash/maphash` wrappers, because random seeding makes stable cross-process
  outputs easy to misuse;
- `crc32` wrappers, because CRC32 is checksum-shaped rather than a general
  cache-key hash;
- classpath resource loading, system-property wrappers, Java version helpers,
  shutdown queues, temp/output/env helpers, or broad byte/string aliases.

## API Design

### Wildcard String Matching

`MatchWildcard(pattern, value string) (bool, error)` is pattern-first to align
with Go's `path.Match` and `filepath.Match`.

Syntax:

- `?` matches exactly one Unicode rune.
- `*` matches zero or more Unicode runes.
- consecutive `*` tokens collapse to one wildcard.
- `\*`, `\?`, and `\\` match literal `*`, `?`, and `\`.
- any other escaped rune matches the rune literally.
- a trailing backslash in a string pattern returns an error.
- matching is case-sensitive and locale-independent.

The implementation should parse pattern runes into tokens, then use dynamic
programming instead of recursive backtracking. This avoids exponential behavior
on inputs such as many repeated `*a` fragments.

`FirstWildcardMatch(value, patterns...)` returns the zero-based index of the
first matching pattern, or `-1` when none match. It returns the first malformed
pattern error before considering later patterns.

### Wildcard Path Matching

`MatchWildcardPath(pattern, path string) (bool, error)` is lexical only. It does
not inspect the filesystem, does not call `filepath.Clean`, and does not apply
OS-specific case folding.

Path syntax:

- both `/` and `\` are accepted as separators in the input path and pattern;
- repeated, leading, and trailing separators are ignored during tokenization;
- ordinary path segments use the same `*`, `?`, and escape semantics as
  `MatchWildcard`;
- because `\` can also be a separator, escaped `*`, `?`, and `\` inside a path
  pattern segment are supported for slash-separated pattern segments;
- a segment that is exactly `**` matches zero or more path segments;
- `**` has special meaning only as a full segment;
- matching is case-sensitive on every OS.

This keeps behavior portable across macOS, Linux, Windows-style input strings,
and CI logs. It intentionally does not mirror `filepath.Match`, whose separator
and escaping behavior depends on OS path rules.

`FirstWildcardPathMatch(path, patterns...)` follows the same index and malformed
pattern behavior as `FirstWildcardMatch`.

### Hashing

`XXH64Bytes` and `XXH64String` compute XXH64 with seed 0 and return `uint64`.
They are deterministic for the same byte sequence across processes and
platforms supported by the dependency.

The hash helpers are explicitly non-cryptographic. Documentation must direct
callers to `crypto/*` or keyed MACs for security-sensitive input, attacker
controlled token generation, signatures, passwords, or integrity checks.

Only raw byte and string inputs are accepted. Callers that need composite keys
must encode them explicitly using a stable format owned by that caller. This
avoids the Kotlin `XXHasher.hash(vararg Any?)` pitfall where object hash-code
semantics become the public API.

## Testing Requirements

Wildcard tests must cover:

- exact matches and mismatches;
- `*`, `?`, consecutive stars, empty pattern/value behavior;
- escaped wildcard literals and escaped backslashes;
- trailing escape malformed pattern error;
- Unicode rune matching;
- case-sensitive behavior;
- first-match index behavior;
- path `**` matching zero, one, and many segments;
- `/` and `\` separator handling;
- `**` only acting specially as a full segment.

Hash tests must cover:

- empty bytes and empty string outputs;
- ASCII and Unicode string outputs;
- deterministic repeated calls;
- bytes and string equivalence for the same UTF-8 bytes;
- non-cryptographic behavior documented in README, not tested as a security
  property.

Use TDD red/green cycles:

1. write failing tests for wildcard functions;
2. implement wildcard parsing/matching;
3. write failing tests for XXH64 helpers;
4. implement hash helpers and promote the dependency to direct if `go mod tidy`
   requires it.

## Documentation Requirements

Update `core/README.md` and `core/README.ko.md` with:

- wildcard syntax and path semantics;
- case-sensitivity and lexical-only path behavior;
- malformed trailing escape behavior;
- XXH64 byte/string examples;
- non-cryptographic warning;
- exclusions for JVM resources, system properties, shutdown hooks, generic
  object hashing, temp/output/env helpers, and broad string/byte aliases.

The package-level doc should mention wildcard/hash helpers without making
`core` sound like a generic utility dumping ground.

## Verification Plan

Run:

```bash
go test -count=1 ./core
go test -race -count=1 ./core
go test ./...
make fmt-check
make tidy-check
make vet
make lint
make ci
git diff --check
```

The full `make ci` gate may run Testcontainers-backed packages. If Docker or
shared ports fail for unrelated environmental reasons, capture the exact failure
and rerun the affected package sequentially when applicable.

## Acceptance Mapping

- Dependency choices justified: spec compares stdlib FNV/CRC32/maphash and
  narrow XXH64 dependency use.
- Wildcard semantics covered: syntax, escaping, path separator, `**`, Unicode,
  malformed patterns, and case-sensitivity are fixed above.
- Hash outputs covered: deterministic XXH64 byte/string fixtures are required.
- Resource/system helpers: excluded because Go standard library APIs expose OS
  errors and lifecycle ownership more clearly.
- Portability limits: README must document lexical path matching and
  case-sensitive behavior on macOS/Linux/CI.
- Public vs internal support: wildcard and XXH64 are public `core` API;
  resource/system/temp/env helpers are not implemented.
