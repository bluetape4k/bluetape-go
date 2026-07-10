# Issue #592 Lesson: Shared Redis Construction Has a Narrow Compatibility Boundary

## Context

`probabilistic/redis` had local Bloom and HyperLogLog structural key formatting
that exactly matched the shared `redis.KeyBuilder` for its fixed prefixes and
validated namespaces.

## Learning

Reuse a shared Redis key builder only after the provider's own validation has
accepted caller input, and use only the constructed key value when the provider
owns a distinct public diagnostic contract. Shared construction does not imply
that validation errors, redacted identifiers, or typed operational errors are
compatible.

For this package, the shared builder's 24-hex `Key.RedactedID` must not replace
the existing `redis-key:` plus 12-hex probabilistic identifier. The provider's
namespace validation also remains the first caller-visible boundary because it
contains sensitive-marker policy beyond generic hash-tag validation.

## Durable Checks

- Assert exact Redis key bytes, including a colon-containing Cluster hash tag.
- Add a direct private-adapter RED test when output-parity tests would be
  false-green against the old formatting implementation.
- Keep shared builder failures opaque and unwrapped after locally validated
  input; do not expose shared key-validation error types through the provider.
- Preserve provider-specific `RedisError` and script metadata sentinel mapping
  unless a separate public compatibility issue approves changing them.
- Mark benchmark work N/A for construction-only migrations. Any cross-provider
  performance conclusion belongs to issue #560 and requires a result table,
  chart, and written analysis together.
