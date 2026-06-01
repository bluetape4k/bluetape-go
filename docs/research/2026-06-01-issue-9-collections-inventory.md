# Issue 9 Collections Inventory

## Context

Issue #9 ports useful `bluetape4k/core` collection support into idiomatic Go.
The Kotlin source includes many extension helpers and mutable data structures,
so this inventory narrows the first Go slice to transformation helpers that the
standard library does not already cover.

## Source Inventory

Observed `bluetape4k-projects/bluetape4k/core` collection areas:

- `IterableSupport.kt`: iterator conversion, counting, primitive array
  conversion, catching map/for-each variants, sliding windows.
- `ListSupport.kt`: range list construction.
- `CollectionSupport.kt`: prepend/append/swap, padding, element counts,
  chunk-by-predicate, safe sub-list, zip-with-index.
- `MapEntrySupport.kt`: pair/map-entry conversion.
- `BoundedStack.kt`, `RingBuffer.kt`, `QueueSupport.kt`, `PaginatedList.kt`:
  reusable data structures and pagination models.
- `permutations/*`, `graph/*`, Eclipse Collections adapters, and Java stream
  support: specialized or JVM-specific areas.

## Implement Now

- `Chunk`: fixed-size chunking with error-returning validation.
- `ChunkBy`: predicate-started chunking.
- `Distinct` and `DistinctBy`: order-preserving duplicate removal.
- `GroupBy` and `CountBy`: grouping/counting by derived keys.
- `MapErr` and `FilterErr`: stop-on-first-error transformations.
- `FilterMap`: map and keep only successful/selected results.

## Adopt Standard Library Instead

- Simple contains/index/delete/sort/reverse operations: use Go's `slices`
  package directly.
- Map key/value iteration where order does not matter: use Go's `maps` package
  or direct `for range`.
- Pair/map-entry conversion: Go map iteration already exposes key/value pairs.
- Iterator-to-slice conversion: use Go iterator helpers when the input is an
  iterator, not a slice.

## Defer

- Bounded stack, queue, ring buffer, pagination, and permutation helpers need
  separate API decisions and examples before becoming public packages.
- Sliding windows can follow if a real use case appears; `Chunk` and `ChunkBy`
  cover the current 0.1.0 needs.
- Primitive array conversion is JVM-specific and not useful in Go.
- Eclipse Collections and Java stream adapters are not portable to Go.

## Decision

Create a small public `collections` package. Keep helpers deterministic,
allocation-conscious, and explicit about nil versus empty slices/maps. Do not
wrap the standard `slices` or `maps` packages without bluetape-specific value.

