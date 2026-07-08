# Issue #434 UUID v7 hot-path lesson

Issue: #434
Date: 2026-07-08

## Lesson

Do not treat a mutex-shaped UUID v7 profile as sufficient evidence for an
atomic logical tick rewrite.

In the fresh issue #434 benchmark, the current shared-generator UUID v7 parallel
row measured about `192.6 ns/op`. A local atomic tick reservation candidate
measured about `193.2 ns/op`, with unchanged allocations. The candidate was
therefore rejected instead of adding concurrency complexity to `id/uuid.go`.

Future UUID v7 work should first target string encoding/allocation or explicit
caller-owned pooling/sharding contracts, and should keep a measured
baseline-vs-candidate comparison before changing production code.
