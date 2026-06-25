// Package server provides small shared contracts for Testcontainers fixtures.
//
// The package is intentionally limited to started-container inspection,
// connection details, explicit test-scoped environment export, and cleanup. It
// does not start containers, keep global server registries, or export process
// environment variables outside testing.TB.Setenv.
package server
