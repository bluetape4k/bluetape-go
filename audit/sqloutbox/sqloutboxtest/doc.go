// Package sqloutboxtest provides deterministic sqloutbox publisher helpers for
// tests, local examples, and workshop adoption.
//
// The package is not a durable transport adapter. It implements the
// sqloutbox.Publisher contract without broker lifecycle, topology, retention,
// authentication, or replay policy.
package sqloutboxtest
