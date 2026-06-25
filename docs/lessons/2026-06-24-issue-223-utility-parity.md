# Issue #223 Lessons

- Utility parity should start from repeated Go callers, not from JVM module
  breadth. Existing `core`, `measure`, and `money` packages already cover the
  narrow repeated helper demand found for this milestone.
- Logging parity is especially easy to over-port. Go should stay compatible
  with `log/slog`, `context.Context`, and explicit hooks instead of adding
  global logger state, MDC-shaped APIs, or framework coupling.
- Broad math, geo, and science helpers need separate research issues because
  they mix pure values, dependency-heavy algorithms, provider-backed IO, and
  domain-specific edge cases.
- Closing an inventory issue without new code is acceptable when the durable
  outcome is a documented boundary plus follow-up issues for the oversized
  domains.
