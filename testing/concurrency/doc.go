// Package concurrencytest provides deterministic helpers for concurrent tests.
//
// The package mirrors the testing intent of bluetape4k-junit5 stress helpers
// while keeping Go APIs named around Go concepts. Use these helpers when a test
// needs repeated bounded goroutine execution or context-aware async job checks.
// Prefer plain go test and direct concurrency.Group usage for one-off
// goroutine or structured-cancellation checks.
package concurrencytest
