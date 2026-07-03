# Issue #360 Collection Helper Scope

Date: 2026-07-03

Kotlin `bluetape4k-core` exposes a broad collection surface: mutable list
extensions, sequence DSLs, synchronized containers, primitive-array adapters,
and permutation utilities. The Go package already had useful bounded stack,
ring buffer, page, permutation, chunk, group, distinct, and error-aware map
contracts.

Lesson: when adapting Kotlin collection helpers to Go, start from actual
service-code readability gaps instead of feature parity. Narrow helpers such as
`Sliding`, `PadTo`, `SafeSubslice`, `Count`, `ZipWithIndex`, and `ForEachErr`
are acceptable because their contracts are clearer than repeated local code.
Synchronized container facades, broad sequence DSLs, Java stream parity, and
primitive-array conversion helpers should remain explicit non-goals unless a
future issue proves concrete production demand.

Prevention: before adding another collection helper, scan the repo for duplicated
production logic and document why the helper is clearer than `slices`, `maps`,
or a short local loop.
