# Serialization

Issue #12 establishes `serialization` as the safe binary contract layer before
compression work. Keep defaults dependency-free and explicit: JSON via
`encoding/json`, raw byte/string serializers as copy-safe utilities, and a
small versioned envelope for persisted payloads. Do not introduce JVM-style
object deserialization or reflection-heavy binary formats without a separate
security review.
