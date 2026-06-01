# Issue 8 Core Support Inventory

## Context

Issue #8 ports broadly reusable `bluetape4k/core` support concepts into
idiomatic Go. The source module is large and Kotlin/JVM-oriented, so this
inventory separates direct implementation from standard-library adoption and
deferred domains.

## Source Inventory

Observed `bluetape4k-projects/bluetape4k/core` areas:

- `support/*`: require/assert helpers, string helpers, value conversion,
  number parsing, UUID helpers, result/lazy/timeout helpers, Java type helpers.
- `collections/*`: list/map/sequence helpers, bounded stack, ring buffer,
  pagination, permutations.
- `codec/*`: Base58, Base62, Base64, Hex, URL-safe UUID encoders.
- `concurrent/*`: Future/CompletableFuture helpers, locks, reducers, executor
  and thread helpers.
- `ranges/*`: closed/open range models and validation.
- `javatimes/*`: JVM time/date convenience helpers.
- `apache/*`, Java/Kotlin reflection helpers, and JVM-specific adapters.

## Implement Now

- Validation helpers that return `error`: blank/empty text, ordered ranges,
  positive and non-negative numeric checks.
- Pointer helpers: `Ptr`, `ValueOr`, `ValueOrZero`.
- Zero/default helpers: `Zero`, `IsZero`, `DefaultIfZero`, `FirstNonZero`.
- String helpers where they clarify service code: `HasText`, defaulting helpers,
  UTF-8 byte-safe truncation.
- Small numeric helpers: `Clamp`, hex digit and prefixed hex format checks.

## Adopt Standard Library Instead

- UTF-8 byte conversion: use `[]byte(s)` and `string(b)`.
- Generic equality for comparable values: use `==`.
- Numeric parsing: use `strconv` directly unless a future API needs
  bluetape-specific parsing semantics.
- Time/date helpers: use `time` directly until a repeated workflow emerges.
- Hashing: use `hash`, `hash/fnv`, `crypto/*`, or package-specific hashers
  instead of a generic object-hash helper.

## Defer

- Collection helpers, bounded stack, ring buffer, pagination, and permutations:
  tracked by #9.
- Goroutine/context helpers: tracked by #10.
- String codecs and URL-safe IDs: tracked by #11.
- Binary serializer and compressor abstractions: tracked by #12 and #13.
- JVM/Kotlin reflection, Apache Commons adapters, Java Optional helpers, and
  thread/future helpers: not portable to Go and should not be ported directly.

## Decision

Keep `core` small. Add helpers only when they reduce repeated service code while
remaining obvious to a Go reader. Do not create a Kotlin-extension-shaped
utility bag.

