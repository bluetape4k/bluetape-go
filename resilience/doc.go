// Package resilience provides composable, context-aware policies for service
// calls.
//
// The package owns its implementation inside bluetape-go. External resilience
// libraries are useful references, but the runtime behavior and public API stay
// first-party so policies can evolve consistently across retry, timeout,
// circuit breaker, bulkhead, observability, and HTTP integration.
//
// Policies expose synchronous OnEvent hooks for observability. The Event
// payload uses stable policy type, event kind, category, and error category
// labels so callers can bridge events to logging, metrics, or tracing without a
// built-in telemetry exporter dependency. Keep handlers fast and non-blocking;
// a slow handler runs on the protected call path.
package resilience
