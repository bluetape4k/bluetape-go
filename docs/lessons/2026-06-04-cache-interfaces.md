# Lessons Learned - Cache Interfaces (2026-06-04)

**Related issue**: #22
**Affected package**: `cache`

## L1: Generic cache keys need an explicit flight-key namespace

### Problem

`singleflight.Group` accepts string keys, but the public cache API uses
`K comparable`. A naive implementation could convert keys with `fmt.Sprint` and
accidentally merge different values that stringify the same way.

### Lesson

When a generic API delegates to a string-keyed coordination primitive, design a
package-owned collision-free namespace and test it with keys whose `String`
output is identical.

## L2: Cache-aside loaders need documented delete/clear ordering

### Problem

`Delete` and `Clear` are safe concurrent methods, but they do not cancel a
loader that already started through `GetOrLoad`. Without a documented ordering
contract, callers may assume delete/clear permanently prevents a later in-flight
write.

### Lesson

For cache-aside APIs, document whether mutation methods cancel or only race with
in-flight loaders. Tests should prove data-race safety, and README/package docs
should state the caller-visible ordering.

## L3: Exported concrete types should be zero-value safe or hidden

### Problem

The first `Memory[K,V]` implementation exposed a concrete type but initialized
its maps only through `NewMemory`. A caller using `var c cache.Memory[...]`
could panic on `Set`.

### Lesson

When a Go package exports a concrete type, either keep the zero value usable or
avoid exposing the type. Add a zero-value test before PR review closes.
