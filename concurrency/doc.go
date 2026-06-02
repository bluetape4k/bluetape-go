// Package concurrency provides context-aware helpers for running goroutines.
//
// The package builds on golang.org/x/sync/errgroup and keeps the public
// contract small: safe goroutine launch, errgroup-style task groups, bounded
// parallel map/for-each helpers, and simple worker pools. Task panics are
// converted into errors so callers can treat failed goroutines uniformly.
package concurrency
