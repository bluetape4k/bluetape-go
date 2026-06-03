// Package resilience provides composable, context-aware policies for service
// calls.
//
// The package owns its implementation inside bluetape-go. External resilience
// libraries are useful references, but the runtime behavior and public API stay
// first-party so policies can evolve consistently across retry, timeout,
// circuit breaker, bulkhead, and HTTP integration.
package resilience
