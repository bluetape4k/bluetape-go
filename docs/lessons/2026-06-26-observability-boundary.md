# Issue #275 Lessons

- Treat `log/slog` as the Go logging contract by default. A library package
  should not mutate defaults, own global logger state, or clone JVM MDC helper
  shapes.
- Observability belongs first to the package that owns the event source. Define
  stable events and labels there, then let callers bridge to logging, metrics,
  or tracing.
- Generic telemetry facades create dependency and lifecycle ownership before
  there is proof of package-specific value. Keep exporter dependencies out of
  core packages until a concrete adapter issue proves the need.
