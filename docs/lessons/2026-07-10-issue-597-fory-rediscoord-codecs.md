# Issue #597 Lesson: Serialization Bounds Must Cover Every Allocation Layer

## Context

`rediscoord` stores a codec payload inside a binary profile wrapper, then inside
a JSON/base64 owner-result envelope, then inside Redis. Apache Fory also returns
marshal bytes owned by mutable runtime state.

## Learning

A payload limit at provider decode is not enough. The owner path must reject
oversized provider bytes before copying or wrapping them, preflight the outer
JSON/base64 size before its large allocation, and bound the Redis read response
before JSON decode. The runtime-owned Fory bytes must be copied while the same
mutex still protects the runtime.

Error redaction has the same layered property. A sanitized `Error()` is
insufficient when `Unwrap()` exposes raw registration or provider text. Replace
untrusted causes with safe sentinel causes while keeping stable typed
operation/profile/reason labels.

## Durable Checks

- Pin wire profile, wrapper version, and all provider metadata limits.
- Reject limits larger than the wire length field.
- Use a root-kind whitelist and prove nil/empty/zero semantics for every public
  profile.
- Make shared runtime ownership panic-safe and detect value copies with
  `go vet -copylocks`.
- Treat namespace, codec/profile, registration set, and every size limit as one
  rollout tuple.
- Keep benchmark claims out of implementation PRs. Benchmark work requires a
  same-condition result table, Chart, and written analysis.
